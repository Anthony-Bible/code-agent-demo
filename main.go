package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	// Add this:
	"github.com/invopop/jsonschema"
)

type ToolDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	InputSchema anthropic.ToolInputSchemaParam `json:"inputSchema"`
	Function    func(input json.RawMessage) (string, error)
}

type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
	tools          []ToolDefinition
}

func main() {
	client := anthropic.NewClient()
	scanner := bufio.NewScanner(os.Stdin)

	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			fmt.Println("Bye!")
			return "", false
		}
		return scanner.Text(), true
	}
	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}
	agent := NewAgent(&client, getUserMessage, tools)
	err := agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error %s\n", err)
	}
}

func NewAgent(client *anthropic.Client, getUserMessage func() (string, bool), tools []ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []anthropic.MessageParam{}
	fmt.Println("Chat with claude(use 'ctrl+c'  to quit)")
	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mClaude\u001b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok {
				break
			}

			userMessage := anthropic.NewUserMessage(anthropic.NewTextBlock(userInput))
			conversation = append(conversation, userMessage)
		}

		message, err := a.runInference(ctx, conversation)
		if err != nil {
			fmt.Printf("Error %s\n", err)
			return err
		}
		conversation = append(conversation, message.ToParam())

		toolResults := []anthropic.ContentBlockParamUnion{}

		for _, content := range message.Content {
			switch content.Type {
			case "text":
				fmt.Printf("\u001b[93mClaude\u001b[0m:  %s\n", content.Text)
			case "thinking":
				fmt.Printf("\u001b[95mClaude (thinking)\u001b[0m:  %s\n", content.Thinking)
			case "tool_use":
				result := a.executeTool(content.ID, content.Name, content.Input)
				toolResults = append(toolResults, result)
			default:
				fmt.Printf("\u001b[93mClaude\u001b[0m:  %s\n", content.Type)
				fmt.Printf("Unknown content type: %s\n", content.Type)
			}
		}
		if len(toolResults) == 0 {
			readUserInput = true
			continue
		}
		readUserInput = false
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
	}
	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	anthropicTools := []anthropic.ToolUnionParam{}
	for _, tool := range a.tools {
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: tool.InputSchema,
			},
		})
	}
	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		// Model:     anthropic.ModelClaudeHaiku4_5,
		Model:     "hf:zai-org/GLM-4.6",
		MaxTokens: int64(1024),
		Messages:  conversation,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfDisabled: &anthropic.ThinkingConfigDisabledParam{},
		},
		Tools: anthropicTools,
	})
	return message, err
}

func GenerateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)

	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

func (a *Agent) executeTool(id string, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
	var toolDef ToolDefinition
	var found bool
	for _, tool := range a.tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return anthropic.NewToolResultBlock(id, "tool not found", true)
	}
	fmt.Printf("\u001b[92mClaude\u001b[0m:  %s(%s)\n", name, input)
	response, err := toolDef.Function(input)
	if err != nil {
		return anthropic.NewToolResultBlock(id, err.Error(), true)
	}
	return anthropic.NewToolResultBlock(id, response, false)
}

// tools definitions
var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Reads the contents of a given relative file path, use this when you want to see what's inside a file. Do not use this with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "Lists files and directories sat a given path. If not path is provided, lists files in the current working directory.",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Makes edits to a text file.
Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.
If the file specified with path doesn't exist, it will be created.'`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

// ReadFileInput represents the input required to read a file from the working directory by specifying its relative path.
type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path to the file to read in the working directory.."`
}

// ListFilesInput represents the input required to list files and directories in a given path. If no path is provided, lists files in the current working directory.
type ListFilesInput struct {
	Path string `json:"path" jsonschema_description:"The relative path to the directory to list files in. If not provided, lists files in the current working directory."`
}

// EditFileInput represents the input required to edit a file by replacing occurrences of a specified string with a new string.
type EditFileInput struct {
	Path   string `json:"path"    jsonschema_description:"The relative path to the file to edit."`
	OldStr string `json:"old_str" jsonschema_description:"The string to replace."`
	NewStr string `json:"new_str" jsonschema_description:"The string to replace 'old_str' with."`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

var EditFileInputSchema = GenerateSchema[EditFileInput]()

// ReadFile reads the contents of a file specified by the relative path in the input and returns it as a string.
func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}
	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}
	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}
	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		panic(err)
	}
	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters")
	}
	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldStr == "" {
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		return "", err
	}
	oldContent := string(content)
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)
	if oldContent == newContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("old string not found in file")
	}
	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0o644)
	if err != nil {
		return "", err
	}
	return "OK", nil
}

func createNewFile(filePath string, content string) (string, error) {
	dir := path.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	err := os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	return fmt.Sprintf("Created file %s", filePath), nil
}
