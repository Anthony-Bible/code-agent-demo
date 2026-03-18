package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

const rcaPromptTemplate = `As an expert SRE, analyze the following investigation findings and correlate them into a structured Root Cause Analysis (RCA).

Findings:
%s

You must respond with a JSON object containing a list of RCA findings. Each finding should include a summary, a list of root causes, and suggested remedies.
Return at least 2 remedies for each finding.

Expected JSON format:
{
  "findings": [
    {
      "summary": "Overall summary of the issue",
      "causes": [
        {
          "id": "C1",
          "description": "Detailed description of the cause",
          "confidence_score": 0.9,
          "evidence": ["Evidence 1", "Evidence 2"]
        }
      ],
      "remedies": [
        {
          "description": "Remedy 1",
          "actionable_steps": ["Step 1", "Step 2"],
          "impact": "High"
        }
      ]
    }
  ]
}

Only return the JSON object.`

// RCAService provides Root Cause Analysis correlation logic.
type RCAService struct {
	aiProvider port.AIProvider
}

// NewRCAService creates a new RCAService.
func NewRCAService(aiProvider port.AIProvider) *RCAService {
	return &RCAService{
		aiProvider: aiProvider,
	}
}

// Correlate takes a list of investigation findings and correlates them into structured RCAFindings.
func (s *RCAService) Correlate(ctx context.Context, findings []entity.InvestigationFinding) ([]entity.RCAFinding, error) {
	if len(findings) == 0 {
		return nil, nil
	}

	slog.Info("correlating findings into RCA", "findings_count", len(findings)) //nolint:sloglint // keep global logger for now

	// 1. Prepare findings for the prompt
	var findingsText strings.Builder
	for i, f := range findings {
		fmt.Fprintf(&findingsText, "%d. [%s] %s (Severity: %s)\n", i+1, f.Type, f.Description, f.Severity)
	}

	// 2. Build prompt for Claude
	prompt := fmt.Sprintf(rcaPromptTemplate, findingsText.String())

	// 3. Send to AI
	messages := []port.MessageParam{
		{
			Role:    entity.RoleUser,
			Content: prompt,
		},
	}

	resp, _, err := s.aiProvider.SendMessage(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get RCA correlation from AI: %w", err)
	}

	// 4. Parse response
	var response struct {
		Findings []entity.RCAFinding `json:"findings"`
	}

	content := resp.Content

	// Pre-process response to strip markdown code blocks if the AI provided them
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	err = json.Unmarshal([]byte(content), &response)
	if err != nil {
		// Attempt to extract JSON if Claude added conversational filler
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start != -1 && end != -1 && end > start {
			err = json.Unmarshal([]byte(content[start:end+1]), &response)
		}

		if err != nil {
			contentPreview := resp.Content
			if len(contentPreview) > 200 {
				contentPreview = contentPreview[:200] + "..."
			}
			slog.Error("failed to parse RCA findings JSON", //nolint:sloglint // keep global logger for now
				"error", err,
				"content_preview", contentPreview)
			return nil, fmt.Errorf("failed to parse RCA findings JSON: %w", err)
		}
	}

	// 5. Basic validation of the result
	for _, f := range response.Findings {
		if err := f.Validate(); err != nil {
			slog.Error("AI returned invalid RCA finding", "error", err) //nolint:sloglint // keep global logger for now
			return nil, fmt.Errorf("AI returned invalid RCA finding: %w", err)
		}
	}

	slog.Info("RCA correlation complete", "findings_count", len(response.Findings)) //nolint:sloglint // keep global logger for now
	return response.Findings, nil
}
