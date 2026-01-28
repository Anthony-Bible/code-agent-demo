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
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration values for the application.
type Config struct {
	// AIModel is the model identifier to use for AI requests.
	// Defaults to "hf:zai-org/GLM-4.6"
	AIModel string

	// MaxTokens is the maximum number of tokens to generate in AI responses.
	// Defaults to 20000
	MaxTokens int64

	// WorkingDir is the base directory for file operations.
	// All file paths are resolved relative to this directory.
	// Defaults to "." (current directory)
	WorkingDir string

	// WelcomeMessage is displayed when the chat session starts.
	// Defaults to "Chat with Claude (use 'ctrl+c' to quit)"
	WelcomeMessage string

	// GoodbyeMessage is displayed when the chat session ends.
	// Defaults to "Bye!"
	GoodbyeMessage string

	// HistoryFile is the path to the command history file.
	// Defaults to "~/.code-editing-agent-history".
	// Set to empty string to disable history persistence.
	HistoryFile string

	// HistoryMaxEntries is the maximum number of history entries to keep.
	// Defaults to 1000.
	HistoryMaxEntries int

	// ExtendedThinking enables extended thinking mode.
	// Defaults to false.
	ExtendedThinking bool

	// ThinkingBudget is the token budget for extended thinking.
	// Defaults to 10000.
	ThinkingBudget int64

	// ShowThinking determines whether to show thinking output.
	// Defaults to false.
	ShowThinking bool

	// AutoApproveSafeCommands determines whether non-dangerous bash commands
	// are automatically approved without user confirmation.
	// Dangerous commands are still blocked.
	// Defaults to false (all commands require confirmation).
	AutoApproveSafeCommands bool

	// CommandValidationMode determines how commands are validated.
	// "blacklist" (default): blocks dangerous commands, allows everything else
	// "whitelist": only allows explicitly whitelisted commands
	CommandValidationMode string

	// CommandWhitelistJSON is a JSON array of whitelist patterns with optional excludes.
	// Format: [{"pattern": "regex", "exclude": "regex", "description": "text"}]
	// Each entry must have a "pattern" field; "exclude" and "description" are optional.
	CommandWhitelistJSON string

	// AskLLMOnUnknown determines whether to ask the LLM to evaluate
	// non-whitelisted commands before blocking them.
	// Only applies in whitelist mode.
	// Defaults to true.
	AskLLMOnUnknown bool
}

// Defaults returns a Config struct with all default values set.
func Defaults() *Config {
	return &Config{
		AIModel:               "hf:zai-org/GLM-4.6",
		MaxTokens:             20000,
		WorkingDir:            ".",
		WelcomeMessage:        "Chat with Claude (use 'ctrl+c' to quit)",
		GoodbyeMessage:        "Bye!",
		HistoryFile:           "~/.code-editing-agent-history",
		HistoryMaxEntries:     1000,
		ExtendedThinking:      false,
		ThinkingBudget:        10000,
		ShowThinking:          false,
		CommandValidationMode: "blacklist",
		CommandWhitelistJSON:  "",
		AskLLMOnUnknown:       true,
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
	// Set defaults first
	cfg := Defaults()

	// Load from viper (reads flags and env vars)
	viper.SetEnvPrefix("AGENT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Override defaults with viper values
	if viper.IsSet("model") {
		cfg.AIModel = viper.GetString("model")
	}
	if viper.IsSet("max_tokens") {
		cfg.MaxTokens = viper.GetInt64("max_tokens")
	}
	if viper.IsSet("workingDir") {
		cfg.WorkingDir = viper.GetString("workingDir")
	}
	if viper.IsSet("welcomeMessage") {
		cfg.WelcomeMessage = viper.GetString("welcomeMessage")
	}
	if viper.IsSet("goodbyeMessage") {
		cfg.GoodbyeMessage = viper.GetString("goodbyeMessage")
	}
	// For history_file, we need to check if the env var is set (including empty string)
	// because empty string is valid for in-memory only mode
	if val, ok := os.LookupEnv("AGENT_HISTORY_FILE"); ok {
		cfg.HistoryFile = val
	}
	if viper.IsSet("history_max_entries") {
		val := viper.GetInt("history_max_entries")
		if val > 0 {
			cfg.HistoryMaxEntries = val
		}
	}
	if viper.IsSet("auto_approve_safe") {
		cfg.AutoApproveSafeCommands = viper.GetBool("auto_approve_safe")
	}
	if viper.IsSet("thinking.enabled") {
		cfg.ExtendedThinking = viper.GetBool("thinking.enabled")
	}
	if viper.IsSet("thinking.budget") {
		budget := viper.GetInt64("thinking.budget")
		switch {
		case budget <= 0:
			cfg.ThinkingBudget = 10000
		case budget < 1024:
			cfg.ThinkingBudget = 1024
		default:
			cfg.ThinkingBudget = budget
		}
	}
	if viper.IsSet("thinking.show") {
		cfg.ShowThinking = viper.GetBool("thinking.show")
	}

	// Command validation mode: "blacklist" (default) or "whitelist"
	if viper.IsSet("command_validation_mode") {
		cfg.CommandValidationMode = viper.GetString("command_validation_mode")
	}

	// Command whitelist: JSON array of patterns with optional excludes
	if val, ok := os.LookupEnv("AGENT_COMMAND_WHITELIST_JSON"); ok && val != "" {
		cfg.CommandWhitelistJSON = val
	}

	// Ask LLM on unknown: whether to ask LLM before blocking non-whitelisted commands
	if viper.IsSet("ask_llm_on_unknown") {
		cfg.AskLLMOnUnknown = viper.GetBool("ask_llm_on_unknown")
	}

	return cfg
}
