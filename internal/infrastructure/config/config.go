// Package config provides configuration management for the code editing agent.
// It uses viper for loading configuration from command-line flags, environment variables,
// and optionally config files.
//
// Configuration priority (highest to lowest):
// 1. Command-line flags
// 2. Environment variables (with AGENT_ prefix)
// 3. Config file (if specified)
// 4. Defaults
package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration values for the application.
type Config struct {
	// AIModel is the model identifier to use for AI requests.
	// Defaults to "hf:zai-org/GLM-4.7"
	AIModel string `mapstructure:"model"`

	// MaxTokens is the maximum number of tokens to generate in AI responses.
	// Defaults to 20000
	MaxTokens int64 `mapstructure:"max_tokens"`

	// WorkingDir is the base directory for file operations.
	// All file paths are resolved relative to this directory.
	// Defaults to "." (current directory)
	WorkingDir string `mapstructure:"working_dir"`

	// UI contains configuration for the user interface.
	UI UIConfig `mapstructure:"ui"`

	// History contains configuration for command history.
	History HistoryConfig `mapstructure:"history"`

	// Thinking contains configuration for extended AI thinking.
	Thinking ThinkingConfig `mapstructure:"thinking"`

	// Safety contains configuration for command safety and validation.
	Safety SafetyConfig `mapstructure:"safety"`

	// Serve contains configuration for the webhook server command.
	Serve ServeConfig `mapstructure:"serve"`

	// Log contains logging configuration.
	Log LogConfig `mapstructure:"log"`

	// CompactionThreshold is the total token count at which the conversation
	// is automatically compacted (summarized) to manage context window size.
	// Defaults to 160000.
	CompactionThreshold int64 `mapstructure:"compaction_threshold"`
}

// UIConfig holds UI-related configuration.
type UIConfig struct {
	// WelcomeMessage is displayed when the chat session starts.
	WelcomeMessage string `mapstructure:"welcome_message"`

	// GoodbyeMessage is displayed when the chat session ends.
	GoodbyeMessage string `mapstructure:"goodbye_message"`
}

// HistoryConfig holds history-related configuration.
type HistoryConfig struct {
	// File is the path to the command history file.
	File string `mapstructure:"file"`

	// MaxEntries is the maximum number of history entries to keep.
	MaxEntries int `mapstructure:"max_entries"`
}

// ThinkingConfig holds configuration for extended AI thinking.
type ThinkingConfig struct {
	// Enabled enables extended thinking mode.
	Enabled bool `mapstructure:"enabled"`

	// Budget is the token budget for extended thinking.
	Budget int64 `mapstructure:"budget"`

	// Show determines whether to show thinking output.
	Show bool `mapstructure:"show"`
}

// ServeConfig holds configuration for the webhook server command.
type ServeConfig struct {
	// Addr is the address to listen on (e.g., ":8080", "0.0.0.0:9090").
	Addr string `mapstructure:"addr"`

	// ConfigPath is the path to the alert sources config file.
	ConfigPath string `mapstructure:"config_path"`
}

// LogConfig holds logging-related configuration.
type LogConfig struct {
	// Level sets the minimum log level. Accepted values (case-insensitive):
	// "debug", "info", "warn", "error". Defaults to "info".
	Level string `mapstructure:"level"`
	// Format selects the output format. Accepted values: "json", "text".
	// Defaults to "text".
	Format string `mapstructure:"format"`
}

// SafetyConfig holds safety and validation configuration.
type SafetyConfig struct {
	// AutoApproveSafeCommands determines whether non-dangerous bash commands
	// are automatically approved without user confirmation.
	AutoApproveSafeCommands bool `mapstructure:"auto_approve_safe"`

	// CommandValidationMode determines how commands are validated.
	// "blacklist" (default) or "whitelist".
	CommandValidationMode string `mapstructure:"command_validation_mode"`

	// CommandWhitelistJSON is a JSON array of whitelist patterns.
	CommandWhitelistJSON string `mapstructure:"command_whitelist_json"`

	// AskLLMOnUnknown determines whether to ask the LLM to evaluate
	// non-whitelisted commands before blocking them.
	AskLLMOnUnknown bool `mapstructure:"ask_llm_on_unknown"`

	// CommandWhitelistOverride determines whether custom whitelist patterns
	// replace the defaults entirely (true) or extend them (false).
	CommandWhitelistOverride bool `mapstructure:"command_whitelist_override"`
}

// Defaults returns a Config struct with all default values set.
func Defaults() *Config {
	return &Config{
		AIModel:    "hf:zai-org/GLM-4.7",
		MaxTokens:  20000,
		WorkingDir: ".",
		UI: UIConfig{
			WelcomeMessage: "Chat with Claude (use 'ctrl+c' to quit)",
			GoodbyeMessage: "Bye!",
		},
		History: HistoryConfig{
			File:       "~/.code-agent-demo-history",
			MaxEntries: 1000,
		},
		Thinking: ThinkingConfig{
			Enabled: false,
			Budget:  10000,
			Show:    false,
		},
		Safety: SafetyConfig{
			AutoApproveSafeCommands:  false,
			CommandValidationMode:    "blacklist",
			CommandWhitelistJSON:     "",
			AskLLMOnUnknown:          true,
			CommandWhitelistOverride: false,
		},
		Serve: ServeConfig{
			Addr:       ":8080",
			ConfigPath: "config/alert-sources.yaml",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		CompactionThreshold: 160000,
	}
}

// LoadConfig loads and returns the configuration from viper.
// It sets up environment variable bindings with the AGENT_ prefix.
//
// The caller is expected to have set up viper with BindPFlag() calls
// for command-line flags before calling this function.
//
// Returns:
//   - *Config: The loaded configuration
func LoadConfig() *Config {
	// Use ExperimentalBindStruct to automatically discover env vars based on struct tags
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())

	// Set defaults first
	cfg := Defaults()

	// Configure environment variable loading
	v.SetEnvPrefix("AGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind from global viper to pick up flags already bound there
	// This is a bridge between cobra's use of global viper and our local instance
	for _, key := range viper.AllKeys() {
		if viper.IsSet(key) {
			v.Set(key, viper.Get(key))
		}
	}

	// Unmarshal into the config struct
	if err := v.Unmarshal(cfg); err != nil {
		slog.Error("failed to unmarshal config, using defaults", "error", err) //nolint:sloglint // no injected logger at config load time
	}

	// For history.file, we need to check if the env var is set (including empty string)
	// because empty string is valid for in-memory only mode
	if val, ok := os.LookupEnv("AGENT_HISTORY_FILE"); ok {
		cfg.History.File = val
	}

	// Post-processing and validation
	if cfg.History.MaxEntries <= 0 {
		cfg.History.MaxEntries = 1000
	}

	switch {
	case cfg.Thinking.Budget <= 0:
		cfg.Thinking.Budget = 10000
	case cfg.Thinking.Budget < 1024:
		cfg.Thinking.Budget = 1024
	}

	cfg.CompactionThreshold = loadCompactionThreshold(v, cfg.CompactionThreshold)

	return cfg
}

// loadCompactionThreshold reads the compaction threshold from viper, returning
// the provided default if not set or invalid.
func loadCompactionThreshold(v *viper.Viper, defaultVal int64) int64 {
	if !v.IsSet("compaction_threshold") {
		return defaultVal
	}
	threshold := v.GetInt64("compaction_threshold")
	if threshold < 10000 {
		return defaultVal
	}
	return threshold
}
