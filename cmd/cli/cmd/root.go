package cmd

import (
	"code-editing-agent/internal/infrastructure/config"
	signalhandler "code-editing-agent/internal/infrastructure/signal"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// global config shared between commands.
var cfg *config.Config

type configKey struct{}

type interruptHandlerKey struct{}

func contextWithConfig(ctx context.Context, c *config.Config) context.Context {
	return context.WithValue(ctx, configKey{}, c)
}

func configFromContext(ctx context.Context) *config.Config {
	if c, ok := ctx.Value(configKey{}).(*config.Config); ok {
		return c
	}
	return nil
}

func contextWithInterruptHandler(ctx context.Context, h *signalhandler.InterruptHandler) context.Context {
	return context.WithValue(ctx, interruptHandlerKey{}, h)
}

// InterruptHandlerFromContext retrieves the InterruptHandler from the given context.
// Returns nil if no handler was stored in the context.
// This is used by subcommands to access the shared interrupt handler for
// graceful shutdown handling.
func InterruptHandlerFromContext(ctx context.Context) *signalhandler.InterruptHandler {
	if h, ok := ctx.Value(interruptHandlerKey{}).(*signalhandler.InterruptHandler); ok {
		return h
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_ = args // args unused but required by cobra
		// Load configuration
		cfg = config.LoadConfig()

		// Store config in command context and package variable
		cmd.SetContext(contextWithConfig(cmd.Context(), cfg))

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
	// Create interrupt handler with 2 second timeout for double-press detection
	handler := signalhandler.NewInterruptHandler(2 * time.Second)
	handler.Start()
	defer handler.Stop()

	// Create context with the interrupt handler
	ctx := contextWithInterruptHandler(handler.Context(), handler)

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
