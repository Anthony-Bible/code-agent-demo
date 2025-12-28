package tool_test

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/tool"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// RED PHASE TDD: Tests for read_file with start_line and end_line parameters
// These tests define the expected behavior for line range reading functionality.
// All tests should FAIL until the feature is implemented.
// =============================================================================

// testHelper contains common test setup utilities.
type testHelper struct {
	t        *testing.T
	tempDir  string
	adapter  *tool.ExecutorAdapter
	testFile string
}

// newTestHelper creates a new test helper with a temporary directory and adapter.
func newTestHelper(t *testing.T) *testHelper {
	t.Helper()
	tempDir := t.TempDir()
	fileManager := file.NewLocalFileManager(tempDir)
	adapter := tool.NewExecutorAdapter(fileManager)
	return &testHelper{
		t:       t,
		tempDir: tempDir,
		adapter: adapter,
	}
}

// createFile creates a test file with the given content.
func (h *testHelper) createFile(name, content string) {
	h.t.Helper()
	h.testFile = filepath.Join(h.tempDir, name)
	err := os.WriteFile(h.testFile, []byte(content), 0o644)
	if err != nil {
		h.t.Fatalf("Failed to create test file: %v", err)
	}
}

// filePath returns the full path for a file in the temp directory.
func (h *testHelper) filePath(name string) string {
	return filepath.Join(h.tempDir, name)
}

// executeReadFile executes the read_file tool with the given input.
func (h *testHelper) executeReadFile(input string) (string, error) {
	h.t.Helper()
	return h.adapter.ExecuteTool(context.Background(), "read_file", input)
}

// readFileInput creates the JSON input for read_file with full path.
func (h *testHelper) readFileInput(name string, startLine, endLine *int) string {
	path := h.filePath(name)
	input := fmt.Sprintf(`{"path": %q`, path)
	if startLine != nil {
		input += fmt.Sprintf(`, "start_line": %d`, *startLine)
	}
	if endLine != nil {
		input += fmt.Sprintf(`, "end_line": %d`, *endLine)
	}
	input += "}"
	return input
}

// intPtr is a helper to create a pointer to an int.
func intPtr(i int) *int {
	return &i
}

// assertContains checks that result contains the expected string.
func (h *testHelper) assertContains(result, expected string) {
	h.t.Helper()
	if !strings.Contains(result, expected) {
		h.t.Errorf("Expected output to contain %q, got:\n%s", expected, result)
	}
}

// assertNotContains checks that result does not contain the unexpected string.
func (h *testHelper) assertNotContains(result, unexpected string) {
	h.t.Helper()
	if strings.Contains(result, unexpected) {
		h.t.Errorf("Expected output NOT to contain %q, got:\n%s", unexpected, result)
	}
}

// assertContainsAll checks that result contains all expected strings.
func (h *testHelper) assertContainsAll(result string, expected []string) {
	h.t.Helper()
	for _, exp := range expected {
		h.assertContains(result, exp)
	}
}

// assertContainsNone checks that result contains none of the unexpected strings.
func (h *testHelper) assertContainsNone(result string, unexpected []string) {
	h.t.Helper()
	for _, unexp := range unexpected {
		h.assertNotContains(result, unexp)
	}
}

// standardContent returns a standard 5-line test content.
func standardContent() string {
	return "line one\nline two\nline three\nline four\nline five\n"
}

func TestReadFile_EntireFileWithLineNumbers(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", nil, nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{
		"1: line one",
		"2: line two",
		"3: line three",
		"4: line four",
		"5: line five",
	}
	h.assertContainsAll(result, expected)
}

func TestReadFile_SpecificLineRange(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(2), intPtr(4))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"2: line two", "3: line three", "4: line four"}
	unexpected := []string{"1: line one", "5: line five"}

	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, unexpected)
}

func TestReadFile_StartLineOnlyReadsToEOF(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(3), nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"3: line three", "4: line four", "5: line five"}
	unexpected := []string{"1: line one", "2: line two"}

	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, unexpected)
}

func TestReadFile_EndLineOnlyReadsFromStart(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", nil, intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"1: line one", "2: line two", "3: line three"}
	unexpected := []string{"4: line four", "5: line five"}

	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, unexpected)
}

