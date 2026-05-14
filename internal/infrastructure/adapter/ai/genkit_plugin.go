// Package ai — genkitPluginAdapter is the per-provider extension point for
// GenkitAdapter. It abstracts the three things that vary across Genkit
// plugins: which plugin instance to load into the registry, what provider
// prefix to qualify model names with, and what raw-config struct to pass
// through ai.WithConfig (Genkit's escape hatch for provider-specific
// settings like max tokens and thinking budgets).
//
// A new provider is added by:
//  1. Implementing genkitPluginAdapter in a new file (see
//     genkit_plugin_anthropic.go for the canonical example).
//  2. Registering it from an init() with RegisterGenkitPlugin.
//  3. Selecting it at runtime via AGENT_GENKIT_PLUGIN.
package ai

import (
	"fmt"
	"sort"
	"sync"

	"github.com/firebase/genkit/go/core/api"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// genkitPluginAdapter abstracts a single Genkit plugin (Anthropic, Google AI,
// Vertex AI, Ollama, …) behind a stable surface that GenkitAdapter consumes.
//
// Implementations should be cheap to construct: Plugin() may be called once
// per process (the registry is global), Name() may be called per request
// for model-name qualification, BuildRequestConfig() is called per request,
// and ValidateModel() is called at construction and from SetModel.
type genkitPluginAdapter interface {
	// Name returns the provider prefix used to qualify model names
	// (e.g. "anthropic", "googleai", "vertexai", "ollama"). It MUST match
	// the Name() returned by the underlying api.Plugin so genkit's resolver
	// can find the model.
	Name() string

	// Plugin returns a configured api.Plugin ready to be passed to
	// genkit.WithPlugins. Implementations typically read provider-specific
	// configuration (API keys, base URLs) from environment variables here.
	Plugin() api.Plugin

	// BuildRequestConfig returns the provider-specific config struct passed
	// to ai.WithConfig for a single request. Returning nil is valid and
	// causes the option to be omitted. The thinking parameter is nil when
	// extended thinking is disabled.
	BuildRequestConfig(maxTokens int64, thinking *port.ThinkingModeInfo) any

	// ValidateModel returns a non-nil error if the model identifier is not
	// usable with this plugin (e.g. anthropic rejects hf:* prefixes).
	// Plugins with no constraints should return nil.
	ValidateModel(model string) error
}

// genkitPluginFactory constructs a fresh plugin adapter. Factories run once
// per GenkitAdapter construction; they SHOULD return an error rather than
// panic when their environment is misconfigured.
type genkitPluginFactory func() (genkitPluginAdapter, error)

//nolint:gochecknoglobals // process-global registry by design; init() in plugin files registers built-ins.
var (
	genkitPluginsMu sync.RWMutex
	genkitPlugins   = map[string]genkitPluginFactory{}
)

// RegisterGenkitPlugin registers a Genkit plugin factory under the given
// name. It is safe to call from init() functions; the registry is process
// global. Re-registering an existing name overwrites the prior factory,
// which is intentional so tests can stub built-ins.
func RegisterGenkitPlugin(name string, f genkitPluginFactory) {
	if name == "" || f == nil {
		return
	}
	genkitPluginsMu.Lock()
	defer genkitPluginsMu.Unlock()
	genkitPlugins[name] = f
}

// resolveGenkitPlugin looks up and constructs the plugin registered under
// the given name. An empty name resolves to "anthropic" for backward
// compatibility with callers that predate the abstraction.
func resolveGenkitPlugin(name string) (genkitPluginAdapter, error) {
	if name == "" {
		name = GenkitPluginAnthropic
	}
	genkitPluginsMu.RLock()
	f, ok := genkitPlugins[name]
	genkitPluginsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown genkit plugin %q (registered: %v)",
			name, registeredGenkitPluginNames())
	}
	return f()
}

// registeredGenkitPluginNames returns the sorted list of registered plugin
// names. Used in error messages so users see which plugins are available.
func registeredGenkitPluginNames() []string {
	genkitPluginsMu.RLock()
	defer genkitPluginsMu.RUnlock()
	names := make([]string, 0, len(genkitPlugins))
	for n := range genkitPlugins {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Built-in plugin name constants. Defining them here (rather than in each
// plugin file) keeps the canonical list visible alongside the abstraction.
const (
	GenkitPluginAnthropic = "anthropic"
	GenkitPluginGoogleAI  = "googleai"
	GenkitPluginVertexAI  = "vertexai"
	GenkitPluginOllama    = "ollama"
)
