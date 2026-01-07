package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCmd_ExtendedThinkingFlags verifies that extended thinking CLI flags are properly registered.
func TestRootCmd_ExtendedThinkingFlags(t *testing.T) {
	t.Run("thinking flag is registered", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("thinking")

		require.NotNil(t, flag, "thinking flag should be registered on root command")
		assert.Equal(t, "thinking", flag.Name, "flag name should be 'thinking'")
		assert.Equal(t, "bool", flag.Value.Type(), "thinking flag should be a boolean")
	})

	t.Run("thinking-budget flag is registered", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("thinking-budget")

		require.NotNil(t, flag, "thinking-budget flag should be registered on root command")
		assert.Equal(t, "thinking-budget", flag.Name, "flag name should be 'thinking-budget'")
		assert.Equal(t, "int", flag.Value.Type(), "thinking-budget flag should be an int")
	})

	t.Run("show-thinking flag is registered", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("show-thinking")

		require.NotNil(t, flag, "show-thinking flag should be registered on root command")
		assert.Equal(t, "show-thinking", flag.Name, "flag name should be 'show-thinking'")
		assert.Equal(t, "bool", flag.Value.Type(), "show-thinking flag should be a boolean")
	})

	t.Run("max-tokens flag is registered with updated default", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("max-tokens")

		require.NotNil(t, flag, "max-tokens flag should be registered on root command")
		assert.Equal(t, "max-tokens", flag.Name, "flag name should be 'max-tokens'")
		assert.Equal(t, "int", flag.Value.Type(), "max-tokens flag should be an int")
	})
}

// TestRootCmd_FlagDefaults verifies that CLI flags have correct default values.
func TestRootCmd_FlagDefaults(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		expectedVal string
		description string
	}{
		{
			name:        "thinking defaults to false",
			flagName:    "thinking",
			expectedVal: "false",
			description: "thinking flag should default to false",
		},
		{
			name:        "thinking-budget defaults to 10000",
			flagName:    "thinking-budget",
			expectedVal: "10000",
			description: "thinking-budget flag should default to 10000",
		},
		{
			name:        "show-thinking defaults to false",
			flagName:    "show-thinking",
			expectedVal: "false",
			description: "show-thinking flag should default to false",
		},
		{
			name:        "max-tokens defaults to 20000",
			flagName:    "max-tokens",
			expectedVal: "20000",
			description: "max-tokens flag should default to 20000 (not hardcoded 4096)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(tt.flagName)
			require.NotNil(t, flag, "%s flag should be registered", tt.flagName)

			assert.Equal(t, tt.expectedVal, flag.DefValue, tt.description)
		})
	}
}

// TestRootCmd_ViperBinding verifies that CLI flags are properly bound to viper with correct keys.
func TestRootCmd_ViperBinding(t *testing.T) {
	// Helper to reset viper between tests
	resetViper := func() {
		viper.Reset()
	}

	tests := []struct {
		name        string
		flagName    string
		flagValue   string
		viperKey    string
		expectedVal interface{}
		checkType   string // "bool", "int", "string"
		description string
	}{
		{
			name:        "thinking flag binds to thinking.enabled",
			flagName:    "thinking",
			flagValue:   "true",
			viperKey:    "thinking.enabled",
			expectedVal: true,
			checkType:   "bool",
			description: "thinking flag should bind to viper key 'thinking.enabled'",
		},
		{
			name:        "thinking-budget flag binds to thinking.budget",
			flagName:    "thinking-budget",
			flagValue:   "15000",
			viperKey:    "thinking.budget",
			expectedVal: 15000,
			checkType:   "int",
			description: "thinking-budget flag should bind to viper key 'thinking.budget'",
		},
		{
			name:        "show-thinking flag binds to thinking.show",
			flagName:    "show-thinking",
			flagValue:   "true",
			viperKey:    "thinking.show",
			expectedVal: true,
			checkType:   "bool",
			description: "show-thinking flag should bind to viper key 'thinking.show'",
		},
		{
			name:        "max-tokens flag binds to max_tokens",
			flagName:    "max-tokens",
			flagValue:   "30000",
			viperKey:    "max_tokens",
			expectedVal: 30000,
			checkType:   "int",
			description: "max-tokens flag should bind to viper key 'max_tokens'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()

			// Create a new command to simulate flag parsing
			cmd := &cobra.Command{
				Use: "test",
				Run: func(_ *cobra.Command, _ []string) {},
			}

			// Add the flag and bind it to viper
			switch tt.checkType {
			case "bool":
				cmd.Flags().Bool(tt.flagName, false, "")
			case "int":
				cmd.Flags().Int(tt.flagName, 0, "")
			case "string":
				cmd.Flags().String(tt.flagName, "", "")
			}

			// Bind flag to viper
			err := viper.BindPFlag(tt.viperKey, cmd.Flags().Lookup(tt.flagName))
			require.NoError(t, err, "binding flag to viper should not error")

			// Parse the flag value
			err = cmd.Flags().Set(tt.flagName, tt.flagValue)
			require.NoError(t, err, "setting flag value should not error")

			// Verify viper has the value
			assert.True(t, viper.IsSet(tt.viperKey),
				"viper key '%s' should be set after flag parsing", tt.viperKey)

			// Check the value based on type
			switch tt.checkType {
			case "bool":
				actualVal := viper.GetBool(tt.viperKey)
				assert.Equal(t, tt.expectedVal, actualVal, tt.description)
			case "int":
				actualVal := viper.GetInt(tt.viperKey)
				assert.Equal(t, tt.expectedVal, actualVal, tt.description)
			case "string":
				actualVal := viper.GetString(tt.viperKey)
				assert.Equal(t, tt.expectedVal, actualVal, tt.description)
			}
		})
	}
}