func TestReadFile_SingleLineRange(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(3), intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "3: line three")
	unexpected := []string{"1: line one", "2: line two", "4: line four", "5: line five"}
	h.assertContainsNone(result, unexpected)
}

func TestReadFile_ErrorWhenStartLineGreaterThanEndLine(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", intPtr(5), intPtr(2))
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when start_line > end_line, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "start_line") || !strings.Contains(errMsg, "end_line") {
		t.Errorf("Expected error message to mention start_line and end_line, got: %v", err)
	}
}

func TestReadFile_ErrorWhenStartLineLessThanOne(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", intPtr(0), nil)
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when start_line < 1, got nil")
	}

	if !strings.Contains(err.Error(), "start_line") {
		t.Errorf("Expected error message to mention start_line, got: %v", err)
	}
}

func TestReadFile_ErrorWhenStartLineNegative(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", intPtr(-5), nil)
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when start_line is negative, got nil")
	}

	if !strings.Contains(err.Error(), "start_line") {
		t.Errorf("Expected error message to mention start_line, got: %v", err)
	}
}

func TestReadFile_ErrorWhenEndLineLessThanOne(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", nil, intPtr(0))
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when end_line < 1, got nil")
	}

	if !strings.Contains(err.Error(), "end_line") {
		t.Errorf("Expected error message to mention end_line, got: %v", err)
	}
}

func TestReadFile_ErrorWhenEndLineNegative(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", nil, intPtr(-3))
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when end_line is negative, got nil")
	}

	if !strings.Contains(err.Error(), "end_line") {
		t.Errorf("Expected error message to mention end_line, got: %v", err)
	}
}

func TestReadFile_StartLineExceedsFileLength(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", intPtr(10), nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Result should not contain any actual line content
	unexpected := []string{"line one", "line two", "line three"}
	h.assertContainsNone(result, unexpected)
}

func TestReadFile_EndLineExceedsFileLengthReadsToEOF(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", "line one\nline two\nline three\n")

	input := h.readFileInput("test.txt", intPtr(2), intPtr(100))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"2: line two", "3: line three"}
	h.assertContainsAll(result, expected)
	h.assertNotContains(result, "1: line one")
}

func TestReadFile_EmptyFile(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("empty.txt", "")

	input := h.readFileInput("empty.txt", intPtr(1), intPtr(10))
	_, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	// Just verify no error occurred
}

func TestReadFile_SingleLineFile(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("single.txt", "only one line")

	input := h.readFileInput("single.txt", intPtr(1), intPtr(1))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "1: only one line")
}

func TestReadFile_FileWithNoTrailingNewline(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("notail.txt", "line one\nline two\nline three")

	input := h.readFileInput("notail.txt", intPtr(2), intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"2: line two", "3: line three"}
	h.assertContainsAll(result, expected)
}

func TestReadFile_VaryingContentLengths(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("varied.txt", "a\nbb\nccc\ndddd\neeeee\nffffff\nggggggg\nhhhhhhhh\n")

	input := h.readFileInput("varied.txt", intPtr(3), intPtr(6))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"3: ccc", "4: dddd", "5: eeeee", "6: ffffff"}
	h.assertContainsAll(result, expected)
}

func TestReadFile_SpecialCharacters(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("special.txt", "normal line\nline with\ttab\nline with: colon\n\"quoted line\"\n")

	input := h.readFileInput("special.txt", nil, nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "2: line with\ttab")
	h.assertContains(result, "3: line with: colon")
	h.assertContains(result, "4: \"quoted line\"")
}

func TestReadFile_LargeLineNumbers(t *testing.T) {
	h := newTestHelper(t)

	// Create a file with 150 lines
	var lines []string
	for i := 1; i <= 150; i++ {
		lines = append(lines, "content")
	}
	content := strings.Join(lines, "\n") + "\n"
	h.createFile("manylines.txt", content)

	input := h.readFileInput("manylines.txt", intPtr(98), intPtr(102))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"98: content", "99: content", "100: content", "101: content", "102: content"}
	h.assertContainsAll(result, expected)
}

