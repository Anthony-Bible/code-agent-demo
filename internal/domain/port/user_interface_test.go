package port

import (
	"context"
	"testing"
)

// TestUserInterfaceInterface_Contract validates that UserInterface interface exists with expected methods.
func TestUserInterfaceInterface_Contract(_ *testing.T) {
	// Verify that UserInterface interface exists
	var _ UserInterface = (*mockUserInterface)(nil)
}

// mockUserInterface is a minimal implementation to validate interface contract.
type mockUserInterface struct{}

func (m *mockUserInterface) GetUserInput(_ context.Context) (string, bool) {
	return "", false
}

func (m *mockUserInterface) DisplayMessage(_ string, _ string) error {
	return nil
}

func (m *mockUserInterface) DisplayError(_ error) error {
	return nil
}

func (m *mockUserInterface) DisplayToolResult(_, _, _ string) error {
	return nil
}

func (m *mockUserInterface) DisplaySystemMessage(_ string) error {
	return nil
}

func (m *mockUserInterface) DisplayThinking(_ string) error {
	return nil
}

func (m *mockUserInterface) BeginStreamingResponse() error {
	return nil
}

func (m *mockUserInterface) EndStreamingResponse() error {
	return nil
}

func (m *mockUserInterface) DisplayStreamingText(_ string) error {
	return nil
}

func (m *mockUserInterface) DisplaySubagentStatus(_, _, _ string) error {
	return nil
}

func (m *mockUserInterface) SetPrompt(_ string) error {
	return nil
}

func (m *mockUserInterface) ClearScreen() error {
	return nil
}

func (m *mockUserInterface) SetColorScheme(_ ColorScheme) error {
	return nil
}

func (m *mockUserInterface) ConfirmBashCommand(
	_ string,
	_ bool,
	_ string,
	_ string,
) bool {
	return false
}

// TestUserInterfaceGetUserInput_Exists validates GetUserInput method exists.
func TestUserInterfaceGetUserInput_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if GetUserInput method doesn't exist with correct signature
	_ = ui.GetUserInput
}

// TestUserInterfaceDisplayMessage_Exists validates DisplayMessage method exists.
func TestUserInterfaceDisplayMessage_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if DisplayMessage method doesn't exist with correct signature
	_ = ui.DisplayMessage
}

// TestUserInterfaceDisplayError_Exists validates DisplayError method exists.
func TestUserInterfaceDisplayError_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if DisplayError method doesn't exist with correct signature
	_ = ui.DisplayError
}

// TestUserInterfaceDisplayToolResult_Exists validates DisplayToolResult method exists.
func TestUserInterfaceDisplayToolResult_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if DisplayToolResult method doesn't exist with correct signature
	_ = ui.DisplayToolResult
}

// TestUserInterfaceDisplaySystemMessage_Exists validates DisplaySystemMessage method exists.
func TestUserInterfaceDisplaySystemMessage_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if DisplaySystemMessage method doesn't exist with correct signature
	_ = ui.DisplaySystemMessage
}

// TestUserInterfaceSetPrompt_Exists validates SetPrompt method exists.
func TestUserInterfaceSetPrompt_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if SetPrompt method doesn't exist with correct signature
	_ = ui.SetPrompt
}

// TestUserInterfaceClearScreen_Exists validates ClearScreen method exists.
func TestUserInterfaceClearScreen_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if ClearScreen method doesn't exist with correct signature
	_ = ui.ClearScreen
}

// TestUserInterfaceSetColorScheme_Exists validates SetColorScheme method exists.
func TestUserInterfaceSetColorScheme_Exists(_ *testing.T) {
	var ui UserInterface = (*mockUserInterface)(nil)

	// This will fail to compile if SetColorScheme method doesn't exist with correct signature
	_ = ui.SetColorScheme
}
