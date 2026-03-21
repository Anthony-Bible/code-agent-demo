package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/config"
	signalhandler "github.com/anthony-bible/code-agent-demo/internal/infrastructure/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startPprof starts a pprof HTTP server on localhost:6060 using an explicit mux.
func startPprof() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	srv := &http.Server{
		Addr:              "localhost:6060",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.ListenAndServe() }()
}

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

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "code-agent-demo",
	Short: "AI-powered code editing assistant",
	Long: `Code Editing Agent is an AI-powered assistant that helps you
investigate alerts and run automated AI workflows.

It uses AI to provide intelligent code analysis,
alert investigation, and root cause analysis.

Use "code-agent-demo serve" to start the webhook server.`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Load configuration
		cfg = config.LoadConfig()

		// Store config in command context and package variable
		cmd.SetContext(contextWithConfig(cmd.Context(), cfg))

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	startPprof()

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
	pflags := rootCmd.PersistentFlags()
	pflags.String("model", "hf:zai-org/GLM-4.7", "AI model to use for requests")
	pflags.StringP("dir", "d", ".", "Working directory for file operations")
	pflags.Int64("max-tokens", 20000, "Maximum tokens to generate in AI responses")
	pflags.Bool("thinking", false, "Enable extended thinking")
	pflags.Int64("thinking-budget", 10000, "Token budget for thinking (min 1024)")
	pflags.Bool("show-thinking", false, "Display thinking content")

	// Bind flags to viper
	// Map flag names to the internal config keys used by viper/LoadConfig
	bindings := map[string]string{
		"model":           "model",
		"dir":             "working_dir",
		"max-tokens":      "max_tokens",
		"thinking":        "thinking.enabled",
		"thinking-budget": "thinking.budget",
		"show-thinking":   "thinking.show",
	}

	for flagName, viperKey := range bindings {
		if err := viper.BindPFlag(viperKey, pflags.Lookup(flagName)); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to bind %s flag: %v\n", flagName, err)
		}
	}
}