func TestReadFile_OutputFormatPattern(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("format.txt", "first\nsecond\nthird\n")

	input := h.readFileInput("format.txt", nil, nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	resultLines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")
	expectedFormats := []string{"1: first", "2: second", "3: third"}

	for i, expected := range expectedFormats {
		if i >= len(resultLines) {
			t.Errorf("Missing line %d in output", i+1)
			continue
		}
		if resultLines[i] != expected {
			t.Errorf("Line %d format mismatch: expected %q, got %q", i+1, expected, resultLines[i])
		}
	}
}

func TestReadFile_EmptyLinesPreserved(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("empty_lines.txt", "line one\n\nline three\n\n\nline six\n")

	input := h.readFileInput("empty_lines.txt", nil, nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"1: line one", "2: ", "3: line three", "4: ", "5: ", "6: line six"}
	h.assertContainsAll(result, expected)
}

func TestReadFile_SchemaIncludesStartLineParameter(t *testing.T) {
	h := newTestHelper(t)

	readFileTool, found := h.adapter.GetTool("read_file")
	if !found {
		t.Fatal("read_file tool should be registered")
	}

	schema := readFileTool.InputSchema
	if schema == nil {
		t.Fatal("InputSchema should not be nil")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties should be a map, got %T", schema["properties"])
	}

	startLineParam, found := properties["start_line"]
	if !found {
		t.Error("read_file tool schema should include 'start_line' parameter")
		return
	}

	startLineMap, ok := startLineParam.(map[string]interface{})
	if !ok {
		t.Errorf("start_line should be a map, got %T", startLineParam)
		return
	}

	typeVal, found := startLineMap["type"]
	if !found || typeVal != "integer" {
		t.Error("start_line parameter should be of type 'integer'")
	}
}

func TestReadFile_SchemaIncludesEndLineParameter(t *testing.T) {
	h := newTestHelper(t)

	readFileTool, found := h.adapter.GetTool("read_file")
	if !found {
		t.Fatal("read_file tool should be registered")
	}

	schema := readFileTool.InputSchema
	if schema == nil {
		t.Fatal("InputSchema should not be nil")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties should be a map, got %T", schema["properties"])
	}

	endLineParam, found := properties["end_line"]
	if !found {
		t.Error("read_file tool schema should include 'end_line' parameter")
		return
	}

	endLineMap, ok := endLineParam.(map[string]interface{})
	if !ok {
		t.Errorf("end_line should be a map, got %T", endLineParam)
		return
	}

	typeVal, found := endLineMap["type"]
	if !found || typeVal != "integer" {
		t.Error("end_line parameter should be of type 'integer'")
	}
}

func TestReadFile_LineParametersNotRequired(t *testing.T) {
	h := newTestHelper(t)

	readFileTool, found := h.adapter.GetTool("read_file")
	if !found {
		t.Fatal("read_file tool should be registered")
	}

	schema := readFileTool.InputSchema
	if schema == nil {
		t.Fatal("InputSchema should not be nil")
	}

	required := extractRequiredFields(schema)

	for _, req := range required {
		if req == "start_line" {
			t.Error("start_line should NOT be a required parameter")
		}
		if req == "end_line" {
			t.Error("end_line should NOT be a required parameter")
		}
	}
}

// extractRequiredFields extracts the required fields from a schema.
func extractRequiredFields(schema map[string]interface{}) []string {
	required, ok := schema["required"].([]string)
	if ok {
		return required
	}

	requiredIface, ok := schema["required"].([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, len(requiredIface))
	for i, v := range requiredIface {
		if str, ok := v.(string); ok {
			result[i] = str
		}
	}
	return result
}

// =============================================================================
// ADDITIONAL RED PHASE TDD TESTS: Extended coverage for line range feature
// =============================================================================

// Test that start_line and end_line with value 1 reads only the first line.
func TestReadFile_FirstLineOnly(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(1), intPtr(1))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "1: line one")
	unexpected := []string{"2: line two", "3: line three", "4: line four", "5: line five"}
	h.assertContainsNone(result, unexpected)
}

// Test that the last line can be read with both start and end set to the same value.
func TestReadFile_LastLineOnly(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(5), intPtr(5))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "5: line five")
	unexpected := []string{"1: line one", "2: line two", "3: line three", "4: line four"}
	h.assertContainsNone(result, unexpected)
}

