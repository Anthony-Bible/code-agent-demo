package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/anthony-bible/code-agent-demo/internal/application/usecase"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/alert"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/webhook"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/config"
	signalhandler "github.com/anthony-bible/code-agent-demo/internal/infrastructure/signal"

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
  code-agent-demo serve --addr :8080
  code-agent-demo serve --config config/alert-sources.yaml

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

	// Bind flags to viper
	bindings := map[string]string{
		"addr":              "serve.addr",
		"config":            "serve.config_path",
		"auto-approve-safe": "safety.auto_approve_safe",
	}
	for flagName, viperKey := range bindings {
		if err := viper.BindPFlag(viperKey, serveCmd.Flags().Lookup(flagName)); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to bind %s flag: %v\n", flagName, err)
		}
	}
}

// registerAlertSources registers alert sources from config with the source manager.
func registerAlertSources(webhookCfg *config.WebhookServerConfig, container *config.Container) error {
	sourceManager := container.AlertSourceManager()
	ui := container.UIAdapter()

	// Create registry with builtin factories
	registry := alert.NewSourceRegistry()
	registry.RegisterBuiltinFactories()

	for _, srcCfg := range webhookCfg.Sources {
		source, err := registry.CreateSource(alert.SourceConfig{
			Type:        srcCfg.Type,
			Name:        srcCfg.Name,
			WebhookPath: srcCfg.WebhookPath,
			Extra:       srcCfg.Extra,
		})
		if err != nil {
			return fmt.Errorf("failed to create source %s: %w", srcCfg.Name, err)
		}

		if err := sourceManager.RegisterSource(source); err != nil {
			return err
		}

		_ = ui.DisplaySystemMessage(
			"Registered alert source: " + srcCfg.Name + " (type=" + srcCfg.Type + ", path=" + srcCfg.WebhookPath + ")",
		)
	}
	return nil
}

// applyBasicAuth registers Basic Auth credentials from the config with the webhook adapter.
// Sources without a basic_auth block are left unauthenticated (no change to their endpoint).
// This function is intentionally separate from registerAlertSources so credentials from the
// parsed config are copied into the adapter for the matching webhook path without being logged here.
func applyBasicAuth(webhookCfg *config.WebhookServerConfig, adapter *webhook.HTTPAdapter, logger port.Logger) {
	for _, srcCfg := range webhookCfg.Sources {
		if srcCfg.BasicAuth == nil {
			continue
		}
		if srcCfg.BasicAuth.Username == "" || srcCfg.BasicAuth.Password == "" {
			logger.Warn("basic_auth block present but username or password is empty — endpoint will NOT be protected",
				"path", srcCfg.WebhookPath)
			continue
		}
		adapter.SetBasicAuth(srcCfg.WebhookPath, srcCfg.BasicAuth.Username, srcCfg.BasicAuth.Password)
	}
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

	// Get serve config from unified config
	addr := cfg.Serve.Addr
	configPath := cfg.Serve.ConfigPath

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
	}, container.Logger())

	// Create webhook adapter with configured address
	adapterCfg := webhook.DefaultConfig()
	adapterCfg.Addr = addr
	webhookAdapter := webhook.NewHTTPAdapter(sourceManager, adapterCfg, container.Logger())
	webhookAdapter.SetAsyncAlertHandler(alertHandler.HandleEntityAlertAsync, alertHandler.RunEntityAlertInvestigation)

	// Register per-source Basic Auth credentials with the webhook adapter.
	// Credentials are applied after adapter creation so they are not logged.
	// They are stored in the adapter's internal map and in the parsed config.
	applyBasicAuth(webhookCfg, webhookAdapter, container.Logger())

	// Set up SIGHUP handler for skill hot-reload
	reloadHandler := setupSkillReloadHandler(container)
	defer reloadHandler.Stop()

	// Print startup info
	_ = ui.DisplaySystemMessage("")
	_ = ui.DisplaySystemMessage("Starting webhook server on " + addr)
	_ = ui.DisplaySystemMessage("Health check: GET http://localhost" + addr + "/health")
	_ = ui.DisplaySystemMessage("Ready check:  GET http://localhost" + addr + "/ready")
	for _, srcCfg := range webhookCfg.Sources {
		authTag := ""
		if srcCfg.BasicAuth != nil && srcCfg.BasicAuth.Username != "" && srcCfg.BasicAuth.Password != "" {
			authTag = " [basic auth]"
		}
		_ = ui.DisplaySystemMessage("Webhook:      POST http://localhost" + addr + srcCfg.WebhookPath + authTag)
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
