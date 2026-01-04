// Package subagent provides an implementation of the domain SubagentManager port.
// It follows hexagonal architecture principles by providing infrastructure-level
// subagent discovery and management operations for the AI coding agent.
//
// Subagents are discovered from multiple directories in priority order:
//   - ./agents (project root, highest priority)
//   - ./.claude/agents (project .claude directory)
//   - ~/.claude/agents (user global, lowest priority)
//
// When the same subagent name exists in multiple directories, the highest priority
// directory wins. Each subagent is represented by a directory containing an AGENT.md
// file with YAML frontmatter defining the subagent's metadata.
//
// Example usage:
//
//	sm := subagent.NewLocalSubagentManager()
//	result, err := sm.DiscoverAgents(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Found %d subagents from %d directories\n", result.TotalCount, len(result.AgentsDirs))
package subagent

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DirConfig represents a directory to search for subagents with its source type.
type DirConfig struct {
	Path       string
	SourceType entity.SubagentSourceType
}

// LocalSubagentManager implements the SubagentManager port for managing local file system subagents.
// It discovers subagents from multiple directories, loads their metadata, and manages
// their registration state.
type LocalSubagentManager struct {
	mu           sync.RWMutex
	agentsDirs   []DirConfig                 // Directories to search for subagents in priority order
	agents       map[string]*entity.Subagent // Discovered subagents by name
	programmatic map[string]*entity.Subagent // Programmatically registered subagents by name
}

// NewLocalSubagentManager creates a new LocalSubagentManager instance.
// Subagents are discovered from multiple directories in priority order:
// ./agents, ./.claude/agents, and ~/.claude/agents.
func NewLocalSubagentManager() port.SubagentManager {
	return &LocalSubagentManager{
		agentsDirs:   []DirConfig{},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}
}

// NewLocalSubagentManagerWithDirs creates a LocalSubagentManager with custom directories.
// This is primarily for testing to avoid discovering subagents from user's home directory.
func NewLocalSubagentManagerWithDirs(dirs []DirConfig) port.SubagentManager {
	return &LocalSubagentManager{
		agentsDirs:   dirs,
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}
}

// DiscoverAgents scans all configured subagent directories for available subagents.
// Directories are searched in priority order, and when a subagent name exists in
// multiple directories, the highest priority version is used.
func (sm *LocalSubagentManager) DiscoverAgents(_ context.Context) (*port.SubagentDiscoveryResult, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	dirsToSearch := sm.getDirsToSearch()

	var discoveredAgents []port.SubagentInfo
	var agentsDirs []string
	seenAgents := make(map[string]bool)

	for _, dirConfig := range dirsToSearch {
		agentsDirs = append(agentsDirs, dirConfig.Path)
		agents := sm.discoverFromDirectory(dirConfig, seenAgents)
		discoveredAgents = append(discoveredAgents, agents...)
	}

	return &port.SubagentDiscoveryResult{
		Subagents:  discoveredAgents,
		AgentsDirs: agentsDirs,
		TotalCount: len(discoveredAgents),
	}, nil
}

// getDirsToSearch returns the list of directories to search for subagents.
func (sm *LocalSubagentManager) getDirsToSearch() []DirConfig {
	return sm.agentsDirs
}

// discoverFromDirectory scans a single directory for AGENT.md files.
// The seenAgents map tracks already-discovered agent names for deduplication.
// Returns agent info for each valid agent found that has not already been seen.
func (sm *LocalSubagentManager) discoverFromDirectory(
	dirConfig DirConfig,
	seenAgents map[string]bool,
) []port.SubagentInfo {
	var agents []port.SubagentInfo

	info, err := os.Stat(dirConfig.Path)
	if err != nil || !info.IsDir() {
		return agents
	}

	_ = filepath.Walk(dirConfig.Path, func(path string, info os.FileInfo, _ error) error {
		if path == dirConfig.Path || info == nil {
			return nil
		}
		if info.Name() == "AGENT.md" && !info.IsDir() {
			if agentInfo := sm.processAgentFileWithSource(path, dirConfig.SourceType, seenAgents); agentInfo != nil {
				agents = append(agents, *agentInfo)
			}
		}
		return nil
	})

	return agents
}

// processAgentFileWithSource processes an AGENT.md file with source type and deduplication.
func (sm *LocalSubagentManager) processAgentFileWithSource(
	path string,
	sourceType entity.SubagentSourceType,
	seenAgents map[string]bool,
) *port.SubagentInfo {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	agent, parseErr := entity.ParseSubagentMetadataFromYAML(string(content))
	if parseErr != nil {
		return nil
	}

	if err := agent.Validate(); err != nil {
		return nil
	}

	// Skip if already seen (higher priority directory already discovered this agent)
	if seenAgents[agent.Name] {
		return nil
	}
	seenAgents[agent.Name] = true

	dirPath := filepath.Dir(path)
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		absPath = dirPath
	}
	agent.ScriptPath = absPath
	agent.OriginalPath = dirPath
	agent.SourceType = sourceType

	sm.agents[agent.Name] = agent

	info := sm.agentToInfo(agent)
	return &info
}

