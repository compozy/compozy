package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/tmc/langchaingo/llms"
)

// actionPattern represents a regex pattern and its corresponding action ID
type actionPattern struct {
	actionID string
	pattern  *regexp.Regexp
}

// actionPatterns is a table of regex patterns for matching action IDs
var actionPatterns = []actionPattern{
	{"read_content", regexp.MustCompile(`(?i)Read file content|read_content`)},
	{"analyze_content", regexp.MustCompile(`(?i)Analyze the following Go code|analyze`)},
	{"process_city", regexp.MustCompile(`(?i)Process city data|process_city`)},
	{"analyze_activity", regexp.MustCompile(`(?i)Analyze a single activity|analyze_activity`)},
	{"process_item", regexp.MustCompile(`(?i)Process a single collection item|process_item`)},
}

// DynamicMockLLM is a mock LLM that returns fixture-defined outputs
type DynamicMockLLM struct {
	model           string
	expectedOutputs map[string]core.Output
	fallback        *MockLLM
}

// NewDynamicMockLLM creates a new dynamic mock LLM that returns fixture-defined outputs
// for prompts that match expected action patterns. Falls back to NewMockLLM for unmatched prompts.
func NewDynamicMockLLM(model string, expectedOutputs map[string]core.Output) *DynamicMockLLM {
	copied := make(map[string]core.Output, len(expectedOutputs))
	// shallow copy is enough for test fixtures; deep copy if needed later
	maps.Copy(copied, expectedOutputs)
	return &DynamicMockLLM{
		model:           model,
		expectedOutputs: copied,
		fallback:        NewMockLLM(model),
	}
}

// GenerateContent returns the expected output for a given action ID
func (m *DynamicMockLLM) GenerateContent(
	ctx context.Context,
	messages []llms.MessageContent,
	options ...llms.CallOption,
) (*llms.ContentResponse, error) {
	// Extract action ID from messages
	actionID := m.extractActionID(messages)

	// Look for expected output
	if actionID != "" {
		if expectedOutput, exists := m.expectedOutputs[actionID]; exists {
			// Convert core.Output to JSON string
			jsonBytes, err := json.Marshal(expectedOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal expected output: %w", err)
			}

			return &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						Content: string(jsonBytes),
					},
				},
			}, nil
		}
	}

	// Fall back to static mock behavior if no expected output found
	return m.fallback.GenerateContent(ctx, messages, options...)
}

// extractActionID attempts to extract the action ID from messages
func (m *DynamicMockLLM) extractActionID(messages []llms.MessageContent) string {
	text := m.extractPromptText(messages)
	return m.matchActionPattern(text)
}

// extractPromptText extracts text content from messages
func (m *DynamicMockLLM) extractPromptText(messages []llms.MessageContent) string {
	var b strings.Builder
	for _, message := range messages {
		for _, part := range message.Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(textPart.Text)
			}
		}
	}
	return b.String()
}

// matchActionPattern matches text against known action patterns using regex table
func (m *DynamicMockLLM) matchActionPattern(text string) string {
	if text == "" {
		return ""
	}
	for _, ap := range actionPatterns {
		if ap.pattern.MatchString(text) {
			return ap.actionID
		}
	}
	return ""
}

// Call implements the legacy Call interface
func (m *DynamicMockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	// For legacy interface, fall back to static mock
	return m.fallback.Call(ctx, prompt, options...)
}
