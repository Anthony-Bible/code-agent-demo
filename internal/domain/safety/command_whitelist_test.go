package safety

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestCommandWhitelist_IsAllowed(t *testing.T) {
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: "list directory"},
		{Pattern: regexp.MustCompile(`^cat(\s|$)`), Description: "display file"},
		{Pattern: regexp.MustCompile(`^git\s+status(\s|$)`), Description: "git status"},
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		wantDesc    string
	}{
		{
			name:        "simple ls allowed",
			command:     "ls",
			wantAllowed: true,
			wantDesc:    "list directory",
		},
		{
			name:        "ls with args allowed",
			command:     "ls -la",
			wantAllowed: true,
			wantDesc:    "list directory",
		},
		{
			name:        "cat with file allowed",
			command:     "cat file.txt",
			wantAllowed: true,
			wantDesc:    "display file",
		},
		{
			name:        "git status allowed",
			command:     "git status",
			wantAllowed: true,
			wantDesc:    "git status",
		},
		{
			name:        "git status with args allowed",
			command:     "git status -s",
			wantAllowed: true,
			wantDesc:    "git status",
		},
		{
			name:        "rm not allowed",
			command:     "rm file.txt",
			wantAllowed: false,
			wantDesc:    "",
		},
		{
			name:        "sudo not allowed",
			command:     "sudo ls",
			wantAllowed: false,
			wantDesc:    "",
		},
		{
			name:        "curl not allowed",
			command:     "curl http://example.com",
			wantAllowed: false,
			wantDesc:    "",
		},
		{
			name:        "empty command not allowed",
			command:     "",
			wantAllowed: false,
			wantDesc:    "",
		},
		{
			name:        "tab_between_command_and_flag",
			command:     "ls\t-la",
			wantAllowed: true,
			wantDesc:    "list directory",
		},
		{
			name:        "multiple_spaces_between_args",
			command:     "ls  -la",
			wantAllowed: true,
			wantDesc:    "list directory",
		},
		{
			name:        "mixed_whitespace_git_status",
			command:     "git\t\tstatus",
			wantAllowed: true,
			wantDesc:    "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, desc := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) allowed = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
			if desc != tt.wantDesc {
				t.Errorf("IsAllowed(%q) desc = %q, want %q", tt.command, desc, tt.wantDesc)
			}
		})
	}
}

func TestCommandWhitelist_IsAllowedWithPipes_ReDoSProtection(t *testing.T) {
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: "list directory"},
	}
	whitelist := NewCommandWhitelist(patterns)

	// Command exceeding MaxCommandLength should be rejected
	longCommand := "ls " + strings.Repeat("a", MaxCommandLength+1)
	allowed, desc := whitelist.IsAllowedWithPipes(longCommand)
	if allowed {
		t.Errorf("expected command exceeding MaxCommandLength to be rejected")
	}
	if desc != "" {
		t.Errorf("expected empty description for rejected command, got %q", desc)
	}
}

func TestCommandWhitelist_IsAllowedWithPipes_MaxTotalSegments(t *testing.T) {
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: "list directory"},
		{Pattern: regexp.MustCompile(`^echo(\s|$)`), Description: "echo"},
	}
	whitelist := NewCommandWhitelist(patterns)

	// Build a command with more segments than MaxTotalSegments
	// Using pipes to create many segments: "ls | ls | ls | ..."
	segments := make([]string, MaxTotalSegments+10)
	for i := range segments {
		segments[i] = "ls"
	}
	manySegmentsCommand := strings.Join(segments, " | ")

	allowed, desc := whitelist.IsAllowedWithPipes(manySegmentsCommand)
	if allowed {
		t.Errorf("expected command exceeding MaxTotalSegments to be rejected")
	}
	if desc != "" {
		t.Errorf("expected empty description for rejected command, got %q", desc)
	}

	// Verify that a command just under the limit is still allowed
	segmentsUnderLimit := make([]string, MaxTotalSegments-10)
	for i := range segmentsUnderLimit {
		segmentsUnderLimit[i] = "ls"
	}
	underLimitCommand := strings.Join(segmentsUnderLimit, " | ")

	allowed, _ = whitelist.IsAllowedWithPipes(underLimitCommand)
	if !allowed {
		t.Errorf("expected command under MaxTotalSegments to be allowed")
	}
}

func TestCommandWhitelist_IsAllowedWithPipes(t *testing.T) {
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: "list directory"},
		{Pattern: regexp.MustCompile(`^grep(\s|$)`), Description: "search"},
		{Pattern: regexp.MustCompile(`^wc(\s|$)`), Description: "word count"},
		{Pattern: regexp.MustCompile(`^sort(\s|$)`), Description: "sort"},
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
	}{
		{
			name:        "single command allowed",
			command:     "ls -la",
			wantAllowed: true,
		},
		{
			name:        "piped commands all allowed",
			command:     "ls -la | grep foo",
			wantAllowed: true,
		},
		{
			name:        "three piped commands allowed",
			command:     "ls -la | grep foo | wc -l",
			wantAllowed: true,
		},
		{
			name:        "and chain allowed",
			command:     "ls -la && grep foo file.txt",
			wantAllowed: true,
		},
		{
			name:        "or chain allowed",
			command:     "ls -la || grep foo file.txt",
			wantAllowed: true,
		},
		{
			name:        "sequential chain allowed",
			command:     "ls -la; grep foo file.txt",
			wantAllowed: true,
		},
		{
			name:        "piped with one not allowed",
			command:     "ls -la | rm file.txt",
			wantAllowed: false,
		},
		{
			name:        "first command not allowed",
			command:     "curl http://example.com | grep foo",
			wantAllowed: false,
		},
		{
			name:        "middle command not allowed",
			command:     "ls | rm -rf / | wc",
			wantAllowed: false,
		},
		{
			name:        "empty command",
			command:     "",
			wantAllowed: false,
		},
		{
			name:        "only separators",
			command:     "| && ||",
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowedWithPipes(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowedWithPipes(%q) = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
		})
	}
}

func TestDefaultWhitelistPatterns(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	// Test that expected commands are allowed
	allowedCommands := []string{
		"ls",
		"ls -la",
		"cat file.txt",
		"head -n 10 file.txt",
		"tail -f log.txt",
		"grep pattern file.txt",
		"git status",
		"git log --oneline",
		"git diff HEAD~1",
		"go version",
		"go env",
		"pwd",
		"whoami",
		"ps aux",
		"df -h",
		"docker ps",
		"kubectl get pods",
	}

	for _, cmd := range allowedCommands {
		t.Run("allowed_"+cmd, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(cmd)
			if !allowed {
				t.Errorf("expected %q to be allowed", cmd)
			}
		})
	}

	// Test that dangerous commands are NOT allowed
	blockedCommands := []string{
		"rm file.txt",
		"rm -rf /",
		"sudo ls",
		"chmod 777 file",
		"chown root file",
		"dd if=/dev/zero of=/dev/sda",
		"curl http://example.com",
		"wget http://example.com",
		"git push",
		"git commit",
		"git add .",
		"go build",
		"go run main.go",
		"npm install",
		"make",
		"mkdir test",
		"touch file.txt",
		"mv old new",
		"cp src dst",
	}

	for _, cmd := range blockedCommands {
		t.Run("blocked_"+cmd, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(cmd)
			if allowed {
				t.Errorf("expected %q to be blocked", cmd)
			}
		})
	}
}

func TestParseWhitelistPatternsJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		wantCount   int
		wantErr     bool
		testCommand string
		wantMatch   bool
		wantDesc    string
	}{
		{
			name:        "valid pattern with description",
			jsonStr:     `[{"pattern": "^mycommand\\s", "description": "my command"}]`,
			wantCount:   1,
			wantErr:     false,
			testCommand: "mycommand arg",
			wantMatch:   true,
			wantDesc:    "my command",
		},
		{
			name:        "multiple patterns",
			jsonStr:     `[{"pattern": "^mycommand(\\s|$)"}, {"pattern": "^othercommand(\\s|$)"}]`,
			wantCount:   2,
			wantErr:     false,
			testCommand: "mycommand arg",
			wantMatch:   true,
			wantDesc:    "custom pattern: ^mycommand(\\s|$)",
		},
		{
			name:        "pattern with exclude",
			jsonStr:     `[{"pattern": "^find(\\s|$)", "exclude_pattern": "-exec\\s", "description": "find without exec"}]`,
			wantCount:   1,
			wantErr:     false,
			testCommand: "find . -name foo",
			wantMatch:   true,
			wantDesc:    "find without exec",
		},
		{
			name:        "empty string returns nil",
			jsonStr:     "",
			wantCount:   0,
			wantErr:     false,
			testCommand: "anything",
			wantMatch:   false,
		},
		{
			name:        "empty array",
			jsonStr:     "[]",
			wantCount:   0,
			wantErr:     false,
			testCommand: "anything",
			wantMatch:   false,
		},
		{
			name:        "invalid JSON syntax",
			jsonStr:     `[{"pattern": "^test"`,
			wantCount:   0,
			wantErr:     true,
			testCommand: "test",
			wantMatch:   false,
		},
		{
			name:        "invalid pattern regex",
			jsonStr:     `[{"pattern": "[invalid"}]`,
			wantCount:   0,
			wantErr:     true,
			testCommand: "anything",
			wantMatch:   false,
		},
		{
			name:        "invalid exclude regex",
			jsonStr:     `[{"pattern": "^valid", "exclude_pattern": "[invalid"}]`,
			wantCount:   0,
			wantErr:     true,
			testCommand: "valid",
			wantMatch:   false,
		},
		{
			name:        "missing required pattern field",
			jsonStr:     `[{"description": "no pattern"}]`,
			wantCount:   0,
			wantErr:     true,
			testCommand: "anything",
			wantMatch:   false,
		},
		{
			name:        "mixed valid and invalid patterns fails completely",
			jsonStr:     `[{"pattern": "^valid(\\s|$)"}, {"pattern": "[invalid"}]`,
			wantCount:   0,
			wantErr:     true,
			testCommand: "valid",
			wantMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns, err := ParseWhitelistPatternsJSON(tt.jsonStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseWhitelistPatternsJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(patterns) != tt.wantCount {
				t.Errorf("ParseWhitelistPatternsJSON() count = %d, want %d", len(patterns), tt.wantCount)
			}

			if len(patterns) > 0 {
				whitelist := NewCommandWhitelist(patterns)
				allowed, desc := whitelist.IsAllowed(tt.testCommand)
				if allowed != tt.wantMatch {
					t.Errorf("test command %q match = %v, want %v", tt.testCommand, allowed, tt.wantMatch)
				}
				if tt.wantDesc != "" && desc != tt.wantDesc {
					t.Errorf("description = %q, want %q", desc, tt.wantDesc)
				}
			}
		})
	}
}

func TestParseWhitelistPatternsJSON_ExcludePattern(t *testing.T) {
	// Test that exclude pattern correctly blocks matching commands
	jsonStr := `[{"pattern": "^find(\\s|$)", "exclude_pattern": "(-exec\\s|-delete)", "description": "safe find"}]`
	patterns, err := ParseWhitelistPatternsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}

	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		command     string
		wantAllowed bool
	}{
		{"find . -name foo", true},
		{"find /tmp -type f", true},
		{"find . -exec rm {} \\;", false},
		{"find . -delete", false},
	}

	for _, tt := range tests {
		allowed, _ := whitelist.IsAllowed(tt.command)
		if allowed != tt.wantAllowed {
			t.Errorf("IsAllowed(%q) = %v, want %v", tt.command, allowed, tt.wantAllowed)
		}
	}
}

func TestParseWhitelistPatternsJSON_ReDoSProtection(t *testing.T) {
	// Pattern exceeding MaxCommandLength should be rejected
	longPattern := strings.Repeat("a", MaxCommandLength+1)
	jsonStr := `[{"pattern": "` + longPattern + `"}]`
	patterns, err := ParseWhitelistPatternsJSON(jsonStr)
	if err == nil {
		t.Error("expected error for pattern exceeding MaxCommandLength")
	}
	if len(patterns) != 0 {
		t.Errorf("expected no patterns, got %d", len(patterns))
	}

	// Exclude pattern exceeding MaxCommandLength should also be rejected
	longExclude := strings.Repeat("b", MaxCommandLength+1)
	jsonStr2 := `[{"pattern": "^test", "exclude_pattern": "` + longExclude + `"}]`
	patterns2, err2 := ParseWhitelistPatternsJSON(jsonStr2)
	if err2 == nil {
		t.Error("expected error for exclude pattern exceeding MaxCommandLength")
	}
	if len(patterns2) != 0 {
		t.Errorf("expected no patterns, got %d", len(patterns2))
	}
}

func TestValidateMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantMode CommandValidationMode
		wantErr  bool
	}{
		{
			name:     "blacklist explicit",
			mode:     "blacklist",
			wantMode: ModeBlacklist,
			wantErr:  false,
		},
		{
			name:     "whitelist",
			mode:     "whitelist",
			wantMode: ModeWhitelist,
			wantErr:  false,
		},
		{
			name:     "empty defaults to blacklist",
			mode:     "",
			wantMode: ModeBlacklist,
			wantErr:  false,
		},
		{
			name:     "case insensitive blacklist",
			mode:     "BLACKLIST",
			wantMode: ModeBlacklist,
			wantErr:  false,
		},
		{
			name:     "case insensitive whitelist",
			mode:     "WHITELIST",
			wantMode: ModeWhitelist,
			wantErr:  false,
		},
		{
			name:     "invalid mode",
			mode:     "invalid",
			wantMode: ModeBlacklist,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := ValidateMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMode(%q) error = %v, wantErr %v", tt.mode, err, tt.wantErr)
			}
			if mode != tt.wantMode {
				t.Errorf("ValidateMode(%q) = %v, want %v", tt.mode, mode, tt.wantMode)
			}
		})
	}
}

