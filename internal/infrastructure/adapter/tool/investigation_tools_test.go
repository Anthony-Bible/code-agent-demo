package tool_test

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/tool"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// RED PHASE TDD: Tests for investigation tools (complete_investigation and escalate_investigation)
// These tests define the expected behavior for the new investigation tools.
// All tests should FAIL until the tools are implemented.
// =============================================================================

// investigationTestHelper contains common test setup utilities for investigation tools.
type investigationTestHelper struct {
	t       *testing.T
	adapter *tool.ExecutorAdapter
}

// defaultTestInvestigationID is a pre-registered investigation ID for tests.
const defaultTestInvestigationID = "test-investigation-id"

// newInvestigationTestHelper creates a new test helper with a file manager and adapter.
// It pre-registers a default investigation ID for tests that need a valid investigation.
func newInvestigationTestHelper(t *testing.T) *investigationTestHelper {
	t.Helper()
	tempDir := t.TempDir()
	fileManager := file.NewLocalFileManager(tempDir)
	adapter := tool.NewExecutorAdapter(fileManager)
	// Pre-register a default investigation ID for tests
	adapter.RegisterInvestigation(defaultTestInvestigationID)
	return &investigationTestHelper{
		t:       t,
		adapter: adapter,
	}
}

// executeCompleteInvestigation executes the complete_investigation tool with the given input.
func (h *investigationTestHelper) executeCompleteInvestigation(input string) (string, error) {
	h.t.Helper()
	return h.adapter.ExecuteTool(context.Background(), "complete_investigation", input)
}

// executeEscalateInvestigation executes the escalate_investigation tool with the given input.
func (h *investigationTestHelper) executeEscalateInvestigation(input string) (string, error) {
	h.t.Helper()
	return h.adapter.ExecuteTool(context.Background(), "escalate_investigation", input)
}

// getToolProperties extracts properties from a tool schema.
func (h *investigationTestHelper) getToolProperties(toolName string) (map[string]interface{}, error) {
	h.t.Helper()
	toolDef, found := h.adapter.GetTool(toolName)
	if !found {
		return nil, fmt.Errorf("%s tool should be registered", toolName)
	}

	schema := toolDef.InputSchema
	if schema == nil {
		return nil, errors.New("InputSchema should not be nil")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("properties should be a map, got %T", schema["properties"])
	}

	return properties, nil
}

// assertPropertyExists checks that a property exists and returns it as a map.
func (h *investigationTestHelper) assertPropertyExists(
	properties map[string]interface{},
	propName string,
) map[string]interface{} {
	h.t.Helper()
	prop, found := properties[propName]
	if !found {
		h.t.Errorf("schema should include '%s' property", propName)
		return nil
	}

	propMap, ok := prop.(map[string]interface{})
	if !ok {
		h.t.Errorf("%s should be a map, got %T", propName, prop)
		return nil
	}

	return propMap
}

// assertPropertyType checks that a property has the expected type.
func (h *investigationTestHelper) assertPropertyType(propMap map[string]interface{}, propName, expectedType string) {
	h.t.Helper()
	if propMap == nil {
		return
	}
	if propMap["type"] != expectedType {
		h.t.Errorf("%s type should be '%s', got %v", propName, expectedType, propMap["type"])
	}
}

// =============================================================================
// complete_investigation Tool Tests
// =============================================================================

func TestCompleteInvestigationTool_Registration(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tools, err := h.adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tl := range tools {
		if tl.Name == "complete_investigation" {
			found = true
			break
		}
	}

	if !found {
		t.Error("complete_investigation tool should be registered")
	}
}

func TestCompleteInvestigationTool_SchemaConfidenceProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	confidenceMap := h.assertPropertyExists(properties, "confidence")
	if confidenceMap == nil {
		return
	}

	h.assertPropertyType(confidenceMap, "confidence", "number")

	if confidenceMap["minimum"] != float64(0) {
		t.Errorf("confidence minimum should be 0, got %v", confidenceMap["minimum"])
	}
	if confidenceMap["maximum"] != float64(1) {
		t.Errorf("confidence maximum should be 1, got %v", confidenceMap["maximum"])
	}
}

func TestCompleteInvestigationTool_SchemaFindingsProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	findingsMap := h.assertPropertyExists(properties, "findings")
	h.assertPropertyType(findingsMap, "findings", "array")
}

func TestCompleteInvestigationTool_SchemaRootCauseProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	rootCauseMap := h.assertPropertyExists(properties, "root_cause")
	h.assertPropertyType(rootCauseMap, "root_cause", "string")
}

func TestCompleteInvestigationTool_SchemaRecommendedActionsProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	recommendedActionsMap := h.assertPropertyExists(properties, "recommended_actions")
	h.assertPropertyType(recommendedActionsMap, "recommended_actions", "array")
}

func TestCompleteInvestigationTool_RequiredFields(t *testing.T) {
	h := newInvestigationTestHelper(t)

	completeInvestigationTool, found := h.adapter.GetTool("complete_investigation")
	if !found {
		t.Fatal("complete_investigation tool should be registered")
	}

	requiredFields := completeInvestigationTool.RequiredFields
	confidenceRequired := false
	findingsRequired := false

	for _, field := range requiredFields {
		if field == "confidence" {
			confidenceRequired = true
		}
		if field == "findings" {
			findingsRequired = true
		}
	}

	if !confidenceRequired {
		t.Error("'confidence' should be a required field")
	}
	if !findingsRequired {
		t.Error("'findings' should be a required field")
	}
}