// agentToInfo converts an entity.Subagent to a port.SubagentInfo.
// This is a pure conversion function that maps entity fields to the port interface.
func (sm *LocalSubagentManager) agentToInfo(agent *entity.Subagent) port.SubagentInfo {
	return port.SubagentInfo{
		Name:          agent.Name,
		Description:   agent.Description,
		AllowedTools:  agent.AllowedTools,
		Model:         entity.SubagentModel(agent.Model),
		SourceType:    agent.SourceType,
		DirectoryPath: agent.OriginalPath,
	}
}

// findAgentPath searches all configured directories for an agent's AGENT.md file.
// Returns the path to the first matching file found, or empty string if not found.
func (sm *LocalSubagentManager) findAgentPath(agentName string) string {
	for _, dirConfig := range sm.getDirsToSearch() {
		agentPath := filepath.Join(dirConfig.Path, agentName, "AGENT.md")
		if _, err := os.Stat(agentPath); err == nil {
			return agentPath
		}
	}
	return ""
}

// readAndParseAgentFile reads an AGENT.md file and parses it into a full Subagent entity.
// Returns ErrAgentFileNotFound if the file doesn't exist, or a wrapped error for parse failures.
func (sm *LocalSubagentManager) readAndParseAgentFile(agentPath string) (*entity.Subagent, error) {
	content, err := os.ReadFile(agentPath)
	if os.IsNotExist(err) {
		return nil, ErrAgentFileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to read AGENT.md: %w", err)
	}

	fullAgent, err := entity.ParseSubagentFromYAML(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent: %w", err)
	}

	return fullAgent, nil
}

// LoadAgentMetadata loads the metadata for a specific subagent from its AGENT.md file.
// The agentName should match the subagent directory name.
// Returns the subagent entity with all parsed metadata.
// If the agent was discovered with ParseSubagentMetadataFromYAML (progressive disclosure),
// this function will load the full content on-demand.
func (sm *LocalSubagentManager) LoadAgentMetadata(_ context.Context, agentName string) (*entity.Subagent, error) {
	// Validate agentName to prevent path traversal attacks
	// Agent names must match the agentskills.io spec: lowercase alphanumeric and hyphens only
	if err := validateAgentName(agentName); err != nil {
		return nil, err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if agent is already loaded
	agent, exists := sm.agents[agentName]
	if exists && agent.RawContent != "" {
		// Agent exists and has full content loaded
		return agent, nil
	}

	// If agent exists but has no content, use its OriginalPath
	// Otherwise, search through directories
	var agentPath string
	if exists && agent.OriginalPath != "" {
		agentPath = filepath.Join(agent.OriginalPath, "AGENT.md")
	} else {
		agentPath = sm.findAgentPath(agentName)
	}

	if agentPath == "" {
		return nil, ErrAgentFileNotFound
	}

	fullAgent, err := sm.readAndParseAgentFile(agentPath)
	if err != nil {
		return nil, err
	}

	if exists {
		// Update the existing agent with full content while preserving path info
		agent.RawContent = fullAgent.RawContent
		agent.RawFrontmatter = fullAgent.RawFrontmatter
		return agent, nil
	}

	// Return the newly parsed agent
	return fullAgent, nil
}

// RegisterAgent registers a subagent, making it available for use.
// Registered subagents can be invoked by the AI through the tool system.
// Returns an error if the subagent is invalid or already registered.
func (sm *LocalSubagentManager) RegisterAgent(_ context.Context, agent *entity.Subagent) error {
	if agent == nil {
		return ErrInvalidAgent
	}

	if err := agent.Validate(); err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.programmatic[agent.Name]; exists {
		return ErrAgentAlreadyRegistered
	}

	sm.programmatic[agent.Name] = agent
	return nil
}

// UnregisterAgent unregisters a subagent by name, removing it from available subagents.
// Returns an error if the subagent is not found or cannot be unregistered.
func (sm *LocalSubagentManager) UnregisterAgent(_ context.Context, agentName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.programmatic[agentName]; !exists {
		return ErrAgentNotFound
	}

	delete(sm.programmatic, agentName)
	return nil
}

// GetAgentByName returns information about a specific subagent by name.
// Returns nil if the subagent is not found.
func (sm *LocalSubagentManager) GetAgentByName(_ context.Context, agentName string) (*port.SubagentInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Check programmatic agents first
	if agent, ok := sm.programmatic[agentName]; ok {
		info := sm.agentToInfo(agent)
		return &info, nil
	}

	// Check discovered agents
	agent, ok := sm.agents[agentName]
	if !ok {
		return nil, ErrAgentNotFound
	}

	info := sm.agentToInfo(agent)
	return &info, nil
}

// ListAgents returns a list of all registered subagents.
// Registered subagents are those that have been registered and are available for use.
func (sm *LocalSubagentManager) ListAgents(_ context.Context) ([]port.SubagentInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var agentList []port.SubagentInfo

	// Add discovered agents
	for _, agent := range sm.agents {
		agentList = append(agentList, sm.agentToInfo(agent))
	}

	// Add programmatic agents
	for _, agent := range sm.programmatic {
		agentList = append(agentList, sm.agentToInfo(agent))
	}

	return agentList, nil
}