func TestDefaultWhitelistPatterns_Categories(t *testing.T) {
	// Verify each category function returns non-empty patterns
	categories := []struct {
		name    string
		fn      func() []WhitelistPattern
		minSize int
	}{
		{"fileReadPatterns", fileReadPatterns, 10},
		{"searchPatterns", searchPatterns, 10},
		{"textProcessingPatterns", textProcessingPatterns, 10},
		{"gitReadPatterns", gitReadPatterns, 15},
		{"devToolPatterns", devToolPatterns, 30},
		{"systemInfoPatterns", systemInfoPatterns, 15},
		{"utilityPatterns", utilityPatterns, 10},
		{"containerPatterns", containerPatterns, 15},
	}

	for _, cat := range categories {
		t.Run(cat.name, func(t *testing.T) {
			patterns := cat.fn()
			if len(patterns) < cat.minSize {
				t.Errorf("%s() returned %d patterns, expected at least %d", cat.name, len(patterns), cat.minSize)
			}

			// Verify all patterns compile correctly (they're compiled inline)
			for _, p := range patterns {
				if p.Pattern == nil {
					t.Errorf("%s() contains nil pattern", cat.name)
				}
				if p.Description == "" {
					t.Errorf("%s() contains empty description", cat.name)
				}
			}
		})
	}
}

func TestDefaultWhitelistPatterns_TotalCount(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	// Ensure we have a reasonable number of default patterns
	if len(patterns) < 100 {
		t.Errorf("DefaultWhitelistPatterns() returned %d patterns, expected at least 100", len(patterns))
	}
}

func TestFindCommand_WhitelistWithExcludePattern(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		description string
	}{
		// Safe find commands that should be allowed
		{
			name:        "simple find",
			command:     "find .",
			wantAllowed: true,
			description: "basic find should be allowed",
		},
		{
			name:        "find with name flag",
			command:     "find . -name \"*.go\"",
			wantAllowed: true,
			description: "find with -name flag should be allowed",
		},
		{
			name:        "find with type flag",
			command:     "find /tmp -type f",
			wantAllowed: true,
			description: "find with -type flag should be allowed",
		},
		{
			name:        "find with multiple flags",
			command:     "find . -name \"*.txt\" -type f -mtime -7",
			wantAllowed: true,
			description: "find with multiple read-only flags should be allowed",
		},
		{
			name:        "find with executable flag",
			command:     "find . -executable",
			wantAllowed: true,
			description: "find with -executable flag should be allowed (different from -exec)",
		},
		{
			name:        "find with print flag",
			command:     "find . -print",
			wantAllowed: true,
			description: "find with -print flag should be allowed",
		},
		{
			name:        "find with print0 flag",
			command:     "find . -print0",
			wantAllowed: true,
			description: "find with -print0 flag should be allowed",
		},
		{
			name:        "find with maxdepth",
			command:     "find . -maxdepth 2 -name \"*.go\"",
			wantAllowed: true,
			description: "find with -maxdepth should be allowed",
		},

		// Dangerous find commands that should be blocked
		{
			name:        "find with exec",
			command:     "find . -exec rm {} \\;",
			wantAllowed: false,
			description: "find with -exec should be blocked",
		},
		{
			name:        "find with exec and plus",
			command:     "find . -exec rm {} +",
			wantAllowed: false,
			description: "find with -exec and + should be blocked",
		},
		{
			name:        "find with execdir",
			command:     "find . -execdir sh -c 'rm {}' \\;",
			wantAllowed: false,
			description: "find with -execdir should be blocked",
		},
		{
			name:        "find with delete",
			command:     "find . -delete",
			wantAllowed: false,
			description: "find with -delete should be blocked",
		},
		{
			name:        "find with delete at end",
			command:     "find . -name \"*.tmp\" -delete",
			wantAllowed: false,
			description: "find with -delete at end should be blocked",
		},
		{
			name:        "find with ok",
			command:     "find . -ok rm {} \\;",
			wantAllowed: false,
			description: "find with -ok should be blocked",
		},
		{
			name:        "find with okdir",
			command:     "find . -okdir rm {} \\;",
			wantAllowed: false,
			description: "find with -okdir should be blocked",
		},

		// Case variations (should be blocked)
		{
			name:        "find with uppercase EXEC",
			command:     "find . -EXEC rm {} \\;",
			wantAllowed: false,
			description: "find with -EXEC (uppercase) should be blocked",
		},
		{
			name:        "find with mixed case Exec",
			command:     "find . -Exec rm {} \\;",
			wantAllowed: false,
			description: "find with -Exec (mixed case) should be blocked",
		},
		{
			name:        "find with uppercase DELETE",
			command:     "find . -DELETE",
			wantAllowed: false,
			description: "find with -DELETE (uppercase) should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) = %v, want %v (%s)", tt.command, allowed, tt.wantAllowed, tt.description)
			}
		})
	}
}

func TestExcludePattern_Basic(t *testing.T) {
	// Test that ExcludePattern works correctly in isolation
	patterns := []WhitelistPattern{
		{
			Pattern:        regexp.MustCompile(`^cmd(\s|$)`),
			Description:    "test command",
			ExcludePattern: regexp.MustCompile(`--dangerous`),
		},
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
	}{
		{
			name:        "basic command allowed",
			command:     "cmd arg1",
			wantAllowed: true,
		},
		{
			name:        "command with excluded flag blocked",
			command:     "cmd --dangerous",
			wantAllowed: false,
		},
		{
			name:        "command with excluded flag in middle blocked",
			command:     "cmd --dangerous arg2",
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
		})
	}
}

func TestGitBranchTag_DeleteBlocked(t *testing.T) {
	// Test that git branch and tag delete operations are blocked
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		description string
	}{
		// git branch read-only operations (allowed)
		{
			name:        "git branch list",
			command:     "git branch",
			wantAllowed: true,
			description: "simple git branch should be allowed",
		},
		{
			name:        "git branch list all",
			command:     "git branch -a",
			wantAllowed: true,
			description: "git branch -a should be allowed",
		},
		{
			name:        "git branch list verbose",
			command:     "git branch -v",
			wantAllowed: true,
			description: "git branch -v should be allowed",
		},
		{
			name:        "git branch list remote",
			command:     "git branch -r",
			wantAllowed: true,
			description: "git branch -r should be allowed",
		},

		// git branch delete operations (blocked)
		{
			name:        "git branch delete lowercase",
			command:     "git branch -d feature-branch",
			wantAllowed: false,
			description: "git branch -d should be blocked",
		},
		{
			name:        "git branch delete uppercase",
			command:     "git branch -D feature-branch",
			wantAllowed: false,
			description: "git branch -D should be blocked",
		},
		{
			name:        "git branch delete long form",
			command:     "git branch --delete feature-branch",
			wantAllowed: false,
			description: "git branch --delete should be blocked",
		},

		// git tag read-only operations (allowed)
		{
			name:        "git tag list",
			command:     "git tag",
			wantAllowed: true,
			description: "simple git tag should be allowed",
		},
		{
			name:        "git tag list with pattern",
			command:     "git tag -l v1.*",
			wantAllowed: true,
			description: "git tag -l should be allowed",
		},

		// git tag delete operations (blocked)
		{
			name:        "git tag delete",
			command:     "git tag -d v1.0.0",
			wantAllowed: false,
			description: "git tag -d should be blocked",
		},
		{
			name:        "git tag delete long form",
			command:     "git tag --delete v1.0.0",
			wantAllowed: false,
			description: "git tag --delete should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) = %v, want %v (%s)", tt.command, allowed, tt.wantAllowed, tt.description)
			}
		})
	}
}