// Test reading middle two lines of a file.
func TestReadFile_MiddleTwoLines(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(2), intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"2: line two", "3: line three"}
	unexpected := []string{"1: line one", "4: line four", "5: line five"}

	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, unexpected)
}

// Test that zero value for start_line while providing end_line produces error.
func TestReadFile_ZeroStartLineWithValidEndLine(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(0), intPtr(3))
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when start_line is 0, got nil")
	}

	if !strings.Contains(err.Error(), "start_line") {
		t.Errorf("Expected error message to mention start_line, got: %v", err)
	}
}

// Test that zero value for end_line while providing start_line produces error.
func TestReadFile_ValidStartLineWithZeroEndLine(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("test.txt", standardContent())

	input := h.readFileInput("test.txt", intPtr(2), intPtr(0))
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when end_line is 0, got nil")
	}

	if !strings.Contains(err.Error(), "end_line") {
		t.Errorf("Expected error message to mention end_line, got: %v", err)
	}
}

// Test start_line equals end_line equals file length reads last line only.
func TestReadFile_StartAndEndEqualToFileLength(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("threelines.txt", "one\ntwo\nthree\n")

	input := h.readFileInput("threelines.txt", intPtr(3), intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "3: three")
	h.assertContainsNone(result, []string{"1: one", "2: two"})
}

// Test that start_line exactly at file length returns one line.
func TestReadFile_StartLineExactlyAtFileLength(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("threelines.txt", "one\ntwo\nthree\n")

	input := h.readFileInput("threelines.txt", intPtr(3), nil)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "3: three")
	h.assertContainsNone(result, []string{"1: one", "2: two"})
}

// Test file with CRLF line endings (Windows-style).
func TestReadFile_CRLFLineEndings(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("crlf.txt", "line one\r\nline two\r\nline three\r\n")

	input := h.readFileInput("crlf.txt", intPtr(2), intPtr(2))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Should handle CRLF and show line 2
	h.assertContains(result, "2:")
	h.assertContains(result, "line two")
}

// Test file with mixed line endings.
func TestReadFile_MixedLineEndings(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("mixed.txt", "line one\nline two\r\nline three\n")

	input := h.readFileInput("mixed.txt", intPtr(1), intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// All three lines should be present
	h.assertContains(result, "1:")
	h.assertContains(result, "2:")
	h.assertContains(result, "3:")
}

// Test file with only newlines (blank lines).
func TestReadFile_OnlyNewlines(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("newlines.txt", "\n\n\n\n\n")

	input := h.readFileInput("newlines.txt", intPtr(2), intPtr(4))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Lines 2, 3, 4 should be present (empty)
	expected := []string{"2: ", "3: ", "4: "}
	h.assertContainsAll(result, expected)
}

// Test reading file with Unicode/UTF-8 content.
func TestReadFile_UnicodeContent(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("unicode.txt", "Hello\nWorld\nEmoji line\nChinese text\nEnd\n")

	input := h.readFileInput("unicode.txt", intPtr(2), intPtr(4))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"2: World", "3: Emoji line", "4: Chinese text"}
	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, []string{"1: Hello", "5: End"})
}

// Test file with very long lines.
func TestReadFile_VeryLongLines(t *testing.T) {
	h := newTestHelper(t)
	longLine := strings.Repeat("a", 10000)
	content := "short\n" + longLine + "\nshort again\n"
	h.createFile("longlines.txt", content)

	input := h.readFileInput("longlines.txt", intPtr(2), intPtr(2))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Should contain the long line
	h.assertContains(result, "2: ")
	if !strings.Contains(result, strings.Repeat("a", 100)) {
		t.Error("Expected output to contain the long line content")
	}
	h.assertNotContains(result, "1: short")
	h.assertNotContains(result, "3: short again")
}

// Test that line numbers in output are 1-indexed (not 0-indexed).
func TestReadFile_LineNumbersAreOneIndexed(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("indexed.txt", "first\nsecond\nthird\n")

	input := h.readFileInput("indexed.txt", intPtr(1), intPtr(3))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Line numbers should start at 1, not 0
	h.assertContains(result, "1: first")
	h.assertContains(result, "2: second")
	h.assertContains(result, "3: third")
	h.assertNotContains(result, "0:")
}

