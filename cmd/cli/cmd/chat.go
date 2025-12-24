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

	// Main chat loop
	for {
		select {
		case <-ctx.Done():
			// Context cancelled (Ctrl+C pressed)
			fmt.Printf("\n%s\n", cfg.GoodbyeMessage)
			return nil
		default:
			// Continue with chat
		}

		// Get user input
		userInput, ok := uiAdapter.GetUserInput(ctx)
		if !ok {
			// User closed input stream
			fmt.Printf("\n%s\n", cfg.GoodbyeMessage)
			return nil
		}

		// Check if user wants to exit
		if userInput == "exit" || userInput == "quit" || userInput == ":q" {
			fmt.Printf("%s\n", cfg.GoodbyeMessage)
			return nil
		}

		// Send message and get response
		_, err = chatService.SendMessage(ctx, sessionID, userInput)
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