func TestIsAllowedWithPipes_EmptyDescription(t *testing.T) {
	// Test that patterns with empty descriptions still work correctly
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: ""},     // Empty description
		{Pattern: regexp.MustCompile(`^grep(\s|$)`), Description: ""},   // Empty description
		{Pattern: regexp.MustCompile(`^cat(\s|$)`), Description: "cat"}, // Non-empty description
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
	}{
		{
			name:        "single command with empty description",
			command:     "ls -la",
			wantAllowed: true,
		},
		{
			name:        "piped commands all with empty descriptions",
			command:     "ls | grep foo",
			wantAllowed: true,
		},
		{
			name:        "piped commands mixed empty and non-empty descriptions",
			command:     "ls | cat file.txt",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowedWithPipes(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowedWithPipes(%q) = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
		})
	}
}

func TestSplitCommandSegmentsQuoteAware(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
		wantErr bool
	}{
		{
			name:    "simple pipe",
			command: "ls | grep foo",
			want:    []string{"ls", "grep foo"},
			wantErr: false,
		},
		{
			name:    "pipe inside double quotes",
			command: `echo "ls | rm -rf /"`,
			want:    []string{`echo "ls | rm -rf /"`},
			wantErr: false,
		},
		{
			name:    "semicolon inside single quotes",
			command: `echo 'file;rm x'`,
			want:    []string{`echo 'file;rm x'`},
			wantErr: false,
		},
		{
			name:    "escape sequences in double quotes",
			command: `echo "hello\"world" | cat`,
			want:    []string{`echo "hello\"world"`, "cat"},
			wantErr: false,
		},
		{
			name:    "unbalanced double quotes",
			command: `echo "hello`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "unbalanced single quotes",
			command: `echo 'hello`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "bypass attempt with pipe inside quotes then real pipe",
			command: `echo "ls | rm -rf /" | bash`,
			want:    []string{`echo "ls | rm -rf /"`, "bash"},
			wantErr: false,
		},
		{
			name:    "mixed quotes - single inside double",
			command: `echo "hello 'world'" | cat`,
			want:    []string{`echo "hello 'world'"`, "cat"},
			wantErr: false,
		},
		{
			name:    "mixed quotes - double inside single",
			command: `echo 'hello "world"' | cat`,
			want:    []string{`echo 'hello "world"'`, "cat"},
			wantErr: false,
		},
		{
			name:    "apostrophe inside double quotes",
			command: `echo "it's fine" | grep ok`,
			want:    []string{`echo "it's fine"`, "grep ok"},
			wantErr: false,
		},
		{
			name:    "double inside single with &&",
			command: `echo 'say "hi"' && cat file`,
			want:    []string{`echo 'say "hi"'`, "cat file"},
			wantErr: false,
		},
		{
			name:    "complex nested both ways",
			command: `cmd "a 'b' c" | cmd2 'd "e" f'`,
			want:    []string{`cmd "a 'b' c"`, `cmd2 'd "e" f'`},
			wantErr: false,
		},
		{
			name:    "backtick command substitution",
			command: "echo `ls | wc`",
			want:    []string{"echo `ls | wc`"},
			wantErr: false,
		},
		{
			name:    "unbalanced backtick",
			command: "echo `ls",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "trailing escape",
			command: `echo test\`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "consecutive operators - empty segment filtered",
			command: "ls || | grep foo",
			want:    []string{"ls", "grep foo"},
			wantErr: false,
		},
		{
			name:    "dollar-paren command substitution preserved",
			command: `echo $(ls | wc -l) | cat`,
			want:    []string{`echo $(ls | wc -l)`, "cat"},
			wantErr: false,
		},
		{
			name:    "nested dollar-paren command substitution",
			command: `echo $(echo $(pwd)) | cat`,
			want:    []string{`echo $(echo $(pwd))`, "cat"},
			wantErr: false,
		},
		{
			name:    "unbalanced dollar-paren open",
			command: `echo $(ls`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "dollar-paren in single quotes is literal",
			command: `echo '$(rm -rf /)' | cat`,
			want:    []string{`echo '$(rm -rf /)'`, "cat"},
			wantErr: false,
		},
		{
			name:    "dollar-paren in double quotes",
			command: `echo "$(ls | wc -l)" | cat`,
			want:    []string{`echo "$(ls | wc -l)"`, "cat"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommandSegmentsQuoteAware(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitCommandSegmentsQuoteAware(%q) error = %v, wantErr %v", tt.command, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("splitCommandSegmentsQuoteAware(%q) = %v, want %v", tt.command, got, tt.want)
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf(
							"splitCommandSegmentsQuoteAware(%q)[%d] = %q, want %q",
							tt.command,
							i,
							got[i],
							tt.want[i],
						)
					}
				}
			}
		})
	}
}

func TestValidateRegexSafety(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "safe pattern",
			pattern: `^ls(\s|$)`,
			wantErr: false,
		},
		{
			name:    "safe pattern with alternation",
			pattern: `^(git|hg)\s+status`,
			wantErr: false,
		},
		{
			name:    "nested quantifier - plus plus",
			pattern: `(a+)+`,
			wantErr: true,
		},
		{
			name:    "nested quantifier - star star",
			pattern: `(.*)*`,
			wantErr: true,
		},
		{
			name:    "nested quantifier - plus star",
			pattern: `(.+)*`,
			wantErr: true,
		},
		{
			name:    "nested quantifier with curly brace",
			pattern: `(a+){2,}`,
			wantErr: true,
		},
		{
			name:    "large repetition",
			pattern: `a{100,}`,
			wantErr: true,
		},
		{
			name:    "large repetition exact",
			pattern: `a{1000}`,
			wantErr: true,
		},
		{
			name:    "safe small repetition",
			pattern: `a{1,10}`,
			wantErr: false,
		},
		{
			name:    "alternation with outer plus",
			pattern: `(a|b)+`,
			wantErr: true,
		},
		{
			name:    "alternation with outer star",
			pattern: `(a|b)*`,
			wantErr: true,
		},
		{
			name:    "alternation with outer curly brace",
			pattern: `(a|b){2,}`,
			wantErr: true,
		},
		{
			name:    "classic backtracking pattern",
			pattern: `(a|a)*b`,
			wantErr: true,
		},
		{
			name:    "overlapping alternation branches",
			pattern: `(a|ab)*`,
			wantErr: true,
		},
		{
			name:    "safe alternation without quantifier",
			pattern: `^(git|hg|svn)$`,
			wantErr: false,
		},
		{
			name:    "safe alternation with inner quantifier only",
			pattern: `^(foo+|bar)$`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegexSafety(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRegexSafety(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
		})
	}
}

func TestParseWhitelistPatternsJSON_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		testCommand string
		wantMatch   bool
	}{
		{
			name:        "case sensitive by default - lowercase matches",
			jsonStr:     `[{"pattern": "^mycommand(\\s|$)"}]`,
			testCommand: "mycommand arg",
			wantMatch:   true,
		},
		{
			name:        "case sensitive by default - uppercase does not match",
			jsonStr:     `[{"pattern": "^mycommand(\\s|$)"}]`,
			testCommand: "MYCOMMAND arg",
			wantMatch:   false,
		},
		{
			name:        "case insensitive flag - lowercase matches",
			jsonStr:     `[{"pattern": "^mycommand(\\s|$)", "case_insensitive": true}]`,
			testCommand: "mycommand arg",
			wantMatch:   true,
		},
		{
			name:        "case insensitive flag - uppercase matches",
			jsonStr:     `[{"pattern": "^mycommand(\\s|$)", "case_insensitive": true}]`,
			testCommand: "MYCOMMAND arg",
			wantMatch:   true,
		},
		{
			name:        "case insensitive flag - mixed case matches",
			jsonStr:     `[{"pattern": "^mycommand(\\s|$)", "case_insensitive": true}]`,
			testCommand: "MyCommand arg",
			wantMatch:   true,
		},
		{
			name:        "already has (?i) prefix - not duplicated",
			jsonStr:     `[{"pattern": "(?i)^mycommand(\\s|$)", "case_insensitive": true}]`,
			testCommand: "MYCOMMAND arg",
			wantMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns, err := ParseWhitelistPatternsJSON(tt.jsonStr)
			if err != nil {
				t.Fatalf("ParseWhitelistPatternsJSON() unexpected error: %v", err)
			}
			if len(patterns) != 1 {
				t.Fatalf("expected 1 pattern, got %d", len(patterns))
			}

			whitelist := NewCommandWhitelist(patterns)
			allowed, _ := whitelist.IsAllowed(tt.testCommand)
			if allowed != tt.wantMatch {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.testCommand, allowed, tt.wantMatch)
			}
		})
	}
}

func TestParseWhitelistPatternsJSON_ReDoSRejection(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		wantErr bool
	}{
		{
			name:    "nested quantifier rejected",
			jsonStr: `[{"pattern": "(a+)+"}]`,
			wantErr: true,
		},
		{
			name:    "large repetition rejected",
			jsonStr: `[{"pattern": "a{100,}"}]`,
			wantErr: true,
		},
		{
			name:    "nested quantifier in exclude rejected",
			jsonStr: `[{"pattern": "^cmd", "exclude_pattern": "(a+)+"}]`,
			wantErr: true,
		},
		{
			name:    "safe pattern accepted",
			jsonStr: `[{"pattern": "^ls(\\s|$)"}]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseWhitelistPatternsJSON(tt.jsonStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseWhitelistPatternsJSON(%q) error = %v, wantErr %v", tt.jsonStr, err, tt.wantErr)
			}
		})
	}
}

