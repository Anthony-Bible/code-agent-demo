package cmd

import (
	"code-editing-agent/internal/infrastructure/config"
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// chatCmd represents the chat command
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

// inputResult holds the result from the async input goroutine
type inputResult struct {
	text string
	ok   bool
}

// runChat executes the chat command
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

	// Create a new session
	startResp, err := chatService.StartSession(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to start chat session: %w", err)
	}
	sessionID := startResp.SessionID

	// Get interrupt handler from context for graceful shutdown support
	handler := InterruptHandlerFromContext(ctx)

	// Main chat loop
	for {
		// Get the first press channel each iteration (resets after timeout)
		var firstPressCh <-chan struct{}
		if handler != nil {
			firstPressCh = handler.FirstPress()
		}
		// Start async input reader in a goroutine
		inputCh := make(chan inputResult, 1)
		go func() {
			text, ok := uiAdapter.GetUserInput(ctx)
			inputCh <- inputResult{text, ok}
		}()

		// Wait for input OR signals
	waitLoop:
		for {
			select {
			case <-ctx.Done():
				// Context cancelled (second Ctrl+C pressed or external cancellation)
				fmt.Printf("\n%s\n", cfg.GoodbyeMessage)
				return nil
			case <-firstPressCh:
				// First Ctrl+C pressed - show message and continue waiting for input
				fmt.Printf("\nPress Ctrl+C again to exit\n")
				// Set to nil to avoid receiving again on this channel
				firstPressCh = nil
				continue
			case result := <-inputCh:
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
				// Break out of waitLoop to start next iteration of main loop
				break waitLoop
			}
		}
	}
}
