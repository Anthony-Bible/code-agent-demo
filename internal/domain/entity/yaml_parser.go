package entity

import (
	"errors"
	"strings"
)

// extractFrontmatter extracts YAML frontmatter from content enclosed in --- markers.
// Returns the frontmatter content (without --- markers) and the remaining content after frontmatter.
// Returns an error if the frontmatter format is invalid.
func extractFrontmatter(content string) (frontmatter, remainingContent string, err error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", "", errors.New("invalid YAML frontmatter: missing opening ---")
	}

	// Find the closing ---
	firstLineEnd := strings.Index(content[3:], "\n---")
	if firstLineEnd == -1 {
		// Try to find it at the start of a line without the preceding newline
		firstLineEnd = strings.Index(content, "\n---")
		if firstLineEnd == -1 {
			return "", "", errors.New("invalid YAML frontmatter: missing closing ---")
		}
	}

	// Get the frontmatter part
	frontmatterEnd := firstLineEnd + 4
	frontmatterRaw := content[:frontmatterEnd]

	// Get the content after frontmatter
	remaining := strings.TrimSpace(content[frontmatterEnd+3:])

	// Remove the opening and closing --- from frontmatter
	frontmatterRaw = strings.TrimPrefix(frontmatterRaw, "---")
	frontmatterRaw = strings.TrimSuffix(frontmatterRaw, "\n---")
	frontmatterRaw = strings.TrimSpace(frontmatterRaw)

	return frontmatterRaw, remaining, nil
}