func TestCompleteInvestigationTool_ValidInput(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			name: "minimal valid input with required fields only",
			input: map[string]interface{}{
				"confidence": 0.85,
				"findings":   []string{"Found memory leak in service A", "Database connection pool exhausted"},
			},
		},
		{
			name: "full valid input with all fields",
			input: map[string]interface{}{
				"confidence":          0.95,
				"findings":            []string{"CPU spike detected", "Memory usage normal"},
				"root_cause":          "Runaway process in container xyz",
				"recommended_actions": []string{"Restart the container", "Add resource limits"},
			},
		},
		{
			name: "valid input with zero confidence",
			input: map[string]interface{}{
				"confidence": 0.0,
				"findings":   []string{"Unable to determine root cause"},
			},
		},
		{
			name: "valid input with maximum confidence",
			input: map[string]interface{}{
				"confidence": 1.0,
				"findings":   []string{"Definitive root cause identified"},
			},
		},
		{
			name: "valid input with empty optional fields",
			input: map[string]interface{}{
				"confidence":          0.5,
				"findings":            []string{"Partial analysis complete"},
				"root_cause":          "",
				"recommended_actions": []string{},
			},
		},
		{
			name: "valid input with single finding",
			input: map[string]interface{}{
				"confidence": 0.7,
				"findings":   []string{"Single finding"},
			},
		},
		{
			name: "valid input with multiple findings and actions",
			input: map[string]interface{}{
				"confidence":          0.88,
				"findings":            []string{"Finding 1", "Finding 2", "Finding 3", "Finding 4", "Finding 5"},
				"root_cause":          "Complex multi-factor issue",
				"recommended_actions": []string{"Action 1", "Action 2", "Action 3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register a fresh investigation for each subtest to avoid "already completed" errors
			invID := defaultTestInvestigationID + "-" + tt.name
			h.adapter.RegisterInvestigation(invID)
			tt.input["investigation_id"] = invID

			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			result, err := h.executeCompleteInvestigation(string(inputJSON))
			if err != nil {
				t.Errorf("ExecuteTool failed for valid input: %v", err)
			}

			if result == "" {
				t.Error("Expected non-empty result for valid input")
			}
		})
	}
}

func TestCompleteInvestigationTool_InvalidConfidence(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name       string
		confidence interface{}
		wantErr    string
	}{
		{"confidence below minimum (negative)", -0.1, "confidence"},
		{"confidence above maximum", 1.1, "confidence"},
		{"confidence way below minimum", -100.0, "confidence"},
		{"confidence way above maximum", 100.0, "confidence"},
		{"confidence as string", "0.5", "confidence"},
		{"confidence as null", nil, "confidence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"confidence":       tt.confidence,
				"findings":         []string{"Some finding"},
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid confidence %v, got nil", tt.confidence)
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestCompleteInvestigationTool_MissingRequired(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			"missing confidence",
			map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"findings":         []string{"Finding 1"},
			},
			"confidence",
		},
		{
			"missing findings",
			map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"confidence":       0.8,
			},
			"findings",
		},
		{"missing both required fields", map[string]interface{}{"root_cause": "Some cause"}, "required"},
		{"empty input", map[string]interface{}{}, "required"},
		{
			"only optional fields provided",
			map[string]interface{}{"root_cause": "Cause", "recommended_actions": []string{"Action"}},
			"required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			if err == nil {
				t.Error("Expected error for missing required fields, got nil")
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestCompleteInvestigationTool_InvalidFindingsType(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name     string
		findings interface{}
	}{
		{"findings as string", "single finding string"},
		{"findings as number", 123},
		{"findings as object", map[string]string{"key": "value"}},
		{"findings as null", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"confidence":       0.8,
				"findings":         tt.findings,
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid findings type %T, got nil", tt.findings)
			}
		})
	}
}

func TestCompleteInvestigationTool_OutputStructure(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id":    defaultTestInvestigationID,
		"confidence":          0.9,
		"findings":            []string{"Finding 1", "Finding 2"},
		"root_cause":          "Root cause identified",
		"recommended_actions": []string{"Action 1"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Errorf("Result should be valid JSON: %v", err)
	}

	if _, ok := output["status"]; !ok {
		t.Error("Output should contain 'status' field")
	}
}

// =============================================================================
// escalate_investigation Tool Tests
// =============================================================================

func TestEscalateInvestigationTool_Registration(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tools, err := h.adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tl := range tools {
		if tl.Name == "escalate_investigation" {
			found = true
			break
		}
	}

	if !found {
		t.Error("escalate_investigation tool should be registered")
	}
}

func TestEscalateInvestigationTool_SchemaReasonProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("escalate_investigation")
	if err != nil {
		t.Fatal(err)
	}

	reasonMap := h.assertPropertyExists(properties, "reason")
	h.assertPropertyType(reasonMap, "reason", "string")
}