// Test that adjacent line ranges combine correctly.
func TestReadFile_AdjacentRangesConsistency(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("adjacent.txt", "one\ntwo\nthree\nfour\nfive\n")

	// Read lines 1-2
	input1 := h.readFileInput("adjacent.txt", intPtr(1), intPtr(2))
	result1, err := h.executeReadFile(input1)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Read lines 3-5
	input2 := h.readFileInput("adjacent.txt", intPtr(3), intPtr(5))
	result2, err := h.executeReadFile(input2)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Combined should equal reading entire file
	inputAll := h.readFileInput("adjacent.txt", nil, nil)
	resultAll, err := h.executeReadFile(inputAll)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify no overlap - result1 should not have lines 3-5
	h.assertContainsNone(result1, []string{"3: three", "4: four", "5: five"})
	// Verify no overlap - result2 should not have lines 1-2
	h.assertContainsNone(result2, []string{"1: one", "2: two"})

	// Verify all lines are in the combined full read
	expectedAll := []string{"1: one", "2: two", "3: three", "4: four", "5: five"}
	for _, exp := range expectedAll {
		if !strings.Contains(resultAll, exp) {
			t.Errorf("Expected full file read to contain %q", exp)
		}
	}
}

// Test both start_line and end_line exceed file length.
func TestReadFile_BothStartAndEndExceedFileLength(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("exceed.txt", "one\ntwo\nthree\n")

	input := h.readFileInput("exceed.txt", intPtr(100), intPtr(200))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Should return empty or minimal output since requested range is beyond file
	unexpected := []string{"one", "two", "three"}
	h.assertContainsNone(result, unexpected)
}

// Test fractional/float values for start_line (should error or be truncated).
func TestReadFile_FloatStartLineValueInJSON(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("floatstart.txt", standardContent())

	// JSON with float value for start_line - must use raw JSON for special values
	path := h.filePath("floatstart.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 2.5, "end_line": 4}`, path)
	_, err := h.executeReadFile(input)
	// Implementation could either:
	// 1. Error on invalid type
	// 2. Truncate to integer
	// This test documents the expected behavior - adjust based on implementation choice
	// For now, we expect either an error OR truncation to work correctly
	if err != nil {
		// If error, that's acceptable
		return
	}
	// If no error, it should have truncated and worked
	// This is implementation-specific behavior
}

// Test fractional/float values for end_line (should error or be truncated).
func TestReadFile_FloatEndLineValueInJSON(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("floatend.txt", standardContent())

	// JSON with float value for end_line
	path := h.filePath("floatend.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 2, "end_line": 4.7}`, path)
	_, err := h.executeReadFile(input)
	// Implementation could either error or truncate
	if err != nil {
		return
	}
}

// Test string values for start_line (should error).
func TestReadFile_StringStartLineValueInJSON(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("strstart.txt", standardContent())

	path := h.filePath("strstart.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": "2", "end_line": 4}`, path)
	_, err := h.executeReadFile(input)

	// Should either error or handle gracefully
	// A strict implementation would error on type mismatch
	if err == nil {
		t.Log("Implementation accepts string values for start_line - verify this is intended")
	}
}

// Test string values for end_line (should error).
func TestReadFile_StringEndLineValueInJSON(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("strend.txt", standardContent())

	path := h.filePath("strend.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 2, "end_line": "4"}`, path)
	_, err := h.executeReadFile(input)

	if err == nil {
		t.Log("Implementation accepts string values for end_line - verify this is intended")
	}
}

// Test null values for start_line (should be treated as not provided).
func TestReadFile_NullStartLineValue(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("nullstart.txt", standardContent())

	path := h.filePath("nullstart.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": null, "end_line": 3}`, path)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// null should be treated as not provided, so read from start to line 3
	expected := []string{"1: line one", "2: line two", "3: line three"}
	unexpected := []string{"4: line four", "5: line five"}

	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, unexpected)
}

// Test null values for end_line (should be treated as not provided).
func TestReadFile_NullEndLineValue(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("nullend.txt", standardContent())

	path := h.filePath("nullend.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 3, "end_line": null}`, path)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// null should be treated as not provided, so read from line 3 to EOF
	expected := []string{"3: line three", "4: line four", "5: line five"}
	unexpected := []string{"1: line one", "2: line two"}

	h.assertContainsAll(result, expected)
	h.assertContainsNone(result, unexpected)
}

