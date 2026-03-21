package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"golang.org/x/net/html"
)

// fetchInput represents the input for the fetch tool.
type fetchInput struct {
	URL           string `json:"url"`
	IncludeMarkup bool   `json:"includeMarkup,omitempty"`
}

// defaultFetchTimeout is the default timeout for fetch operations.
const defaultFetchTimeout = 30 * time.Second

// maxResponseSize defines an upper bound on the number of bytes we will accept
// in an HTTP response body. The 10MB limit is a compromise: it is large enough
// to cover typical HTML pages and JSON responses used by tools, while still
// preventing unbounded memory growth if a server returns an unexpectedly large
// payload. Callers that use this constant to bound response reads should stop
// reading and treat the operation as failed (for example, by returning an error)
// when the response exceeds this limit, instead of loading the entire body into
// memory.
const maxResponseSize = 10 << 20

// registerFetchTool registers the fetch tool.
func (a *ExecutorAdapter) registerFetchTool() {
	fetchTool := entity.Tool{
		ID:          "fetch",
		Name:        "fetch",
		Description: "Fetches web resources via HTTP/HTTPS. Prefer this to bash-isms like curl/wget",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Full URL to fetch, e.g. https://...",
				},
				"includeMarkup": map[string]interface{}{
					"type":        "boolean",
					"description": "Include the HTML markup? Defaults to false. By default or when set to false, markup will be stripped and converted to plain text. Prefer markup stripping, and only set this to true if the output is confusing: otherwise you may download a massive amount of data",
				},
			},
			"required": []string{"url"},
		},
		RequiredFields: []string{"url"},
	}
	a.tools[fetchTool.Name] = fetchTool
}

// isInThisNetwork checks if an IP is in the 0.0.0.0/8 "this network" block (RFC 1122).
// IsUnspecified only matches 0.0.0.0 exactly; we need the full /8 for SSRF protection
// since some OS stacks route 0.x.x.x to localhost.
func isInThisNetwork(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 0
	}
	return false
}

// isPrivateIP checks if an IP address is in a private/internal range.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		isInThisNetwork(ip)
}

// validateURL validates that the URL is safe to fetch and blocks requests to private/internal resources.
func validateURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https protocol, got: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return errors.New("URL must have a host")
	}

	if parsedURL.User != nil {
		return errors.New("URL contains credentials which are not allowed for security")
	}

	host := parsedURL.Hostname()
	if host == "" {
		return errors.New("invalid hostname in URL")
	}

	// Block literal private IPs early. Hostname-based DNS resolution and
	// private-IP checks are handled by ssrfSafeDialContext at dial time,
	// which uses the request context so DNS respects the fetch timeout.
	if hostIP := net.ParseIP(host); hostIP != nil {
		if isPrivateIP(hostIP) {
			return fmt.Errorf(
				"direct IP address %s is in a private/internal range and is blocked for security",
				hostIP.String(),
			)
		}
	}

	return nil
}

// ssrfSafeDialContext returns a DialContext function that re-validates resolved IPs at connection
// time, preventing DNS rebinding attacks where a hostname resolves to a public IP during
// validation but flips to a private IP before the actual TCP connection.
func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("hostname %s does not resolve to any IP address", host)
	}
	for _, ip := range ips {
		if isPrivateIP(ip.IP) {
			return nil, fmt.Errorf("resolved IP %s is private, blocked for SSRF protection", ip.IP)
		}
	}
	dialer := &net.Dialer{}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// ssrfSafeClient returns a shared HTTP client with DNS-rebinding protection and redirect
// validation. The client is created once and reuses TCP connections across fetch calls.
func (a *ExecutorAdapter) ssrfSafeClient() *http.Client {
	a.fetchClientOnce.Do(func() {
		a.fetchClient = &http.Client{
			Timeout:   defaultFetchTimeout,
			Transport: &http.Transport{DialContext: ssrfSafeDialContext},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("stopped after 3 redirects")
				}
				if err := validateURL(req.URL.String()); err != nil {
					return fmt.Errorf("redirect blocked due to security policy: %w", err)
				}
				return nil
			},
		}
	})
	return a.fetchClient
}

// readResponseBody reads the body up to maxResponseSize and returns it.
func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp.ContentLength > maxResponseSize {
		return nil, fmt.Errorf("response too large: %d bytes (max: %d)", resp.ContentLength, maxResponseSize)
	}

	limited := io.LimitReader(resp.Body, maxResponseSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(body)) > maxResponseSize {
		return nil, fmt.Errorf(
			"response truncated due to size limit (max: %d bytes, read: %d bytes)",
			maxResponseSize, len(body),
		)
	}
	return body, nil
}

// htmlToText converts HTML content to plain text.
func htmlToText(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var result strings.Builder

	// Recursively extract text from nodes
	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}

		case html.ElementNode:
			// Add newline for block elements
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "br":
				result.WriteString(" ")
			case "li":
				result.WriteString(" ")
			}

			// Recursively process children
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				extractText(c)
			}
		case html.DocumentNode:
			// Process all children of document node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				extractText(c)
			}
		case html.ErrorNode, html.CommentNode, html.DoctypeNode, html.RawNode:
			// Skip these node types - they don't contain text content
			return
		}
	}

	extractText(doc)

	// Clean up whitespace
	text := result.String()
	text = strings.Join(strings.Fields(text), " ")

	return text, nil
}

// capFetchTimeout applies defaultFetchTimeout to ctx unless a tighter deadline already exists.
func capFetchTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= defaultFetchTimeout {
		return ctx, func() {} // parent deadline is already tighter
	}
	return context.WithTimeout(ctx, defaultFetchTimeout)
}

// executeFetch executes the fetch tool.
func (a *ExecutorAdapter) executeFetch(ctx context.Context, input json.RawMessage) (string, error) {
	var in fetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal fetch input: %w", err)
	}

	if err := validateURL(in.URL); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	ctx, cancel := capFetchTimeout(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("User-Agent", "github.com/anthony-bible/code-agent-demo/1.0")

	resp, err := a.ssrfSafeClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respText := resp.Status
		if resp.StatusCode == http.StatusForbidden {
			respText = "authorization required"
		}
		return "", fmt.Errorf("HTTP %d (%s)", resp.StatusCode, respText)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return "", err
	}

	content := string(body)
	if !in.IncludeMarkup && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		converted, err := htmlToText(content)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to text: %w", err)
		}
		content = converted
	}

	return content, nil
}