func TestEscalateInvestigationTool_SchemaPriorityProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("escalate_investigation")
	if err != nil {
		t.Fatal(err)
	}

	priorityMap := h.assertPropertyExists(properties, "priority")
	if priorityMap == nil {
		return
	}

	h.assertPropertyType(priorityMap, "priority", "string")

	enumValues, ok := priorityMap["enum"].([]interface{})
	if !ok {
		t.Error("priority should have 'enum' constraint")
		return
	}

	expectedEnum := []string{"low", "medium", "high", "critical"}
	if len(enumValues) != len(expectedEnum) {
		t.Errorf("priority enum should have %d values, got %d", len(expectedEnum), len(enumValues))
	}
	for i, expected := range expectedEnum {
		if i < len(enumValues) && enumValues[i] != expected {
			t.Errorf("priority enum[%d] should be %q, got %v", i, expected, enumValues[i])
		}
	}
}

func TestEscalateInvestigationTool_SchemaPartialFindingsProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("escalate_investigation")
	if err != nil {
		t.Fatal(err)
	}

	partialFindingsMap := h.assertPropertyExists(properties, "partial_findings")
	h.assertPropertyType(partialFindingsMap, "partial_findings", "array")
}

func TestEscalateInvestigationTool_RequiredFields(t *testing.T) {
	h := newInvestigationTestHelper(t)

	escalateInvestigationTool, found := h.adapter.GetTool("escalate_investigation")
	if !found {
		t.Fatal("escalate_investigation tool should be registered")
	}

	requiredFields := escalateInvestigationTool.RequiredFields
	reasonRequired := false
	priorityRequired := false

	for _, field := range requiredFields {
		if field == "reason" {
			reasonRequired = true
		}
		if field == "priority" {
			priorityRequired = true
		}
	}

	if !reasonRequired {
		t.Error("'reason' should be a required field")
	}
	if !priorityRequired {
		t.Error("'priority' should be a required field")
	}
}

func TestEscalateInvestigationTool_ValidInput(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			name:  "minimal valid input - low priority",
			input: map[string]interface{}{"reason": "Need additional expertise", "priority": "low"},
		},
		{
			name:  "minimal valid input - medium priority",
			input: map[string]interface{}{"reason": "Requires database admin review", "priority": "medium"},
		},
		{
			name:  "minimal valid input - high priority",
			input: map[string]interface{}{"reason": "Production impacting issue", "priority": "high"},
		},
		{
			name:  "minimal valid input - critical priority",
			input: map[string]interface{}{"reason": "Complete system outage", "priority": "critical"},
		},
		{
			name: "full valid input with all fields",
			input: map[string]interface{}{
				"reason":           "Complex security incident requiring specialized team",
				"priority":         "critical",
				"partial_findings": []string{"Unauthorized access detected", "Multiple endpoints affected"},
			},
		},
		{
			name: "valid input with empty partial_findings",
			input: map[string]interface{}{
				"reason":           "Need human review",
				"priority":         "medium",
				"partial_findings": []string{},
			},
		},
		{
			name: "valid input with single partial finding",
			input: map[string]interface{}{
				"reason":           "Exceeded investigation scope",
				"priority":         "low",
				"partial_findings": []string{"Initial symptom identified"},
			},
		},
		{
			name: "valid input with multiple partial findings",
			input: map[string]interface{}{
				"reason":           "Multiple interconnected issues",
				"priority":         "high",
				"partial_findings": []string{"Issue 1", "Issue 2", "Issue 3", "Issue 4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register a fresh investigation for each subtest
			invID := defaultTestInvestigationID + "-esc-" + tt.name
			h.adapter.RegisterInvestigation(invID)
			tt.input["investigation_id"] = invID

			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			result, err := h.executeEscalateInvestigation(string(inputJSON))
			if err != nil {
				t.Errorf("ExecuteTool failed for valid input: %v", err)
			}

			if result == "" {
				t.Error("Expected non-empty result for valid input")
			}
		})
	}
}

func TestEscalateInvestigationTool_InvalidPriority(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name     string
		priority interface{}
		wantErr  string
	}{
		{"priority with wrong value - urgent", "urgent", "priority"},
		{"priority with wrong value - p1", "p1", "priority"},
		{"priority with wrong value - highest", "highest", "priority"},
		{"priority with wrong case - LOW", "LOW", "priority"},
		{"priority with wrong case - Medium", "Medium", "priority"},
		{"priority with wrong case - HIGH", "HIGH", "priority"},
		{"priority with wrong case - CRITICAL", "CRITICAL", "priority"},
		{"priority as number", 1, "priority"},
		{"priority as null", nil, "priority"},
		{"priority as empty string", "", "priority"},
		{"priority with whitespace", " high ", "priority"},
		{"priority with typo - hgih", "hgih", "priority"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"reason":           "Valid reason",
				"priority":         tt.priority,
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeEscalateInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid priority %v, got nil", tt.priority)
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestEscalateInvestigationTool_RequiredFieldsMissing(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			"missing reason",
			map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"priority":         "high",
			},
			"reason",
		},
		{
			"missing priority",
			map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"reason":           "Need escalation",
			},
			"priority",
		},
		{"missing both required fields", map[string]interface{}{"partial_findings": []string{"Finding"}}, "required"},
		{"empty input", map[string]interface{}{}, "required"},
		{
			"only optional field provided",
			map[string]interface{}{"partial_findings": []string{"Finding 1", "Finding 2"}},
			"required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeEscalateInvestigation(string(inputJSON))
			if err == nil {
				t.Error("Expected error for missing required fields, got nil")
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestEscalateInvestigationTool_InvalidReasonType(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name   string
		reason interface{}
	}{
		{"reason as number", 123},
		{"reason as boolean", true},
		{"reason as array", []string{"reason1", "reason2"}},
		{"reason as object", map[string]string{"key": "value"}},
		{"reason as null", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"reason":           tt.reason,
				"priority":         "high",
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeEscalateInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid reason type %T, got nil", tt.reason)
			}
		})
	}
}

