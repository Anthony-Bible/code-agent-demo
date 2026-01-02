package cmd

import (
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/infrastructure/adapter/alert"
	"code-editing-agent/internal/infrastructure/adapter/webhook"
	"code-editing-agent/internal/infrastructure/config"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command.
//
//nolint:gochecknoglobals // cobra command pattern requires global variable
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the webhook server",
	Long: `Start an HTTP server to receive webhook alerts from external systems.

The server exposes endpoints for:
- Health checks: GET /health
- Readiness checks: GET /ready
- Webhook receivers: POST /alerts/{source-path}

Example:
  code-editing-agent serve --addr :8080
  code-editing-agent serve --config config/alert-sources.yaml

Alert sources are registered from the config file and receive webhooks
at their configured paths. For example, a Prometheus Alertmanager source
configured with webhook_path "/alerts/prometheus" receives alerts at
POST /alerts/prometheus.`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("addr", ":8080", "Address to listen on (e.g., :8080, 0.0.0.0:9090)")
	serveCmd.Flags().String("config", "config/alert-sources.yaml", "Path to alert sources config file")
	serveCmd.Flags().
		Bool("auto-approve-safe", false, "Auto-approve non-dangerous bash commands (dangerous commands are blocked)")

	// Bind flag to viper
	if err := viper.BindPFlag("auto_approve_safe", serveCmd.Flags().Lookup("auto-approve-safe")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to bind auto-approve-safe flag: %v\n", err)
	}
}

// runServe executes the serve command.
func runServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := GetConfig(cmd)

	// Get command flags
	addr, _ := cmd.Flags().GetString("addr")
	configPath, _ := cmd.Flags().GetString("config")

	// Load alert sources config
	webhookCfg, err := config.LoadAlertSourcesConfigWithDefaults(configPath)
	if err != nil {
		return err
	}

	// Override addr from config if not set via flag and config has it
	if webhookCfg.Addr != "" && !cmd.Flags().Changed("addr") {
		addr = webhookCfg.Addr
	}

	// Initialize the dependency container
	container, err := config.NewContainer(cfg)
	if err != nil {
		return err
	}

	// Get UI adapter for output
	ui := container.UIAdapter()

	// Get alert source manager and register sources from config
	sourceManager := container.AlertSourceManager()

	for _, srcCfg := range webhookCfg.Sources {
		switch srcCfg.Type {
		case "prometheus":
			promSource, err := alert.NewPrometheusSource(alert.SourceConfig{
				Type:        srcCfg.Type,
				Name:        srcCfg.Name,
				WebhookPath: srcCfg.WebhookPath,
				Extra:       srcCfg.Extra,
			})
			if err != nil {
				return err
			}
			if err := sourceManager.RegisterSource(promSource); err != nil {
				return err
			}
			_ = ui.DisplaySystemMessage(
				"Registered alert source: " + srcCfg.Name + " (type=" + srcCfg.Type + ", path=" + srcCfg.WebhookPath + ")",
			)
		default:
			return errors.New("unknown source type: " + srcCfg.Type)
		}
	}

	// Create alert handler for dispatching alerts to investigation use case
	alertHandler := usecase.NewAlertHandler(container.InvestigationUseCase(), usecase.AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  false,
	})

	// Create webhook adapter with configured address
	webhookAdapter := webhook.NewHTTPAdapter(sourceManager, webhook.HTTPAdapterConfig{
		Addr:            addr,
		ReadTimeout:     webhook.DefaultConfig().ReadTimeout,
		WriteTimeout:    webhook.DefaultConfig().WriteTimeout,
		ShutdownTimeout: webhook.DefaultConfig().ShutdownTimeout,
	})
	webhookAdapter.SetAsyncAlertHandler(alertHandler.HandleEntityAlertAsync, alertHandler.RunEntityAlertInvestigation)

	// Print startup info
	_ = ui.DisplaySystemMessage("")
	_ = ui.DisplaySystemMessage("Starting webhook server on " + addr)
	_ = ui.DisplaySystemMessage("Health check: GET http://localhost" + addr + "/health")
	_ = ui.DisplaySystemMessage("Ready check:  GET http://localhost" + addr + "/ready")
	for _, srcCfg := range webhookCfg.Sources {
		_ = ui.DisplaySystemMessage("Webhook:      POST http://localhost" + addr + srcCfg.WebhookPath)
	}
	_ = ui.DisplaySystemMessage("")
	_ = ui.DisplaySystemMessage("Press Ctrl+C to stop")

	// Get interrupt handler for graceful shutdown
	handler := InterruptHandlerFromContext(ctx)
	if handler != nil {
		go func() {
			<-handler.FirstPress()
			_ = ui.DisplaySystemMessage("\nInitiating graceful shutdown...")
		}()
	}

	// Start the webhook server (blocks until context cancelled)
	if err := webhookAdapter.Start(ctx); err != nil {
		return err
	}

	_ = ui.DisplaySystemMessage("Server stopped")
	return nil
}
