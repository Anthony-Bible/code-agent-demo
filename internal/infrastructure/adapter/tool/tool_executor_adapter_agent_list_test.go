package tool_test

import (
"os"
"strings"
"testing"

"code-editing-agent/internal/domain/entity"
"code-editing-agent/internal/infrastructure/adapter/file"
"code-editing-agent/internal/infrastructure/adapter/subagent"
"code-editing-agent/internal/infrastructure/adapter/tool"
"path/filepath"
)

func TestTaskToolIncludesAvailableAgents(t *testing.T) {
// Create temp dir
tmpDir := t.TempDir()

// Create agents directory with a test agent
agentsDir := filepath.Join(tmpDir, "agents", "test-agent")
if err := os.MkdirAll(agentsDir, 0755); err != nil {
t.Fatalf("Failed to create agents dir: %v", err)
}

// Create AGENT.md file
agentContent := `---
name: test-agent
description: A test agent for verification
---
Test agent system prompt.`

agentPath := filepath.Join(agentsDir, "AGENT.md")
if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
t.Fatalf("Failed to write AGENT.md: %v", err)
}

// Create file manager and tool executor
fileManager := file.NewLocalFileManager(tmpDir)
toolExecutor := tool.NewExecutorAdapter(fileManager)

// Create and set subagent manager
subagentManager := subagent.NewLocalSubagentManagerWithDirs([]subagent.DirConfig{
{Path: filepath.Join(tmpDir, "agents"), SourceType: entity.SubagentSourceProject},
})
toolExecutor.SetSubagentManager(subagentManager)

// Get task tool
taskTool, found := toolExecutor.GetTool("task")
if !found {
t.Fatal("Task tool not found")
}

// Verify description includes available agents
if !strings.Contains(taskTool.Description, "Available agents") {
t.Error("Task tool description should include 'Available agents' section")
}

// Verify test agent is listed
if !strings.Contains(taskTool.Description, "test-agent") {
t.Error("Task tool description should include test-agent name")
}

if !strings.Contains(taskTool.Description, "A test agent for verification") {
t.Error("Task tool description should include test-agent description")
}
}
