package cmd

import (
	"code-editing-agent/internal/infrastructure/config"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// global config shared between commands.
var cfg *config.Config

type configKey struct{}

func contextWithConfig(ctx context.Context, c *config.Config) context.Context {
	return context.WithValue(ctx, configKey{}, c)
}

func configFromContext(ctx context.Context) *config.Config {
	if c, ok := ctx.Value(configKey{}).(*config.Config); ok {
		return c
	}
	return nil
}

// executeChat is the function that runs the chat loop
// This is set by chat.go during initialization.
var executeChat func(cmd *cobra.Command, args []string) error

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "code-editing-agent",
	Short: "AI-powered code editing assistant",
	Long: `Code Editing Agent is an AI-powered assistant that helps you
write, edit, and understand code through an interactive chat interface.

It uses the Claude AI to provide intelligent code suggestions,
refactoring options, and explanations.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg = config.LoadConfig()

		// Store config in command context and package variable
		cmd.SetContext(contextWithConfig(cmd.Context(), cfg))

		// Display welcome message
		fmt.Println(cfg.WelcomeMessage)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Execute the chat command by default
		if executeChat != nil {
			return executeChat(cmd, args)
		}
		return errors.New("chat functionality not initialized")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	// Handle graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Update root command context
	rootCmd.SetContext(ctx)

	return rootCmd.Execute()
}

// GetConfig retrieves the configuration from the command context.
func GetConfig(cmd *cobra.Command) *config.Config {
	// First try context, fall back to package variable
	if c := configFromContext(cmd.Context()); c != nil {
		return c
	}
	return cfg
}

func init() {
	// Define flags
	rootCmd.PersistentFlags().String("model", "hf:zai-org/GLM-4.6", "AI model to use for requests")
	rootCmd.PersistentFlags().StringP("dir", "d", ".", "Working directory for file operations")
	rootCmd.PersistentFlags().Int("max-tokens", 1024, "Maximum tokens to generate in AI responses")

	// Bind flags to viper
	if err := viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to bind model flag: %v\n", err)
	}
	if err := viper.BindPFlag("workingDir", rootCmd.PersistentFlags().Lookup("dir")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to bind dir flag: %v\n", err)
	}
	if err := viper.BindPFlag("maxTokens", rootCmd.PersistentFlags().Lookup("max-tokens")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to bind max-tokens flag: %v\n", err)
	}
}
