// Package ai — anthropicGenkitPlugin wires the Genkit Anthropic plugin into
// the genkitPluginAdapter abstraction. Selection happens via
// AGENT_GENKIT_PLUGIN=anthropic (the default).
package ai

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/firebase/genkit/go/core/api"
	genkitanthropic "github.com/firebase/genkit/go/plugins/anthropic"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

//nolint:gochecknoinits // registry pattern: each plugin self-registers at process start.
func init() {
	RegisterGenkitPlugin(GenkitPluginAnthropic, func() (genkitPluginAdapter, error) {
		return &anthropicGenkitPlugin{}, nil
	})
}

// anthropicGenkitPlugin adapts the Genkit Anthropic plugin to the
// genkitPluginAdapter interface. The plugin reads ANTHROPIC_API_KEY (and
// optionally ANTHROPIC_BASE_URL) from the environment at Init() time.
type anthropicGenkitPlugin struct{}

// Name returns the provider prefix used by the Anthropic Genkit plugin.
func (anthropicGenkitPlugin) Name() string { return GenkitPluginAnthropic }

// Plugin returns a fresh Anthropic plugin instance for genkit.WithPlugins.
func (anthropicGenkitPlugin) Plugin() api.Plugin { return &genkitanthropic.Anthropic{} }

// BuildRequestConfig returns the Anthropic-specific config struct that
// the Genkit Anthropic plugin reads MaxTokens and Thinking from.
func (anthropicGenkitPlugin) BuildRequestConfig(maxTokens int64, thinking *port.ThinkingModeInfo) any {
	cfg := anthropic.MessageNewParams{
		MaxTokens: maxTokens,
	}
	if thinking != nil && thinking.Enabled {
		cfg.Thinking = anthropic.ThinkingConfigParamOfEnabled(thinking.BudgetTokens)
	}
	return cfg
}

// ValidateModel rejects hf:* model strings: the Anthropic plugin requires a
// native Anthropic model identifier. Users routing to a custom endpoint
// should set ANTHROPIC_BASE_URL and pass an Anthropic-style model name.
func (anthropicGenkitPlugin) ValidateModel(model string) error {
	if strings.HasPrefix(model, "hf:") {
		return fmt.Errorf("%w (model=%q)", ErrHFModelNotSupported, model)
	}
	return nil
}
