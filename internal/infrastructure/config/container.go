// Package config provides a dependency injection container for wiring together
// all the components of the application following hexagonal architecture principles.
package config

import (
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/domain/service"
	"code-editing-agent/internal/infrastructure/adapter/ai"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/tool"
	"code-editing-agent/internal/infrastructure/adapter/ui"

	appsvc "code-editing-agent/internal/application/service"
)

// Container holds all application dependencies wired together.
// It provides a single point of access to all services and ports,
// following the dependency injection pattern for clean architecture.
//
// The container is responsible for:
// - Creating and initializing all adapters (infrastructure layer)
// - Creating domain services (domain layer)
// - Creating application services (application layer)
// - Providing accessors for all dependencies
type Container struct {
	config          *Config
	chatService     *appsvc.ChatService
	conversationSvc *service.ConversationService
	fileManager     port.FileManager
	uiAdapter       port.UserInterface
	aiAdapter       port.AIProvider
	toolExecutor    port.ToolExecutor
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
	// Step 1: Create infrastructure adapters
	fileManager := file.NewLocalFileManager(cfg.WorkingDir)
	uiAdapter := ui.NewCLIAdapter()
	aiAdapter := ai.NewAnthropicAdapter(cfg.AIModel)
	toolExecutor := tool.NewExecutorAdapter(fileManager)

	// Set up bash command confirmation callback
	// This prompts the user before executing any bash command
	toolExecutor.SetCommandConfirmationCallback(
		func(command string, isDangerous bool, reason, description string) bool {
			return uiAdapter.ConfirmBashCommand(command, isDangerous, reason, description)
		},
	)

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

	return &Container{
		config:          cfg,
		chatService:     chatService,
		conversationSvc: convService,
		fileManager:     fileManager,
		uiAdapter:       uiAdapter,
		aiAdapter:       aiAdapter,
		toolExecutor:    toolExecutor,
	}, nil
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
	return c.conversationSvc
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
