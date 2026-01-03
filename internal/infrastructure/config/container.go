// Package config provides a dependency injection container for wiring together
// all the components of the application following hexagonal architecture principles.
package config

import (
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/domain/service"
	"code-editing-agent/internal/infrastructure/adapter/ai"
	"code-editing-agent/internal/infrastructure/adapter/alert"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/investigation"
	"code-editing-agent/internal/infrastructure/adapter/skill"
	"code-editing-agent/internal/infrastructure/adapter/tool"
	"code-editing-agent/internal/infrastructure/adapter/ui"
	"code-editing-agent/internal/infrastructure/adapter/webhook"
	"context"
	"errors"
	"path/filepath"
	"time"

	appsvc "code-editing-agent/internal/application/service"
)

// investigationStoreAdapter adapts FileInvestigationStore to the usecase.InvestigationStoreWriter interface.
// This is needed because FileInvestigationStore uses concrete *service.InvestigationRecord types
// while the usecase interface uses InvestigationRecordData interface types.
type investigationStoreAdapter struct {
	store *investigation.FileInvestigationStore
}

func (a *investigationStoreAdapter) Store(ctx context.Context, inv usecase.InvestigationRecordData) error {
	stub := appsvc.NewInvestigationRecordWithResult(
		inv.ID(), inv.AlertID(), inv.SessionID(), inv.Status(),
		inv.StartedAt(), inv.CompletedAt(),
		inv.Findings(), inv.ActionsTaken(), inv.Duration(),
		inv.Confidence(), inv.Escalated(), inv.EscalateReason(),
	)
	return a.store.Store(ctx, stub)
}

func (a *investigationStoreAdapter) Get(ctx context.Context, id string) (usecase.InvestigationRecordData, error) {
	return a.store.Get(ctx, id)
}

func (a *investigationStoreAdapter) Update(ctx context.Context, inv usecase.InvestigationRecordData) error {
	stub := appsvc.NewInvestigationRecordWithResult(
		inv.ID(), inv.AlertID(), inv.SessionID(), inv.Status(),
		inv.StartedAt(), inv.CompletedAt(),
		inv.Findings(), inv.ActionsTaken(), inv.Duration(),
		inv.Confidence(), inv.Escalated(), inv.EscalateReason(),
	)
	return a.store.Update(ctx, stub)
}

// Container holds all application dependencies wired together.
// It provides a single point of access to all services and ports,
// following the dependency injection pattern for clean architecture.
//
// The container is responsible for:
// - Creating and initializing all adapters (infrastructure layer)
// - Creating domain services (domain layer)
// - Creating application services (application layer)
// - Providing accessors for all dependencies.
type Container struct {
	config               *Config
	chatService          *appsvc.ChatService
	convService          *service.ConversationService
	fileManager          port.FileManager
	uiAdapter            port.UserInterface
	aiAdapter            port.AIProvider
	toolExecutor         port.ToolExecutor
	skillManager         port.SkillManager
	alertSourceManager   port.AlertSourceManager
	investigationUseCase *usecase.AlertInvestigationUseCase
	webhookAdapter       *webhook.HTTPAdapter
}

// NewContainer creates a new DI container and wires all dependencies.
//
// The wiring order is:
// 1. Create infrastructure adapters (infra layer)
// 2. Create domain services (domain layer)
// 3. Create application services (application layer)
//
// Parameters:
//   - cfg: Configuration object containing application settings
//
// Returns:
//   - *Container: A fully wired dependency container
//   - error: An error if any dependency creation fails
func NewContainer(cfg *Config) (*Container, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Step 1: Create infrastructure adapters
	// Note: order matters - skillManager must be created before aiAdapter
	fileManager := file.NewLocalFileManager(cfg.WorkingDir)
	uiAdapter := ui.NewCLIAdapterWithHistory(cfg.HistoryFile, cfg.HistoryMaxEntries)
	skillManager := skill.NewLocalSkillManager()
	aiAdapter := ai.NewAnthropicAdapter(cfg.AIModel, skillManager)

	// Create base executor and wrap with planning decorator
	baseExecutor := tool.NewExecutorAdapter(fileManager)
	baseExecutor.SetSkillManager(skillManager)
	toolExecutor := tool.NewPlanningExecutorAdapter(baseExecutor, fileManager, cfg.WorkingDir)

	// Set up bash command confirmation callback
	// Behavior depends on cfg.AutoApproveSafeCommands flag
	if cfg.AutoApproveSafeCommands {
		// Auto-approve safe commands, block dangerous ones (headless mode)
		toolExecutor.SetCommandConfirmationCallback(
			func(command string, isDangerous bool, reason string, description string) bool {
				if isDangerous {
					// Block dangerous commands in headless mode
					_ = uiAdapter.DisplaySystemMessage(
						"[BLOCKED] " + description + ": " + command + " (reason: " + reason + ")",
					)
					return false
				}
				// Log auto-approved command
				_ = uiAdapter.DisplaySystemMessage("[AUTO-APPROVED] " + description + ": " + command)
				return true // Auto-approve safe commands
			},
		)
	} else {
		// Default behavior: prompt user before executing any bash command
		toolExecutor.SetCommandConfirmationCallback(
			func(command string, isDangerous bool, reason, description string) bool {
				return uiAdapter.ConfirmBashCommand(command, isDangerous, reason, description)
			},
		)
	}

	// Set up plan mode confirmation callback
	// This prompts the user when the agent wants to enter plan mode
	toolExecutor.SetPlanModeConfirmCallback(func(reason string) bool {
		return uiAdapter.ConfirmBashCommand(
			reason,
			false,
			"enter_plan_mode",
			"Agent wants to enter plan mode:",
		)
	})

	// Step 2: Create domain service (ConversationService)
	// Note: ConversationService directly uses concrete adapter types
	convService, err := service.NewConversationService(aiAdapter, toolExecutor)
	if err != nil {
		return nil, err
	}

	// Step 3: Create application service (ChatService)
	// NewChatServiceFromDomain directly accepts concrete adapter types
	chatService, err := appsvc.NewChatServiceFromDomain(
		convService,
		uiAdapter,
		aiAdapter,
		toolExecutor,
		fileManager,
	)
	if err != nil {
		return nil, err
	}

	// Step 4: Create investigation and alert handling components
	investigationUseCase, alertSourceManager, webhookAdapter, err := createInvestigationComponents(
		cfg, convService, toolExecutor, skillManager,
	)
	if err != nil {
		return nil, err
	}

	return &Container{
		config:               cfg,
		chatService:          chatService,
		convService:          convService,
		fileManager:          fileManager,
		uiAdapter:            uiAdapter,
		aiAdapter:            aiAdapter,
		toolExecutor:         toolExecutor,
		skillManager:         skillManager,
		alertSourceManager:   alertSourceManager,
		investigationUseCase: investigationUseCase,
		webhookAdapter:       webhookAdapter,
	}, nil
}