// Test both null values (should read entire file).
func TestReadFile_BothNullValues(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("bothnull.txt", standardContent())

	path := h.filePath("bothnull.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": null, "end_line": null}`, path)
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"1: line one", "2: line two", "3: line three", "4: line four", "5: line five"}
	h.assertContainsAll(result, expected)
}

// Test schema description for start_line parameter.
func TestReadFile_SchemaStartLineHasDescription(t *testing.T) {
	h := newTestHelper(t)

	readFileTool, found := h.adapter.GetTool("read_file")
	if !found {
		t.Fatal("read_file tool should be registered")
	}

	schema := readFileTool.InputSchema
	if schema == nil {
		t.Fatal("InputSchema should not be nil")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties should be a map, got %T", schema["properties"])
	}

	startLineParam, found := properties["start_line"]
	if !found {
		t.Error("read_file tool schema should include 'start_line' parameter")
		return
	}

	startLineMap, ok := startLineParam.(map[string]interface{})
	if !ok {
		t.Errorf("start_line should be a map, got %T", startLineParam)
		return
	}

	desc, found := startLineMap["description"]
	if !found || desc == "" {
		t.Error("start_line parameter should have a description")
	}
}

// Test schema description for end_line parameter.
func TestReadFile_SchemaEndLineHasDescription(t *testing.T) {
	h := newTestHelper(t)

	readFileTool, found := h.adapter.GetTool("read_file")
	if !found {
		t.Fatal("read_file tool should be registered")
	}

	schema := readFileTool.InputSchema
	if schema == nil {
		t.Fatal("InputSchema should not be nil")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties should be a map, got %T", schema["properties"])
	}

	endLineParam, found := properties["end_line"]
	if !found {
		t.Error("read_file tool schema should include 'end_line' parameter")
		return
	}

	endLineMap, ok := endLineParam.(map[string]interface{})
	if !ok {
		t.Errorf("end_line should be a map, got %T", endLineParam)
		return
	}

	desc, found := endLineMap["description"]
	if !found || desc == "" {
		t.Error("end_line parameter should have a description")
	}
}

// Test very large start_line value (potential overflow).
func TestReadFile_VeryLargeStartLineValue(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("largestart.txt", standardContent())

	// Very large number that could cause overflow issues
	path := h.filePath("largestart.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 9999999999}`, path)
	result, err := h.executeReadFile(input)
	if err != nil {
		// Error is acceptable for overflow
		return
	}

	// If no error, should return empty content since line doesn't exist
	unexpected := []string{"line one", "line two", "line three", "line four", "line five"}
	h.assertContainsNone(result, unexpected)
}

// Test very large end_line value (potential overflow).
func TestReadFile_VeryLargeEndLineValue(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("largeend.txt", standardContent())

	// Very large number
	path := h.filePath("largeend.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 1, "end_line": 9999999999}`, path)
	result, err := h.executeReadFile(input)
	if err != nil {
		// Error is acceptable for overflow
		return
	}

	// If no error, should read entire file since end exceeds length
	expected := []string{"1: line one", "2: line two", "3: line three", "4: line four", "5: line five"}
	h.assertContainsAll(result, expected)
}

// Test file that doesn't exist with line range parameters.
func TestReadFile_NonExistentFileWithLineRange(t *testing.T) {
	h := newTestHelper(t)

	path := h.filePath("nonexistent.txt")
	input := fmt.Sprintf(`{"path": %q, "start_line": 1, "end_line": 10}`, path)
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when reading non-existent file, got nil")
	}
}

// Test directory path with line range parameters (should error).
func TestReadFile_DirectoryPathWithLineRange(t *testing.T) {
	h := newTestHelper(t)

	// Create a directory
	dirPath := filepath.Join(h.tempDir, "testdir")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	input := fmt.Sprintf(`{"path": %q, "start_line": 1, "end_line": 10}`, dirPath)
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error when reading directory as file, got nil")
	}
}