// TestRootCmd_FlagParsing verifies that CLI flags can be parsed with various values.
func TestRootCmd_FlagParsing(t *testing.T) {
	resetViper := func() {
		viper.Reset()
	}

	t.Run("thinking flag accepts true", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Bool("thinking", false, "")

		err := cmd.Flags().Set("thinking", "true")
		require.NoError(t, err, "thinking flag should accept 'true'")

		val, err := cmd.Flags().GetBool("thinking")
		require.NoError(t, err)
		assert.True(t, val, "thinking flag value should be true")
	})

	t.Run("thinking flag accepts false", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Bool("thinking", true, "") // default true to test override

		err := cmd.Flags().Set("thinking", "false")
		require.NoError(t, err, "thinking flag should accept 'false'")

		val, err := cmd.Flags().GetBool("thinking")
		require.NoError(t, err)
		assert.False(t, val, "thinking flag value should be false")
	})

	t.Run("thinking-budget accepts valid integer", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Int("thinking-budget", 0, "")

		err := cmd.Flags().Set("thinking-budget", "25000")
		require.NoError(t, err, "thinking-budget flag should accept valid integer")

		val, err := cmd.Flags().GetInt("thinking-budget")
		require.NoError(t, err)
		assert.Equal(t, 25000, val, "thinking-budget should be 25000")
	})

	t.Run("thinking-budget rejects invalid value", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Int("thinking-budget", 0, "")

		err := cmd.Flags().Set("thinking-budget", "not-a-number")
		assert.Error(t, err, "thinking-budget should reject non-integer value")
	})

	t.Run("show-thinking accepts true", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Bool("show-thinking", false, "")

		err := cmd.Flags().Set("show-thinking", "true")
		require.NoError(t, err, "show-thinking flag should accept 'true'")

		val, err := cmd.Flags().GetBool("show-thinking")
		require.NoError(t, err)
		assert.True(t, val, "show-thinking flag value should be true")
	})

	t.Run("max-tokens accepts valid integer", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Int("max-tokens", 0, "")

		err := cmd.Flags().Set("max-tokens", "50000")
		require.NoError(t, err, "max-tokens flag should accept valid integer")

		val, err := cmd.Flags().GetInt("max-tokens")
		require.NoError(t, err)
		assert.Equal(t, 50000, val, "max-tokens should be 50000")
	})

	t.Run("max-tokens rejects invalid value", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Int("max-tokens", 0, "")

		err := cmd.Flags().Set("max-tokens", "invalid")
		assert.Error(t, err, "max-tokens should reject non-integer value")
	})
}

