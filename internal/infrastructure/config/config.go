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
	// Defaults to 1024
	MaxTokens int

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
}

// Defaults returns a Config struct with all default values set.
func Defaults() *Config {
	return &Config{
		AIModel:           "hf:zai-org/GLM-4.6",
		MaxTokens:         1024,
		WorkingDir:        ".",
		WelcomeMessage:    "Chat with Claude (use 'ctrl+c' to quit)",
		GoodbyeMessage:    "Bye!",
		HistoryFile:       "~/.code-editing-agent-history",
		HistoryMaxEntries: 1000,
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
	if viper.IsSet("maxTokens") {
		cfg.MaxTokens = viper.GetInt("maxTokens")
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

	return cfg
}