func TestIsAllowedWithPipes_QuoteBypass(t *testing.T) {
	// Test that quote-aware splitting prevents bypass attacks
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^echo(\s|$)`), Description: "echo"},
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: "ls"},
		{Pattern: regexp.MustCompile(`^cat(\s|$)`), Description: "cat"},
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
	}{
		{
			name:        "safe echo command",
			command:     `echo "hello world"`,
			wantAllowed: true,
		},
		{
			name:        "bypass attempt - pipe in quotes piped to bash",
			command:     `echo "ls | rm -rf /" | bash`,
			wantAllowed: false, // bash is not whitelisted
		},
		{
			name:        "safe piped command",
			command:     "ls | cat",
			wantAllowed: true,
		},
		{
			name:        "unbalanced quotes blocked",
			command:     `echo "hello`,
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowedWithPipes(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowedWithPipes(%q) = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
		})
	}
}

func TestAwkCommand_SecurityPatterns(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		description string
	}{
		// Safe awk commands (allowed)
		{
			name:        "simple awk print",
			command:     "awk '{print $1}'",
			wantAllowed: true,
			description: "basic awk should be allowed",
		},
		{
			name:        "awk with field separator",
			command:     "awk -F: '{print $1}' /etc/passwd",
			wantAllowed: true,
			description: "awk with -F should be allowed",
		},
		{
			name:        "awk with pattern",
			command:     "awk '/pattern/ {print}' file.txt",
			wantAllowed: true,
			description: "awk with pattern matching should be allowed",
		},
		{
			name:        "awk NR variable",
			command:     "awk 'NR > 1 {print $0}'",
			wantAllowed: true,
			description: "awk with NR should be allowed",
		},

		// Dangerous awk commands (blocked)
		{
			name:        "awk with system call",
			command:     `awk 'BEGIN { system("rm -rf /") }'`,
			wantAllowed: false,
			description: "awk with system() should be blocked",
		},
		{
			name:        "awk with system call spaces",
			command:     `awk 'BEGIN { system ("ls") }'`,
			wantAllowed: false,
			description: "awk with system () should be blocked",
		},
		{
			name:        "awk with getline",
			command:     `awk '{getline < "/etc/passwd"; print}'`,
			wantAllowed: false,
			description: "awk with getline should be blocked",
		},
		{
			name:        "awk with output redirect",
			command:     `awk '{print > "file.txt"}'`,
			wantAllowed: false,
			description: "awk with > redirect should be blocked",
		},
		{
			name:        "awk with append redirect",
			command:     `awk '{print >> "file.txt"}'`,
			wantAllowed: false,
			description: "awk with >> redirect should be blocked",
		},
		{
			name:        "awk with pipe",
			command:     `awk '{print | "bash"}'`,
			wantAllowed: false,
			description: "awk with pipe should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) = %v, want %v (%s)", tt.command, allowed, tt.wantAllowed, tt.description)
			}
		})
	}
}