func TestEscalateInvestigationTool_InvalidPartialFindingsType(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name            string
		partialFindings interface{}
	}{
		{"partial_findings as string", "single finding string"},
		{"partial_findings as number", 123},
		{"partial_findings as object", map[string]string{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": defaultTestInvestigationID,
				"reason":           "Valid reason",
				"priority":         "high",
				"partial_findings": tt.partialFindings,
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeEscalateInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid partial_findings type %T, got nil", tt.partialFindings)
			}
		})
	}
}

func TestEscalateInvestigationTool_OutputStructure(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id": defaultTestInvestigationID,
		"reason":           "Complex issue requiring human expertise",
		"priority":         "high",
		"partial_findings": []string{"Initial assessment complete"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Errorf("Result should be valid JSON: %v", err)
	}

	if _, ok := output["status"]; !ok {
		t.Error("Output should contain 'status' field")
	}
	if _, ok := output["escalation_id"]; !ok {
		t.Error("Output should contain 'escalation_id' field")
	}
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

func TestCompleteInvestigationTool_BoundaryConfidenceValues(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name       string
		confidence float64
		shouldPass bool
	}{
		{"exactly 0", 0.0, true},
		{"exactly 1", 1.0, true},
		{"just above 0", 0.001, true},
		{"just below 1", 0.999, true},
		{"middle value", 0.5, true},
		{"just below 0", -0.001, false},
		{"just above 1", 1.001, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register a fresh investigation for each subtest
			invID := defaultTestInvestigationID + "-boundary-" + tt.name
			h.adapter.RegisterInvestigation(invID)

			input := map[string]interface{}{
				"investigation_id": invID,
				"confidence":       tt.confidence,
				"findings":         []string{"Test finding"},
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			if tt.shouldPass && err != nil {
				t.Errorf("Expected success for confidence %v, got error: %v", tt.confidence, err)
			}
			if !tt.shouldPass && err == nil {
				t.Errorf("Expected error for confidence %v, got nil", tt.confidence)
			}
		})
	}
}

func TestCompleteInvestigationTool_LargeFindingsArray(t *testing.T) {
	h := newInvestigationTestHelper(t)

	findings := make([]string, 100)
	for i := range 100 {
		findings[i] = fmt.Sprintf("Finding number %d with detailed description", i+1)
	}

	// Register a fresh investigation for this test
	invID := defaultTestInvestigationID + "-large-findings"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.75,
		"findings":         findings,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Errorf("Expected success for large findings array, got error: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result for large findings array")
	}
}

func TestEscalateInvestigationTool_LongReasonString(t *testing.T) {
	h := newInvestigationTestHelper(t)

	longReason := strings.Repeat("This is a detailed reason for escalation. ", 100)

	// Register a fresh investigation for this test
	invID := defaultTestInvestigationID + "-long-reason"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"reason":           longReason,
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Errorf("Expected success for long reason string, got error: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result for long reason string")
	}
}

func TestCompleteInvestigationTool_UnicodeContent(t *testing.T) {
	h := newInvestigationTestHelper(t)

	// Register a fresh investigation for this test
	invID := defaultTestInvestigationID + "-unicode-complete"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id":    invID,
		"confidence":          0.85,
		"findings":            []string{"Unicode finding", "Japanese text", "Emoji finding"},
		"root_cause":          "Root cause with unicode",
		"recommended_actions": []string{"Action with special chars: @#$%"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Errorf("Expected success for unicode content, got error: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result for unicode content")
	}
}

func TestEscalateInvestigationTool_UnicodeContent(t *testing.T) {
	h := newInvestigationTestHelper(t)

	// Register a fresh investigation for this test
	invID := defaultTestInvestigationID + "-unicode-escalate"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"reason":           "Escalation reason with unicode characters",
		"priority":         "critical",
		"partial_findings": []string{"Finding with special chars: @#$%^&*()"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Errorf("Expected success for unicode content, got error: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result for unicode content")
	}
}

func TestInvestigationTools_MalformedJSONInput(t *testing.T) {
	h := newInvestigationTestHelper(t)

	malformedInputs := []string{
		`{"confidence": 0.5, "findings": [}`,
		`{confidence: 0.5}`,
		`"not an object"`,
		`null`,
		``,
		`{"unclosed": "brace"`,
	}

	for _, input := range malformedInputs {
		t.Run(fmt.Sprintf("complete_investigation with %q", input), func(t *testing.T) {
			_, err := h.executeCompleteInvestigation(input)
			if err == nil {
				t.Errorf("Expected error for malformed input %q, got nil", input)
			}
		})

		t.Run(fmt.Sprintf("escalate_investigation with %q", input), func(t *testing.T) {
			_, err := h.executeEscalateInvestigation(input)
			if err == nil {
				t.Errorf("Expected error for malformed input %q, got nil", input)
			}
		})
	}
}

