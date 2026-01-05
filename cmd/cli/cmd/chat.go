package cmd

import (
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/infrastructure/config"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// chatCmd represents the chat command.
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive chat session with the AI assistant.
You can ask questions about your code, request edits, or get explanations.

Press Ctrl+C to exit the chat session.`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	// Set the executeChat function so rootCmd can delegate to it
	executeChat = runChat
}

// inputResult holds the result from the async input goroutine.
type inputResult struct {
	text string
	ok   bool
}

// runChat executes the chat command.
func runChat(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := GetConfig(cmd)

	// Initialize the dependency container
	container, err := config.NewContainer(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}

	chatService := container.ChatService()
	uiAdapter := container.UIAdapter()
	subagentManager := container.SubagentManager()

	// Create a new session
	startResp, err := chatService.StartSession(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to start chat session: %w", err)
	}
	sessionID := startResp.SessionID

	// Initialize thinking mode from config if enabled
	if cfg.ExtendedThinking {
		convSvc := container.ConversationService()
		thinkingInfo := port.ThinkingModeInfo{
			Enabled:      true,
			BudgetTokens: cfg.ThinkingBudget,
			ShowThinking: cfg.ShowThinking,
		}
		_ = convSvc.SetThinkingMode(sessionID, thinkingInfo)
	}

	// Discover and display available subagents
	if subagentManager != nil {
		result, err := subagentManager.DiscoverAgents(ctx)
		if err == nil && result.TotalCount > 0 {
			_ = uiAdapter.DisplaySystemMessage("")
			_ = uiAdapter.DisplaySystemMessage(fmt.Sprintf("Discovered %d subagent(s):", result.TotalCount))
			for _, agent := range result.Subagents {
				_ = uiAdapter.DisplaySystemMessage(fmt.Sprintf("  - %s: %s (%s)",
					agent.Name, agent.Description, agent.SourceType))
			}
			_ = uiAdapter.DisplaySystemMessage("")
		}
	}

	// Get interrupt handler from context for graceful shutdown support
	handler := InterruptHandlerFromContext(ctx)

	// Main chat loop
	for {
		// Get the first press channel each iteration (resets after timeout)
		var firstPressCh <-chan struct{}
		if handler != nil {
			firstPressCh = handler.FirstPress()
		}

		// Get user input with context support (readline handles goroutine internally)
		var result inputResult
		done := make(chan struct{})
		go func() {
			defer close(done)
			// Defer a panic recovery to prevent goroutine from hanging
			defer func() {
				if r := recover(); r != nil {
					result = inputResult{"", false}
				}
			}()
			text, ok := uiAdapter.GetUserInput(ctx)
			result = inputResult{text, ok}
		}()

		// Wait for input OR signals (no timeout needed with readline context support)
	waitLoop:
		for {
			select {
			case <-ctx.Done():
				// Context cancelled (second Ctrl+C pressed or external cancellation)
				fmt.Printf("\n%s\n", cfg.GoodbyeMessage)
				return nil
			case <-firstPressCh:
				// First Ctrl+C pressed - show message and re-display prompt
				fmt.Printf("\nPress Ctrl+C again to exit\n")
				fmt.Print("Claude: ")
				// Set to nil to avoid receiving again on this channel
				firstPressCh = nil
				continue
			case <-done:
				// Input goroutine finished
				break waitLoop
			}
		}
		if !result.ok {
			// User closed input stream
			fmt.Printf("\n%s\n", cfg.GoodbyeMessage)
			return nil
		}

		// Check if user wants to exit
		if result.text == "exit" || result.text == "quit" || result.text == ":q" {
			fmt.Printf("%s\n", cfg.GoodbyeMessage)
			return nil
		}

		// Check for :mode command to toggle plan mode
		if strings.HasPrefix(result.text, ":mode") {
			parts := strings.Fields(result.text)
			var mode string
			if len(parts) > 1 {
				mode = parts[1]
			} else {
				mode = "toggle"
			}
			if err := chatService.HandleModeCommand(ctx, sessionID, mode); err != nil {
				_ = uiAdapter.DisplayError(err)
			} else {
				// Display current mode status
				convSvc := container.ConversationService()
				if isPlanMode, _ := convSvc.IsPlanMode(sessionID); isPlanMode {
					_ = uiAdapter.DisplaySystemMessage("Plan mode enabled: Tools will write plans to files instead of executing.")
				} else {
					_ = uiAdapter.DisplaySystemMessage("Plan mode disabled: Tools will execute normally.")
				}
			}
			// Continue to next iteration (don't send to AI)
			continue
		}

		// Check for :thinking command to toggle extended thinking mode
		if strings.HasPrefix(result.text, ":thinking") {
			parts := strings.Fields(result.text)
			var mode string
			if len(parts) > 1 {
				mode = parts[1]
			} else {
				mode = "toggle"
			}
			if err := chatService.HandleThinkingCommand(ctx, sessionID, mode); err != nil {
				_ = uiAdapter.DisplayError(err)
			} else {
				// Display current thinking mode status
				convSvc := container.ConversationService()
				if thinkingInfo, _ := convSvc.GetThinkingMode(sessionID); thinkingInfo.Enabled {
					_ = uiAdapter.DisplaySystemMessage(fmt.Sprintf("Extended thinking enabled: Budget %d tokens", thinkingInfo.BudgetTokens))
				} else {
					_ = uiAdapter.DisplaySystemMessage("Extended thinking disabled")
				}
			}
			// Continue to next iteration (don't send to AI)
			continue
		}

		// Send message and get response
		_, err = chatService.SendMessage(ctx, sessionID, result.text)
		if err != nil {
			// Check for context cancellation specifically
			if errors.Is(err, context.Canceled) {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nOperation cancelled. Type 'exit' to quit or continue.\n")
			} else {
				errMsg := fmt.Sprintf("Error processing message: %v", err)
				_ = uiAdapter.DisplayError(fmt.Errorf("%s", errMsg))
				fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", errMsg)
			}
		}
	}
}