func TestSedCommand_SecurityPatterns(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		description string
	}{
		// Safe sed commands (allowed)
		{
			name:        "simple sed substitution",
			command:     "sed 's/old/new/'",
			wantAllowed: true,
			description: "basic sed should be allowed",
		},
		{
			name:        "sed with global flag",
			command:     "sed 's/old/new/g' file.txt",
			wantAllowed: true,
			description: "sed with /g flag should be allowed",
		},
		{
			name:        "sed with print flag",
			command:     "sed -n 's/pattern/replace/p' file.txt",
			wantAllowed: true,
			description: "sed -n with /p should be allowed",
		},
		{
			name:        "sed delete lines",
			command:     "sed '/pattern/d' file.txt",
			wantAllowed: true,
			description: "sed delete (to stdout) should be allowed",
		},
		{
			name:        "sed print specific lines",
			command:     "sed -n '1,10p' file.txt",
			wantAllowed: true,
			description: "sed print range should be allowed",
		},

		// Dangerous sed commands (blocked)
		{
			name:        "sed in-place edit",
			command:     "sed -i 's/old/new/' file.txt",
			wantAllowed: false,
			description: "sed -i should be blocked",
		},
		{
			name:        "sed in-place at end",
			command:     "sed 's/old/new/' -i",
			wantAllowed: false,
			description: "sed with -i at end should be blocked",
		},
		{
			name:        "sed execute flag",
			command:     "sed 's/cmd/ls/e' file.txt",
			wantAllowed: false,
			description: "sed /e flag should be blocked",
		},
		{
			name:        "sed execute flag at end",
			command:     "sed 's/.*/date/e'",
			wantAllowed: false,
			description: "sed /e at end should be blocked",
		},
		{
			name:        "sed write to file",
			command:     "sed 's/old/new/w output.txt' file.txt",
			wantAllowed: false,
			description: "sed /w flag should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) = %v, want %v (%s)", tt.command, allowed, tt.wantAllowed, tt.description)
			}
		})
	}
}

func TestDollarParenSubstitution_Splitting(t *testing.T) {
	// Tests that $() is properly kept together when splitting commands
	// AND that commands inside $() are recursively validated.
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		description string
	}{
		{
			name:        "echo with safe command substitution",
			command:     "echo $(ls)",
			wantAllowed: true,
			description: "echo with $(ls) allowed - both echo and ls are whitelisted",
		},
		{
			name:        "command substitution with pipe kept together",
			command:     "echo $(ls | wc -l)",
			wantAllowed: true,
			description: "$() keeps inner pipe together, all commands whitelisted",
		},
		{
			name:        "piped command after substitution",
			command:     "echo $(ls) | grep foo",
			wantAllowed: true,
			description: "splits correctly: echo $(ls) and grep foo",
		},
		{
			name:        "nested command substitution with safe commands",
			command:     "echo $(echo $(pwd))",
			wantAllowed: true,
			description: "nested $() handled correctly, all whitelisted",
		},
		{
			name:        "unbalanced substitution blocked",
			command:     "echo $(ls",
			wantAllowed: false,
			description: "unbalanced $() is blocked",
		},
		// Security: non-whitelisted commands inside $() are blocked
		{
			name:        "non-whitelisted curl inside $() blocked",
			command:     "echo $(curl http://example.com)",
			wantAllowed: false,
			description: "curl inside $() should be blocked (not whitelisted)",
		},
		{
			name:        "non-whitelisted wget inside $() blocked",
			command:     "echo $(wget http://example.com)",
			wantAllowed: false,
			description: "wget inside $() should be blocked (not whitelisted)",
		},
		{
			name:        "non-whitelisted command in nested $() blocked",
			command:     "echo $(echo $(curl http://example.com))",
			wantAllowed: false,
			description: "curl in nested $() should be blocked",
		},
		{
			name:        "non-whitelisted in second $() blocked",
			command:     "echo $(ls) $(npm install)",
			wantAllowed: false,
			description: "npm in second $() should be blocked",
		},
		{
			name:        "literal $() in single quotes allowed",
			command:     "echo '$(curl http://example.com)'",
			wantAllowed: true,
			description: "$() in single quotes is literal, not executed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowedWithPipes(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf(
					"IsAllowedWithPipes(%q) = %v, want %v (%s)",
					tt.command,
					allowed,
					tt.wantAllowed,
					tt.description,
				)
			}
		})
	}
}