func TestCompleteInvestigationTool_EmptyFindingsArray(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"confidence": 0.5,
		"findings":   []string{},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeCompleteInvestigation(string(inputJSON))
	t.Logf("Empty findings array result: err=%v", err)
}

func TestEscalateInvestigationTool_EmptyReasonString(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"reason":   "",
		"priority": "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeEscalateInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error for empty reason string, got nil")
	}
}

func TestCompleteInvestigationTool_ToolDescription(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tl, found := h.adapter.GetTool("complete_investigation")
	if !found {
		t.Fatal("complete_investigation tool should be registered")
	}

	if tl.Description == "" {
		t.Error("Tool should have a non-empty description")
	}

	desc := strings.ToLower(tl.Description)
	if !strings.Contains(desc, "investigation") && !strings.Contains(desc, "complete") {
		t.Error("Description should mention investigation or complete")
	}
}

func TestEscalateInvestigationTool_ToolDescription(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tl, found := h.adapter.GetTool("escalate_investigation")
	if !found {
		t.Fatal("escalate_investigation tool should be registered")
	}

	if tl.Description == "" {
		t.Error("Tool should have a non-empty description")
	}

	desc := strings.ToLower(tl.Description)
	if !strings.Contains(desc, "escalat") {
		t.Error("Description should mention escalation")
	}
}

// =============================================================================
// Investigation Context Tests
// These tests verify that investigation tools properly handle investigation context
// (investigation_id) which is needed to correlate tool results with investigations.
// =============================================================================

func TestCompleteInvestigationTool_SchemaInvestigationIDProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	invIDMap := h.assertPropertyExists(properties, "investigation_id")
	h.assertPropertyType(invIDMap, "investigation_id", "string")
}

func TestEscalateInvestigationTool_SchemaInvestigationIDProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("escalate_investigation")
	if err != nil {
		t.Fatal(err)
	}

	invIDMap := h.assertPropertyExists(properties, "investigation_id")
	h.assertPropertyType(invIDMap, "investigation_id", "string")
}

func TestCompleteInvestigationTool_InvestigationIDRequired(t *testing.T) {
	h := newInvestigationTestHelper(t)

	completeInvestigationTool, found := h.adapter.GetTool("complete_investigation")
	if !found {
		t.Fatal("complete_investigation tool should be registered")
	}

	invIDRequired := false
	for _, field := range completeInvestigationTool.RequiredFields {
		if field == "investigation_id" {
			invIDRequired = true
			break
		}
	}

	if !invIDRequired {
		t.Error("'investigation_id' should be a required field")
	}
}

func TestEscalateInvestigationTool_InvestigationIDRequired(t *testing.T) {
	h := newInvestigationTestHelper(t)

	escalateInvestigationTool, found := h.adapter.GetTool("escalate_investigation")
	if !found {
		t.Fatal("escalate_investigation tool should be registered")
	}

	invIDRequired := false
	for _, field := range escalateInvestigationTool.RequiredFields {
		if field == "investigation_id" {
			invIDRequired = true
			break
		}
	}

	if !invIDRequired {
		t.Error("'investigation_id' should be a required field")
	}
}