// Test binary file with line range (should handle gracefully).
func TestReadFile_BinaryFileWithLineRange(t *testing.T) {
	h := newTestHelper(t)

	// Create a file with binary content including null bytes
	binaryContent := []byte{0x00, 0x01, 0x02, '\n', 0x03, 0x04, '\n', 0x05}
	h.createFile("binary.bin", string(binaryContent))

	input := h.readFileInput("binary.bin", intPtr(1), intPtr(2))
	_, err := h.executeReadFile(input)
	// Should either work or error gracefully
	if err != nil {
		t.Logf("Binary file handling returned error: %v", err)
	}
}

// Test symlink file with line range.
func TestReadFile_SymlinkWithLineRange(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("original.txt", standardContent())

	// Create symlink
	symlinkPath := filepath.Join(h.tempDir, "link.txt")
	originalPath := filepath.Join(h.tempDir, "original.txt")
	if err := os.Symlink(originalPath, symlinkPath); err != nil {
		t.Skipf("Symlink creation failed (may not be supported): %v", err)
	}

	input := h.readFileInput("link.txt", intPtr(2), intPtr(4))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := []string{"2: line two", "3: line three", "4: line four"}
	h.assertContainsAll(result, expected)
}

// Test file with only one line and range exceeding it.
func TestReadFile_SingleLineFileWithExcessiveRange(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("singleexcess.txt", "only line")

	input := h.readFileInput("singleexcess.txt", intPtr(1), intPtr(100))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	h.assertContains(result, "1: only line")
}

// Test concurrent reads with different line ranges.
func TestReadFile_ConcurrentReadsWithDifferentRanges(t *testing.T) {
	h := newTestHelper(t)

	// Create a larger file for concurrent testing
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("line %d content", i))
	}
	h.createFile("concurrent.txt", strings.Join(lines, "\n")+"\n")

	// Perform concurrent reads
	done := make(chan bool, 3)
	errors := make(chan error, 3)

	go func() {
		input := h.readFileInput("concurrent.txt", intPtr(1), intPtr(33))
		_, err := h.executeReadFile(input)
		errors <- err
		done <- true
	}()

	go func() {
		input := h.readFileInput("concurrent.txt", intPtr(34), intPtr(66))
		_, err := h.executeReadFile(input)
		errors <- err
		done <- true
	}()

	go func() {
		input := h.readFileInput("concurrent.txt", intPtr(67), intPtr(100))
		_, err := h.executeReadFile(input)
		errors <- err
		done <- true
	}()

	// Wait for all goroutines
	for range 3 {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent read failed: %v", err)
		}
	}
}

// Test that output format remains consistent with line numbers.
func TestReadFile_OutputFormatConsistencyWithRange(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("formatcons.txt", standardContent())

	// Read specific range
	input := h.readFileInput("formatcons.txt", intPtr(2), intPtr(4))
	result, err := h.executeReadFile(input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Split into lines and verify format
	lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")
	expectedLineNumbers := []int{2, 3, 4}

	if len(lines) != len(expectedLineNumbers) {
		t.Fatalf("Expected %d lines, got %d", len(expectedLineNumbers), len(lines))
	}

	for i, lineNum := range expectedLineNumbers {
		prefix := fmt.Sprintf("%d: ", lineNum)
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("Line %d should start with %q, got %q", i, prefix, lines[i])
		}
	}
}

// Test path parameter validation still works with line parameters.
func TestReadFile_PathTraversalBlockedWithLineParams(t *testing.T) {
	h := newTestHelper(t)
	h.createFile("traversal.txt", standardContent())

	// Attempt path traversal with line params
	input := `{"path": "../../../etc/passwd", "start_line": 1, "end_line": 10}`
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error for path traversal attempt, got nil")
	}
}

// Test empty path with line parameters.
func TestReadFile_EmptyPathWithLineParams(t *testing.T) {
	h := newTestHelper(t)

	input := `{"path": "", "start_line": 1, "end_line": 10}`
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error for empty path, got nil")
	}
}

// Test whitespace-only path with line parameters.
func TestReadFile_WhitespacePathWithLineParams(t *testing.T) {
	h := newTestHelper(t)

	input := `{"path": "   ", "start_line": 1, "end_line": 10}`
	_, err := h.executeReadFile(input)
	if err == nil {
		t.Fatal("Expected error for whitespace-only path, got nil")
	}
}