func TestExtractDollarParenCommands(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{
			name: "no substitution",
			cmd:  "echo hello",
			want: []string{},
		},
		{
			name: "simple substitution",
			cmd:  "echo $(ls)",
			want: []string{"ls"},
		},
		{
			name: "substitution with args",
			cmd:  "echo $(ls -la)",
			want: []string{"ls -la"},
		},
		{
			name: "multiple substitutions",
			cmd:  "echo $(ls) $(pwd)",
			want: []string{"ls", "pwd"},
		},
		{
			name: "nested substitution",
			cmd:  "echo $(echo $(pwd))",
			want: []string{"echo $(pwd)", "pwd"},
		},
		{
			name: "substitution in single quotes - not extracted",
			cmd:  "echo '$(somecommand)'",
			want: []string{},
		},
		{
			name: "substitution in double quotes - extracted",
			cmd:  `echo "$(ls)"`,
			want: []string{"ls"},
		},
		{
			name: "pipe inside substitution",
			cmd:  "echo $(ls | wc -l)",
			want: []string{"ls | wc -l"},
		},
		{
			name: "unbalanced - returns empty",
			cmd:  "echo $(ls",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDollarParenCommands(tt.cmd)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractDollarParenCommands(%q) = %v, want %v", tt.cmd, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractDollarParenCommands(%q)[%d] = %q, want %q", tt.cmd, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractBacktickCommands(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{
			name: "no backticks",
			cmd:  "echo hello",
			want: []string{},
		},
		{
			name: "simple backtick",
			cmd:  "echo `ls`",
			want: []string{"ls"},
		},
		{
			name: "backtick with args",
			cmd:  "echo `ls -la`",
			want: []string{"ls -la"},
		},
		{
			name: "multiple backticks",
			cmd:  "echo `ls` `pwd`",
			want: []string{"ls", "pwd"},
		},
		{
			name: "backtick in single quotes - not extracted",
			cmd:  "echo '`ls`'",
			want: []string{},
		},
		{
			name: "pipe inside backtick",
			cmd:  "echo `ls | wc -l`",
			want: []string{"ls | wc -l"},
		},
		{
			name: "escaped backtick inside",
			cmd:  "echo `echo \\`pwd\\``",
			want: []string{"echo `pwd`"},
		},
		{
			name: "unbalanced - returns empty",
			cmd:  "echo `ls",
			want: []string{},
		},
		{
			name: "$() inside backtick",
			cmd:  "echo `echo $(pwd)`",
			want: []string{"echo $(pwd)", "pwd"},
		},
		{
			name: "backtick with complex command",
			cmd:  "VAR=`cat /etc/passwd`",
			want: []string{"cat /etc/passwd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBacktickCommands(tt.cmd)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractBacktickCommands(%q) = %v, want %v", tt.cmd, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractBacktickCommands(%q)[%d] = %q, want %q", tt.cmd, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSentinelErrors_SplitCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr error
	}{
		{"unbalanced_double_quotes", `echo "hello`, ErrUnbalancedQuotes},
		{"unbalanced_single_quotes", `echo 'hello`, ErrUnbalancedQuotes},
		{"trailing_escape", `echo test\`, ErrUnbalancedQuotes},
		{"unbalanced_dollar_paren", `echo $(ls`, ErrUnbalancedParens},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := splitCommandSegmentsQuoteAware(tt.cmd)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("splitCommandSegmentsQuoteAware(%q) error = %v, want %v", tt.cmd, err, tt.wantErr)
			}
		})
	}
}

func TestSentinelErrors_ValidateRegex(t *testing.T) {
	err := validateRegexSafety(`(a+)+`)
	if !errors.Is(err, ErrNestedQuantifiers) {
		t.Errorf("validateRegexSafety() error = %v, want %v", err, ErrNestedQuantifiers)
	}
}

func TestSentinelErrors_ParsePattern(t *testing.T) {
	t.Run("pattern_too_long", func(t *testing.T) {
		longPattern := strings.Repeat("a", MaxCommandLength+1)
		_, err := parseAndValidatePattern(longPattern)
		if !errors.Is(err, ErrPatternTooLong) {
			t.Errorf("parseAndValidatePattern() error = %v, want %v", err, ErrPatternTooLong)
		}
	})

	t.Run("pattern_required", func(t *testing.T) {
		_, err := parseSingleWhitelistPattern(WhitelistPatternJSON{})
		if !errors.Is(err, ErrPatternRequired) {
			t.Errorf("parseSingleWhitelistPattern() error = %v, want %v", err, ErrPatternRequired)
		}
	})
}

func TestCommandWhitelist_NilAndEmpty(t *testing.T) {
	tests := []struct {
		name     string
		patterns []WhitelistPattern
		command  string
		want     bool
	}{
		{"nil_patterns", nil, "ls", false},
		{"empty_patterns", []WhitelistPattern{}, "ls", false},
		{"empty_command", []WhitelistPattern{MustSimple("ls", "ls")}, "", false},
		{"whitespace_only_command", []WhitelistPattern{MustSimple("ls", "ls")}, "   ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewCommandWhitelist(tt.patterns)
			got, _ := w.IsAllowed(tt.command)
			if got != tt.want {
				t.Errorf("IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandWhitelist_MaxLengthBoundary(t *testing.T) {
	w := NewCommandWhitelist([]WhitelistPattern{MustSimple("ls", "list")})

	// Note: IsAllowedWithPipes uses `len(cmd) > MaxCommandLength`, so:
	// - Exactly at MaxCommandLength: allowed (if pattern matches)
	// - One over MaxCommandLength: rejected

	// Just under MaxCommandLength (9999 chars) - should match pattern
	underCmd := "ls " + strings.Repeat("a", MaxCommandLength-4)
	if allowed, _ := w.IsAllowedWithPipes(underCmd); !allowed {
		t.Errorf("command under MaxCommandLength (%d chars) should be allowed", len(underCmd))
	}

	// Exactly at MaxCommandLength (10000 chars) - should match pattern
	exactCmd := "ls " + strings.Repeat("a", MaxCommandLength-3)
	if allowed, _ := w.IsAllowedWithPipes(exactCmd); !allowed {
		t.Errorf("command exactly at MaxCommandLength (%d chars) should be allowed", len(exactCmd))
	}

	// One over MaxCommandLength (10001 chars) - should be rejected
	overCmd := "ls " + strings.Repeat("a", MaxCommandLength-2)
	if allowed, _ := w.IsAllowedWithPipes(overCmd); allowed {
		t.Errorf("command over MaxCommandLength (%d chars) should be rejected", len(overCmd))
	}
}

func TestCommandWhitelist_UnicodeAndSpecialCharacters(t *testing.T) {
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^echo(\s|$)`), Description: "echo command"},
		{Pattern: regexp.MustCompile(`^ls(\s|$)`), Description: "list directory"},
		{Pattern: regexp.MustCompile(`^cat(\s|$)`), Description: "display file"},
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
		wantDesc    string
	}{
		// Unicode in arguments
		{
			name:        "echo with unicode argument",
			command:     "echo Hello世界",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		{
			name:        "echo with emoji",
			command:     "echo 🎉🎊",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		{
			name:        "ls with unicode path",
			command:     "ls /path/to/目录",
			wantAllowed: true,
			wantDesc:    "list directory",
		},
		{
			name:        "cat with unicode filename",
			command:     "cat файл.txt",
			wantAllowed: true,
			wantDesc:    "display file",
		},
		// Mixed ASCII/Unicode
		{
			name:        "mixed ASCII and unicode",
			command:     "echo Hello世界andMore",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// High codepoint characters
		{
			name:        "high codepoint mathematical symbols",
			command:     "echo ∑∏∫∂",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// Unicode in command name (not allowed since pattern requires ASCII "echo")
		{
			name:        "unicode command name not matched",
			command:     "эхо hello",
			wantAllowed: false,
			wantDesc:    "",
		},
		// Control characters (should be handled safely)
		{
			name:        "command with tab character",
			command:     "echo hello\tworld",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// Null byte in command (security check - should be handled)
		{
			name:        "command with null byte",
			command:     "echo hello\x00world",
			wantAllowed: true, // Pattern matches, null byte is just data
			wantDesc:    "echo command",
		},
		// Bell character
		{
			name:        "command with bell character",
			command:     "echo hello\x07world",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// Escape character
		{
			name:        "command with escape character",
			command:     "echo hello\x1bworld",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// RTL override character (potential security issue in display)
		{
			name:        "command with RTL override",
			command:     "echo hello\u202eworld",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// Zero-width characters
		{
			name:        "command with zero-width space",
			command:     "echo hello\u200bworld",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		{
			name:        "command with zero-width joiner",
			command:     "echo hello\u200dworld",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// Combining characters
		{
			name:        "command with combining diacritical",
			command:     "echo café",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
		// Surrogate pair (emoji that requires 4 bytes in UTF-8)
		{
			name:        "command with surrogate pair emoji",
			command:     "echo 🇺🇸",
			wantAllowed: true,
			wantDesc:    "echo command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, desc := whitelist.IsAllowed(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowed(%q) allowed = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
			if desc != tt.wantDesc {
				t.Errorf("IsAllowed(%q) desc = %q, want %q", tt.command, desc, tt.wantDesc)
			}
		})
	}
}

func TestCommandWhitelist_IsAllowedWithPipes_UnicodeInPipes(t *testing.T) {
	patterns := []WhitelistPattern{
		{Pattern: regexp.MustCompile(`^echo(\s|$)`), Description: "echo command"},
		{Pattern: regexp.MustCompile(`^grep(\s|$)`), Description: "search command"},
	}
	whitelist := NewCommandWhitelist(patterns)

	tests := []struct {
		name        string
		command     string
		wantAllowed bool
	}{
		{
			name:        "pipe with unicode in both segments",
			command:     "echo 你好 | grep 世界",
			wantAllowed: true,
		},
		{
			name:        "pipe with emoji argument",
			command:     "echo 🎉 | grep 🎊",
			wantAllowed: true,
		},
		{
			name:        "pipe with mixed unicode and ASCII",
			command:     "echo helloмир | grep test世界",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := whitelist.IsAllowedWithPipes(tt.command)
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowedWithPipes(%q) = %v, want %v", tt.command, allowed, tt.wantAllowed)
			}
		})
	}
}

func TestExtractDollarParenCommands_MaxDepth(t *testing.T) {
	// Build a command with 25 nesting levels (exceeds MaxRecursionDepth of 20)
	cmd := "echo "
	var cmdSb1920 strings.Builder
	for range 25 {
		cmdSb1920.WriteString("$(")
	}
	cmd += cmdSb1920.String()
	cmd += "pwd"
	var cmdSb1924 strings.Builder
	for range 25 {
		cmdSb1924.WriteString(")")
	}
	cmd += cmdSb1924.String()

	// Should not panic and should return limited results
	results := ExtractDollarParenCommands(cmd)

	// Verify we got some results but not all 25 levels
	if len(results) > MaxRecursionDepth {
		t.Errorf("extracted %d commands, expected <= %d", len(results), MaxRecursionDepth)
	}
}

func TestExtractBacktickCommands_MaxDepth(t *testing.T) {
	// Build a command with nested $() inside backticks that exceeds depth
	// Since backticks don't nest the same way, we test $() inside backticks
	cmd := "echo `echo "
	var cmdSb1941 strings.Builder
	for range 25 {
		cmdSb1941.WriteString("$(")
	}
	cmd += cmdSb1941.String()
	cmd += "pwd"
	var cmdSb1945 strings.Builder
	for range 25 {
		cmdSb1945.WriteString(")")
	}
	cmd += cmdSb1945.String()
	cmd += "`"

	// Should not panic and should return limited results
	results := ExtractBacktickCommands(cmd)

	// Verify we got some results but recursion stopped at max depth
	if len(results) > MaxRecursionDepth+1 { // +1 for the backtick itself
		t.Errorf("extracted %d commands, expected <= %d", len(results), MaxRecursionDepth+1)
	}
}

func TestIsAllowedWithPipes_MaxDepth(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	// Build a deeply nested command with whitelisted commands
	cmd := "echo "
	var cmdSb1965 strings.Builder
	for range 25 {
		cmdSb1965.WriteString("$(")
	}
	cmd += cmdSb1965.String()
	cmd += "pwd"
	var cmdSb1969 strings.Builder
	for range 25 {
		cmdSb1969.WriteString(")")
	}
	cmd += cmdSb1969.String()

	// Should be blocked due to excessive depth (even though all commands are whitelisted)
	allowed, _ := whitelist.IsAllowedWithPipes(cmd)
	if allowed {
		t.Error("expected deeply nested command to be blocked due to excessive recursion depth")
	}
}

func TestMaxRecursionDepth_BoundaryConditions(t *testing.T) {
	patterns := DefaultWhitelistPatterns()
	whitelist := NewCommandWhitelist(patterns)

	// Helper to build properly nested command: echo $(echo $(echo $(pwd)))
	// Each level must have a valid whitelisted command, not just $(...)
	buildNestedCommand := func(depth int) string {
		if depth == 0 {
			return "pwd"
		}
		inner := "pwd"
		for range depth {
			inner = "echo $(" + inner + ")"
		}
		return inner
	}

	// The boundary check is: depth >= MaxRecursionDepth
	// So depth 19 passes (19 >= 20 = false), depth 20 fails (20 >= 20 = true)
	tests := []struct {
		name        string
		depth       int
		wantAllowed bool
	}{
		{
			name:        "depth 19 (one under limit) should be allowed",
			depth:       MaxRecursionDepth - 1,
			wantAllowed: true,
		},
		{
			name:        "depth 20 (at limit) should be rejected",
			depth:       MaxRecursionDepth,
			wantAllowed: false,
		},
		{
			name:        "depth 21 (one over limit) should be rejected",
			depth:       MaxRecursionDepth + 1,
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildNestedCommand(tt.depth)
			allowed, _ := whitelist.IsAllowedWithPipes(cmd)
			if allowed != tt.wantAllowed {
				t.Errorf("depth %d: IsAllowedWithPipes() = %v, want %v", tt.depth, allowed, tt.wantAllowed)
			}
		})
	}
}

func TestExtractDollarParenCommands_BoundaryDepth(t *testing.T) {
	// Helper to build properly nested command: echo $(echo $(echo $(pwd)))
	buildNestedCommand := func(depth int) string {
		if depth == 0 {
			return "pwd"
		}
		inner := "pwd"
		for range depth {
			inner = "echo $(" + inner + ")"
		}
		return inner
	}

	// Extraction extracts commands from substitutions at each level
	// For depth N, we expect N extracted commands (one per nesting level)
	// Extraction is limited by depth < MaxRecursionDepth check
	tests := []struct {
		name     string
		depth    int
		maxCount int // Maximum expected extracted commands
	}{
		{
			name:     "depth 19 extracts all levels",
			depth:    MaxRecursionDepth - 1,
			maxCount: MaxRecursionDepth - 1,
		},
		{
			name:     "depth 20 limited by MaxRecursionDepth",
			depth:    MaxRecursionDepth,
			maxCount: MaxRecursionDepth,
		},
		{
			name:     "depth 21 limited by MaxRecursionDepth",
			depth:    MaxRecursionDepth + 1,
			maxCount: MaxRecursionDepth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildNestedCommand(tt.depth)
			results := ExtractDollarParenCommands(cmd)
			if len(results) > tt.maxCount {
				t.Errorf("depth %d: extracted %d commands, expected <= %d", tt.depth, len(results), tt.maxCount)
			}
		})
	}
}

func TestIsDangerousCommand_BoundaryDepth(t *testing.T) {
	// Helper to build properly nested command: echo $(echo $(echo $(pwd)))
	buildNestedCommand := func(depth int) string {
		if depth == 0 {
			return "pwd"
		}
		inner := "pwd"
		for range depth {
			inner = "echo $(" + inner + ")"
		}
		return inner
	}

	// IsDangerousCommand uses depth >= MaxRecursionDepth check
	// So depth 19 is safe (19 >= 20 = false), depth 20 is dangerous (20 >= 20 = true)
	tests := []struct {
		name          string
		depth         int
		wantDangerous bool
	}{
		{
			name:          "depth 19 (under limit) not dangerous",
			depth:         MaxRecursionDepth - 1,
			wantDangerous: false,
		},
		{
			name:          "depth 20 (at limit) is dangerous",
			depth:         MaxRecursionDepth,
			wantDangerous: true,
		},
		{
			name:          "depth 21 (over limit) is dangerous",
			depth:         MaxRecursionDepth + 1,
			wantDangerous: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildNestedCommand(tt.depth)
			dangerous, reason := IsDangerousCommand(cmd)
			if dangerous != tt.wantDangerous {
				t.Errorf("depth %d: IsDangerousCommand() = %v (reason: %s), want dangerous=%v",
					tt.depth, dangerous, reason, tt.wantDangerous)
			}
		})
	}
}