// createInvestigationComponents sets up the investigation framework including
// the use case, alert handler, source manager, and webhook adapter.
func createInvestigationComponents(
	cfg *Config,
	convService *service.ConversationService,
	toolExecutor port.ToolExecutor,
	skillManager port.SkillManager,
) (*usecase.AlertInvestigationUseCase, port.AlertSourceManager, *webhook.HTTPAdapter, error) {
	// Configure investigation safety limits
	invConfig := usecase.AlertInvestigationUseCaseConfig{
		MaxActions:    20,
		MaxDuration:   15 * time.Minute,
		MaxConcurrent: 5,
		AllowedTools: []string{
			"bash", "read_file", "list_files",
			"activate_skill", "complete_investigation", "escalate_investigation",
			"report_investigation",
		},
		BlockedCommands: []string{"rm -rf", "dd if=", "mkfs"},
	}
	investigationUseCase := usecase.NewAlertInvestigationUseCaseWithConfig(invConfig)

	// Wire core dependencies
	investigationUseCase.SetConversationService(convService)
	investigationUseCase.SetToolExecutor(toolExecutor)
	investigationUseCase.SetSkillManager(skillManager)

	// Wire prompt builder (generic builder for all alert types)
	promptRegistry := usecase.NewPromptBuilderRegistry()
	_ = promptRegistry.Register(usecase.NewGenericPromptBuilder())
	investigationUseCase.SetPromptBuilderRegistry(promptRegistry)

	// Wire escalation handler
	investigationUseCase.SetEscalationHandler(usecase.NewLogEscalationHandler())

	// Wire investigation store for persistence
	storePath := filepath.Join(cfg.WorkingDir, ".agent", "investigations")
	fileStore, err := investigation.NewFileInvestigationStore(storePath)
	if err != nil {
		return nil, nil, nil, err
	}
	investigationUseCase.SetInvestigationStore(&investigationStoreAdapter{store: fileStore})

	// Create alert handler with severity-based routing
	alertHandler := usecase.NewAlertHandler(investigationUseCase, usecase.AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  false,
	})

	// Create alert source manager
	alertSourceManager := alert.NewLocalAlertSourceManager()
	alertSourceManager.SetAlertHandler(alertHandler.HandleEntityAlert)

	// Create webhook HTTP adapter
	webhookAdapter := webhook.NewHTTPAdapter(alertSourceManager, webhook.DefaultConfig())
	webhookAdapter.SetAlertHandler(alertHandler.HandleEntityAlert)

	return investigationUseCase, alertSourceManager, webhookAdapter, nil
}

// ChatService returns the application chat service.
// This is the main entry point for chat operations.
func (c *Container) ChatService() *appsvc.ChatService {
	return c.chatService
}

// Config returns the application configuration.
func (c *Container) Config() *Config {
	return c.config
}

// ConversationService returns the domain conversation service.
// Useful for testing and advanced use cases.
func (c *Container) ConversationService() *service.ConversationService {
	return c.convService
}

// FileManager returns the file manager port implementation.
// Useful for direct file operations outside of chat sessions.
func (c *Container) FileManager() port.FileManager {
	return c.fileManager
}

// UIAdapter returns the user interface port implementation.
// Useful for direct UI operations.
func (c *Container) UIAdapter() port.UserInterface {
	return c.uiAdapter
}

// AIAdapter returns the AI provider port implementation.
// Useful for direct AI interactions.
func (c *Container) AIAdapter() port.AIProvider {
	return c.aiAdapter
}

// ToolExecutor returns the tool executor port implementation.
// Useful for direct tool execution outside of chat sessions.
func (c *Container) ToolExecutor() port.ToolExecutor {
	return c.toolExecutor
}

// SkillManager returns the skill manager port implementation.
// Useful for direct skill discovery and activation operations.
func (c *Container) SkillManager() port.SkillManager {
	return c.skillManager
}

// AlertSourceManager returns the alert source manager port implementation.
// Useful for registering and managing alert sources.
func (c *Container) AlertSourceManager() port.AlertSourceManager {
	return c.alertSourceManager
}

// InvestigationUseCase returns the alert investigation use case.
// Useful for managing alert investigations.
func (c *Container) InvestigationUseCase() *usecase.AlertInvestigationUseCase {
	return c.investigationUseCase
}

// WebhookAdapter returns the webhook HTTP adapter.
// Useful for starting the webhook server to receive alerts via HTTP.
func (c *Container) WebhookAdapter() *webhook.HTTPAdapter {
	return c.webhookAdapter
}