func TestCompleteInvestigationTool_InvalidInvestigationID(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name            string
		investigationID interface{}
		wantErr         string
	}{
		{"empty investigation_id", "", "investigation_id"},
		{"investigation_id with only whitespace", "   ", "investigation_id"},
		{"investigation_id as number", 12345, "investigation_id"},
		{"investigation_id as null", nil, "investigation_id"},
		{"investigation_id as array", []string{"id1"}, "investigation_id"},
		{"investigation_id as object", map[string]string{"id": "value"}, "investigation_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": tt.investigationID,
				"confidence":       0.8,
				"findings":         []string{"Finding 1"},
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid investigation_id %v, got nil", tt.investigationID)
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestEscalateInvestigationTool_InvalidInvestigationID(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name            string
		investigationID interface{}
		wantErr         string
	}{
		{"empty investigation_id", "", "investigation_id"},
		{"investigation_id with only whitespace", "   ", "investigation_id"},
		{"investigation_id as number", 12345, "investigation_id"},
		{"investigation_id as null", nil, "investigation_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": tt.investigationID,
				"reason":           "Valid reason",
				"priority":         "high",
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.executeEscalateInvestigation(string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid investigation_id %v, got nil", tt.investigationID)
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestCompleteInvestigationTool_NonExistentInvestigationID(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id": "non-existent-investigation-id-12345",
		"confidence":       0.9,
		"findings":         []string{"Finding 1"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeCompleteInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error for non-existent investigation_id, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("Expected error to contain 'not found', got: %v", err)
	}
}

func TestEscalateInvestigationTool_NonExistentInvestigationID(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id": "non-existent-investigation-id-67890",
		"reason":           "Valid reason",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeEscalateInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error for non-existent investigation_id, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("Expected error to contain 'not found', got: %v", err)
	}
}

// =============================================================================
// Output Structure and Content Tests
// These tests verify the structure and content of tool outputs.
// =============================================================================

func TestCompleteInvestigationTool_OutputContainsInvestigationID(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-123"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.9,
		"findings":         []string{"Finding 1"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if output["investigation_id"] != "test-inv-123" {
		t.Errorf("Output investigation_id should match input, got: %v", output["investigation_id"])
	}
}

func TestCompleteInvestigationTool_OutputContainsTimestamp(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-456"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.85,
		"findings":         []string{"Finding 1"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if _, ok := output["completed_at"]; !ok {
		t.Error("Output should contain 'completed_at' timestamp field")
	}
}

func TestEscalateInvestigationTool_OutputContainsTimestamp(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-789"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"reason":           "Need expert review",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if _, ok := output["escalated_at"]; !ok {
		t.Error("Output should contain 'escalated_at' timestamp field")
	}
}

func TestEscalateInvestigationTool_OutputContainsPriority(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-priority"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"reason":           "Critical issue",
		"priority":         "critical",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if output["priority"] != "critical" {
		t.Errorf("Output priority should match input, got: %v", output["priority"])
	}
}

func TestCompleteInvestigationTool_OutputContainsAllFindings(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-findings"
	h.adapter.RegisterInvestigation(invID)

	inputFindings := []string{"Finding 1", "Finding 2", "Finding 3"}
	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.9,
		"findings":         inputFindings,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	outputFindings, ok := output["findings"].([]interface{})
	if !ok {
		t.Fatal("Output findings should be an array")
	}

	if len(outputFindings) != len(inputFindings) {
		t.Errorf("Output findings count %d should match input count %d", len(outputFindings), len(inputFindings))
	}
}

// =============================================================================
// Schema Additional Properties Tests
// These tests verify that the tool schemas properly handle additional/unknown properties.
// =============================================================================

func TestCompleteInvestigationTool_SchemaHasSeverityProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	severityMap := h.assertPropertyExists(properties, "severity")
	if severityMap == nil {
		return
	}

	h.assertPropertyType(severityMap, "severity", "string")

	enumValues, ok := severityMap["enum"].([]interface{})
	if !ok {
		t.Error("severity should have 'enum' constraint")
		return
	}

	expectedEnum := []string{"info", "warning", "error", "critical"}
	if len(enumValues) != len(expectedEnum) {
		t.Errorf("severity enum should have %d values, got %d", len(expectedEnum), len(enumValues))
	}
}

func TestCompleteInvestigationTool_SchemaSummaryProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("complete_investigation")
	if err != nil {
		t.Fatal(err)
	}

	summaryMap := h.assertPropertyExists(properties, "summary")
	h.assertPropertyType(summaryMap, "summary", "string")
}

func TestEscalateInvestigationTool_SchemaBlockingProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("escalate_investigation")
	if err != nil {
		t.Fatal(err)
	}

	blockingMap := h.assertPropertyExists(properties, "blocking")
	h.assertPropertyType(blockingMap, "blocking", "boolean")
}

func TestEscalateInvestigationTool_SchemaRequiresAckProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("escalate_investigation")
	if err != nil {
		t.Fatal(err)
	}

	requiresAckMap := h.assertPropertyExists(properties, "requires_acknowledgment")
	h.assertPropertyType(requiresAckMap, "requires_acknowledgment", "boolean")
}

// =============================================================================
// State Transition Tests
// These tests verify that investigation tools properly trigger state transitions.
// =============================================================================

func TestCompleteInvestigationTool_TransitionsToCompletedStatus(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-status"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.95,
		"findings":         []string{"Root cause identified"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if output["status"] != "completed" {
		t.Errorf("Output status should be 'completed', got: %v", output["status"])
	}
}

func TestEscalateInvestigationTool_TransitionsToEscalatedStatus(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-escalate-status"
	h.adapter.RegisterInvestigation(invID)

	input := map[string]interface{}{
		"investigation_id": invID,
		"reason":           "Need human intervention",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if output["status"] != "escalated" {
		t.Errorf("Output status should be 'escalated', got: %v", output["status"])
	}
}

func TestCompleteInvestigationTool_RejectsAlreadyCompletedInvestigation(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-double-complete"
	h.adapter.RegisterInvestigation(invID)

	// First completion should succeed
	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.9,
		"findings":         []string{"Initial finding"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("First completion failed: %v", err)
	}

	// Second completion should fail
	input["findings"] = []string{"Updated finding"}
	inputJSON, err = json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeCompleteInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error when completing already-completed investigation, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		t.Errorf("Expected error to mention 'already' completed, got: %v", err)
	}
}

func TestEscalateInvestigationTool_RejectsAlreadyEscalatedInvestigation(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-double-escalate"
	h.adapter.RegisterInvestigation(invID)

	// First escalation should succeed
	input := map[string]interface{}{
		"investigation_id": invID,
		"reason":           "Initial escalation",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeEscalateInvestigation(string(inputJSON))
	if err != nil {
		t.Fatalf("First escalation failed: %v", err)
	}

	// Second escalation should fail
	input["reason"] = "Second escalation attempt"
	inputJSON, err = json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeEscalateInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error when escalating already-escalated investigation, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		t.Errorf("Expected error to mention 'already' escalated, got: %v", err)
	}
}

// =============================================================================
// Additional Input Validation Tests
// =============================================================================

func TestCompleteInvestigationTool_RejectsEmptyFindingsArray(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id": "test-inv-empty-findings",
		"confidence":       0.5,
		"findings":         []string{},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeCompleteInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error for empty findings array, got nil")
	}
}

func TestCompleteInvestigationTool_ValidatesMinFindingsCount(t *testing.T) {
	h := newInvestigationTestHelper(t)

	invID := "test-inv-min-findings"
	h.adapter.RegisterInvestigation(invID)

	// At least one finding should be required for a valid completion
	input := map[string]interface{}{
		"investigation_id": invID,
		"confidence":       0.8,
		"findings":         []string{"At least one finding"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeCompleteInvestigation(string(inputJSON))
	if err != nil {
		t.Errorf("Expected success with at least one finding, got error: %v", err)
	}
}

func TestEscalateInvestigationTool_RejectsEmptyReason(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id": "test-inv-empty-reason",
		"reason":           "",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeEscalateInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error for empty reason, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "reason") {
		t.Errorf("Expected error to mention 'reason', got: %v", err)
	}
}

func TestEscalateInvestigationTool_RejectsWhitespaceOnlyReason(t *testing.T) {
	h := newInvestigationTestHelper(t)

	input := map[string]interface{}{
		"investigation_id": "test-inv-whitespace-reason",
		"reason":           "   \t\n  ",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.executeEscalateInvestigation(string(inputJSON))
	if err == nil {
		t.Error("Expected error for whitespace-only reason, got nil")
	}
}

// =============================================================================
// Concurrency Tests
// These tests verify that investigation tools handle concurrent access correctly.
// =============================================================================

func TestCompleteInvestigationTool_ConcurrentCompletions(t *testing.T) {
	h := newInvestigationTestHelper(t)

	numGoroutines := 10
	errChan := make(chan error, numGoroutines)

	// Pre-register all investigations
	for i := range numGoroutines {
		h.adapter.RegisterInvestigation(fmt.Sprintf("concurrent-inv-%d", i))
	}

	for i := range numGoroutines {
		go func(idx int) {
			input := map[string]interface{}{
				"investigation_id": fmt.Sprintf("concurrent-inv-%d", idx),
				"confidence":       0.8,
				"findings":         []string{fmt.Sprintf("Finding from goroutine %d", idx)},
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				errChan <- fmt.Errorf("goroutine %d: marshal error: %w", idx, err)
				return
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			if err != nil {
				errChan <- fmt.Errorf("goroutine %d: execution error: %w", idx, err)
				return
			}
			errChan <- nil
		}(i)
	}

	successCount := 0
	for range numGoroutines {
		if err := <-errChan; err == nil {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected all %d concurrent completions to succeed, only %d succeeded", numGoroutines, successCount)
	}
}

func TestEscalateInvestigationTool_ConcurrentEscalations(t *testing.T) {
	h := newInvestigationTestHelper(t)

	numGoroutines := 10
	errChan := make(chan error, numGoroutines)

	// Pre-register all investigations
	for i := range numGoroutines {
		h.adapter.RegisterInvestigation(fmt.Sprintf("concurrent-esc-%d", i))
	}

	for i := range numGoroutines {
		go func(idx int) {
			input := map[string]interface{}{
				"investigation_id": fmt.Sprintf("concurrent-esc-%d", idx),
				"reason":           fmt.Sprintf("Escalation from goroutine %d", idx),
				"priority":         "high",
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				errChan <- fmt.Errorf("goroutine %d: marshal error: %w", idx, err)
				return
			}

			_, err = h.executeEscalateInvestigation(string(inputJSON))
			if err != nil {
				errChan <- fmt.Errorf("goroutine %d: execution error: %w", idx, err)
				return
			}
			errChan <- nil
		}(i)
	}

	successCount := 0
	for range numGoroutines {
		if err := <-errChan; err == nil {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected all %d concurrent escalations to succeed, only %d succeeded", numGoroutines, successCount)
	}
}

func TestCompleteInvestigationTool_RaceConditionOnSameInvestigation(t *testing.T) {
	h := newInvestigationTestHelper(t)

	numGoroutines := 5
	investigationID := "race-condition-test-inv"
	h.adapter.RegisterInvestigation(investigationID)

	resultChan := make(chan struct {
		success bool
		err     error
	}, numGoroutines)

	for i := range numGoroutines {
		go func(idx int) {
			input := map[string]interface{}{
				"investigation_id": investigationID,
				"confidence":       float64(idx) / 10.0,
				"findings":         []string{fmt.Sprintf("Finding %d", idx)},
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				resultChan <- struct {
					success bool
					err     error
				}{false, err}
				return
			}

			_, err = h.executeCompleteInvestigation(string(inputJSON))
			resultChan <- struct {
				success bool
				err     error
			}{err == nil, err}
		}(i)
	}

	successCount := 0
	for range numGoroutines {
		result := <-resultChan
		if result.success {
			successCount++
		}
	}

	// Only one completion should succeed for the same investigation
	if successCount != 1 {
		t.Errorf("Expected exactly 1 completion to succeed for same investigation, got %d", successCount)
	}
}

// =============================================================================
// Context Cancellation Tests
// These tests verify that investigation tools respect context cancellation.
// =============================================================================

func TestCompleteInvestigationTool_RespectsContextCancellation(t *testing.T) {
	h := newInvestigationTestHelper(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := map[string]interface{}{
		"investigation_id": "test-inv-cancelled",
		"confidence":       0.9,
		"findings":         []string{"Finding 1"},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.adapter.ExecuteTool(ctx, "complete_investigation", string(inputJSON))
	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "cancel") && !strings.Contains(err.Error(), "context") {
		// Check if it's a context error by testing for specific context errors
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Logf("Got error: %v (may be acceptable if tool not implemented)", err)
		}
	}
}

func TestEscalateInvestigationTool_RespectsContextCancellation(t *testing.T) {
	h := newInvestigationTestHelper(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := map[string]interface{}{
		"investigation_id": "test-inv-cancelled-esc",
		"reason":           "Test reason",
		"priority":         "high",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = h.adapter.ExecuteTool(ctx, "escalate_investigation", string(inputJSON))
	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "cancel") && !strings.Contains(err.Error(), "context") {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Logf("Got error: %v (may be acceptable if tool not implemented)", err)
		}
	}
}

// =============================================================================
// Report Investigation Tool Tests
// This tool allows AI to provide status updates during ongoing investigations.
// =============================================================================

func TestReportInvestigationTool_Registration(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tools, err := h.adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tl := range tools {
		if tl.Name == "report_investigation" {
			found = true
			break
		}
	}

	if !found {
		t.Error("report_investigation tool should be registered")
	}
}

func TestReportInvestigationTool_SchemaMessageProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("report_investigation")
	if err != nil {
		t.Fatal(err)
	}

	messageMap := h.assertPropertyExists(properties, "message")
	h.assertPropertyType(messageMap, "message", "string")
}

func TestReportInvestigationTool_SchemaProgressProperty(t *testing.T) {
	h := newInvestigationTestHelper(t)

	properties, err := h.getToolProperties("report_investigation")
	if err != nil {
		t.Fatal(err)
	}

	progressMap := h.assertPropertyExists(properties, "progress")
	if progressMap == nil {
		return
	}

	h.assertPropertyType(progressMap, "progress", "number")

	if progressMap["minimum"] != float64(0) {
		t.Errorf("progress minimum should be 0, got %v", progressMap["minimum"])
	}
	if progressMap["maximum"] != float64(100) {
		t.Errorf("progress maximum should be 100, got %v", progressMap["maximum"])
	}
}

func TestReportInvestigationTool_RequiredFields(t *testing.T) {
	h := newInvestigationTestHelper(t)

	reportInvestigationTool, found := h.adapter.GetTool("report_investigation")
	if !found {
		t.Fatal("report_investigation tool should be registered")
	}

	requiredFields := reportInvestigationTool.RequiredFields
	invIDRequired := false
	messageRequired := false

	for _, field := range requiredFields {
		if field == "investigation_id" {
			invIDRequired = true
		}
		if field == "message" {
			messageRequired = true
		}
	}

	if !invIDRequired {
		t.Error("'investigation_id' should be a required field")
	}
	if !messageRequired {
		t.Error("'message' should be a required field")
	}
}

func TestReportInvestigationTool_ValidInput(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			name: "minimal valid input",
			input: map[string]interface{}{
				"investigation_id": "test-inv-report",
				"message":          "Currently analyzing logs...",
			},
		},
		{
			name: "with progress percentage",
			input: map[string]interface{}{
				"investigation_id": "test-inv-report-progress",
				"message":          "Halfway through analysis",
				"progress":         50,
			},
		},
		{
			name: "with progress at 0%",
			input: map[string]interface{}{
				"investigation_id": "test-inv-report-start",
				"message":          "Starting investigation",
				"progress":         0,
			},
		},
		{
			name: "with progress at 100%",
			input: map[string]interface{}{
				"investigation_id": "test-inv-report-end",
				"message":          "Almost complete",
				"progress":         100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			result, err := h.adapter.ExecuteTool(context.Background(), "report_investigation", string(inputJSON))
			if err != nil {
				t.Errorf("ExecuteTool failed for valid input: %v", err)
			}

			if result == "" {
				t.Error("Expected non-empty result for valid input")
			}
		})
	}
}

func TestReportInvestigationTool_InvalidProgress(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tests := []struct {
		name     string
		progress interface{}
		wantErr  string
	}{
		{"progress below minimum", -1, "progress"},
		{"progress above maximum", 101, "progress"},
		{"progress way below minimum", -100, "progress"},
		{"progress way above maximum", 1000, "progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"investigation_id": "test-inv-bad-progress",
				"message":          "Test message",
				"progress":         tt.progress,
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, err = h.adapter.ExecuteTool(context.Background(), "report_investigation", string(inputJSON))
			if err == nil {
				t.Errorf("Expected error for invalid progress %v, got nil", tt.progress)
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestReportInvestigationTool_ToolDescription(t *testing.T) {
	h := newInvestigationTestHelper(t)

	tl, found := h.adapter.GetTool("report_investigation")
	if !found {
		t.Fatal("report_investigation tool should be registered")
	}

	if tl.Description == "" {
		t.Error("Tool should have a non-empty description")
	}

	desc := strings.ToLower(tl.Description)
	if !strings.Contains(desc, "report") && !strings.Contains(desc, "progress") && !strings.Contains(desc, "status") {
		t.Error("Description should mention report, progress, or status")
	}
}
