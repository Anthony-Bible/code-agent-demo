package cmd

import (
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/infrastructure/adapter/alert"
	"code-editing-agent/internal/infrastructure/adapter/webhook"
	"code-editing-agent/internal/infrastructure/config"
	signalhandler "code-editing-agent/internal/infrastructure/signal"
	"context"
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

// registerAlertSources registers alert sources from config with the source manager.
func registerAlertSources(webhookCfg *config.WebhookServerConfig, container *config.Container) error {
	sourceManager := container.AlertSourceManager()
	ui := container.UIAdapter()

	for _, srcCfg := range webhookCfg.Sources {
		if srcCfg.Type != "prometheus" {
			return errors.New("unknown source type: " + srcCfg.Type)
		}

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
	}
	return nil
}

// setupSkillReloadHandler creates and starts a SIGHUP handler for skill hot-reload.
func setupSkillReloadHandler(container *config.Container) *signalhandler.ReloadHandler {
	ui := container.UIAdapter()
	skillManager := container.SkillManager()

	reloadHandler := signalhandler.NewReloadHandler(func(reloadCtx context.Context) {
		_ = ui.DisplaySystemMessage("")
		_ = ui.DisplaySystemMessage("Received SIGHUP - reloading skills...")

		result, err := skillManager.DiscoverSkills(reloadCtx)
		if err != nil {
			_ = ui.DisplaySystemMessage("Error discovering skills: " + err.Error())
			return
		}

		_ = ui.DisplaySystemMessage(fmt.Sprintf("Discovered %d skills:", result.TotalCount))
		for _, skill := range result.Skills {
			status := "inactive"
			if skill.IsActive {
				status = "active"
			}
			_ = ui.DisplaySystemMessage(fmt.Sprintf("  - %s (%s, %s)",
				skill.Name, skill.SourceType, status))
		}
		_ = ui.DisplaySystemMessage("")
	})
	reloadHandler.Start()
	return reloadHandler
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

	ui := container.UIAdapter()

	// Register alert sources from config
	if err := registerAlertSources(webhookCfg, container); err != nil {
		return err
	}

	sourceManager := container.AlertSourceManager()

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

	// Set up SIGHUP handler for skill hot-reload
	reloadHandler := setupSkillReloadHandler(container)
	defer reloadHandler.Stop()

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
	_ = ui.DisplaySystemMessage("Send SIGHUP to reload skills")

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