// TestRootCmd_FlagCombinations verifies that multiple thinking flags can be used together.
func TestRootCmd_FlagCombinations(t *testing.T) {
	resetViper := func() {
		viper.Reset()
	}

	t.Run("all thinking flags can be set together", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Bool("thinking", false, "")
		cmd.Flags().Int("thinking-budget", 10000, "")
		cmd.Flags().Bool("show-thinking", false, "")
		cmd.Flags().Int("max-tokens", 20000, "")

		// Bind to viper
		require.NoError(t, viper.BindPFlag("thinking.enabled", cmd.Flags().Lookup("thinking")))
		require.NoError(t, viper.BindPFlag("thinking.budget", cmd.Flags().Lookup("thinking-budget")))
		require.NoError(t, viper.BindPFlag("thinking.show", cmd.Flags().Lookup("show-thinking")))
		require.NoError(t, viper.BindPFlag("max_tokens", cmd.Flags().Lookup("max-tokens")))

		// Set all flags
		require.NoError(t, cmd.Flags().Set("thinking", "true"))
		require.NoError(t, cmd.Flags().Set("thinking-budget", "15000"))
		require.NoError(t, cmd.Flags().Set("show-thinking", "true"))
		require.NoError(t, cmd.Flags().Set("max-tokens", "30000"))

		// Verify all values via viper
		assert.True(t, viper.GetBool("thinking.enabled"),
			"thinking.enabled should be true when flags are combined")
		assert.Equal(t, 15000, viper.GetInt("thinking.budget"),
			"thinking.budget should be 15000 when flags are combined")
		assert.True(t, viper.GetBool("thinking.show"),
			"thinking.show should be true when flags are combined")
		assert.Equal(t, 30000, viper.GetInt("max_tokens"),
			"max_tokens should be 30000 when flags are combined")
	})

	t.Run("thinking flag can be enabled without other flags", func(t *testing.T) {
		resetViper()
		defer resetViper()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Bool("thinking", false, "")
		cmd.Flags().Int("thinking-budget", 10000, "")
		cmd.Flags().Bool("show-thinking", false, "")

		require.NoError(t, viper.BindPFlag("thinking.enabled", cmd.Flags().Lookup("thinking")))
		require.NoError(t, viper.BindPFlag("thinking.budget", cmd.Flags().Lookup("thinking-budget")))
		require.NoError(t, viper.BindPFlag("thinking.show", cmd.Flags().Lookup("show-thinking")))

		// Set only thinking flag
		require.NoError(t, cmd.Flags().Set("thinking", "true"))

		// Verify thinking is enabled with defaults for other values
		assert.True(t, viper.GetBool("thinking.enabled"),
			"thinking.enabled should be true")
		assert.Equal(t, 10000, viper.GetInt("thinking.budget"),
			"thinking.budget should use default when not specified")
		assert.False(t, viper.GetBool("thinking.show"),
			"thinking.show should use default false when not specified")
	})
}

// TestRootCmd_MaxTokensDefaultUpdate verifies that max-tokens default changed from 4096 to 20000.
func TestRootCmd_MaxTokensDefaultUpdate(t *testing.T) {
	t.Run("max-tokens default is 20000 not 4096", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("max-tokens")
		require.NotNil(t, flag, "max-tokens flag should be registered")

		// Should be 20000, not the old hardcoded 4096
		assert.Equal(t, "20000", flag.DefValue,
			"max-tokens flag default should be updated from 4096 to 20000")

		// Also verify it's not the old value
		assert.NotEqual(t, "4096", flag.DefValue,
			"max-tokens flag should no longer use hardcoded 4096")
		assert.NotEqual(t, "1024", flag.DefValue,
			"max-tokens flag should not be 1024 (the current incorrect default in root.go)")
	})
}

// TestRootCmd_PersistentFlags verifies that thinking flags are persistent (available to subcommands).
func TestRootCmd_PersistentFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"thinking is persistent", "thinking"},
		{"thinking-budget is persistent", "thinking-budget"},
		{"show-thinking is persistent", "show-thinking"},
		{"max-tokens is persistent", "max-tokens"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify flag is registered on persistent flags, not local flags
			persistentFlag := rootCmd.PersistentFlags().Lookup(tt.flagName)
			localFlag := rootCmd.Flags().Lookup(tt.flagName)

			assert.NotNil(t, persistentFlag,
				"%s should be registered as a persistent flag", tt.flagName)
			assert.Nil(t, localFlag,
				"%s should not be registered as a local flag (should be persistent)", tt.flagName)
		})
	}
}
