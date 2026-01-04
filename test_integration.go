//go:build ignore
// +build ignore

package main

import (
	"code-editing-agent/internal/infrastructure/config"
	"context"
	"fmt"
	"log"
	"os"
)

func main() {
	fmt.Println("=== Subagent System Integration Test ===\n")

	// Create container with default config
	cfg := &config.Config{
		WorkingDir: ".",
		AIModel:    "claude-sonnet-4-5-20250929",
	}

	container, err := config.NewContainer(cfg)
	if err != nil {
		log.Fatalf("Failed to create container: %v", err)
	}

	ctx := context.Background()

	// Test 1: SubagentManager discovery
	fmt.Println("Test 1: Discovering agents...")
	subagentManager := container.SubagentManager()
	if subagentManager == nil {
		log.Fatal("SubagentManager is nil")
	}

	result, err := subagentManager.DiscoverAgents(ctx)
	if err != nil {
		log.Fatalf("Failed to discover agents: %v", err)
	}

	fmt.Printf("✓ Discovered %d agents:\n", len(result.Subagents))
	for _, agent := range result.Subagents {
		fmt.Printf("  - %s (%s): %s\n", agent.Name, agent.SourceType, agent.Description)
	}
	fmt.Println()

	// Test 2: Load agent metadata
	fmt.Println("Test 2: Loading code-reviewer agent metadata...")
	agent, err := subagentManager.LoadAgentMetadata(ctx, "code-reviewer")
	if err != nil {
		log.Fatalf("Failed to load agent: %v", err)
	}
	fmt.Printf("✓ Loaded agent: %s\n", agent.Name)
	fmt.Printf("  Model: %s\n", agent.Model)
	fmt.Printf("  Max Actions: %d\n", agent.MaxActions)
	fmt.Printf("  Allowed Tools: %v\n", agent.AllowedTools)
	fmt.Printf("  System Prompt Length: %d chars\n", len(agent.RawContent))
	fmt.Println()

	// Test 3: Task tool availability
	fmt.Println("Test 3: Checking task tool availability...")
	toolExecutor := container.ToolExecutor()
	tools, err := toolExecutor.ListTools()
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	var taskToolFound bool
	for _, tool := range tools {
		if tool.Name == "task" {
			taskToolFound = true
			fmt.Printf("✓ Task tool found: %s\n", tool.Description)
			break
		}
	}

	if !taskToolFound {
		log.Fatal("Task tool not found in available tools")
	}
	fmt.Println()

	// Test 4: SubagentUseCase availability
	fmt.Println("Test 4: Checking SubagentUseCase...")
	subagentUseCase := container.SubagentUseCase()
	if subagentUseCase == nil {
		log.Fatal("SubagentUseCase is nil")
	}
	fmt.Println("✓ SubagentUseCase is wired correctly")
	fmt.Println()

	// Success
	fmt.Println("=== All Integration Tests Passed ✓ ===")
	os.Exit(0)
}
