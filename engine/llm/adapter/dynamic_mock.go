package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/tmc/langchaingo/llms"
)

// DynamicMockLLM is a mock LLM that returns fixture-defined outputs
type DynamicMockLLM struct {
	model           string
	expectedOutputs map[string]core.Output
	fallback        *MockLLM
}

// NewDynamicMockLLM creates a new dynamic mock LLM with expected outputs
func NewDynamicMockLLM(model string, expectedOutputs map[string]core.Output) *DynamicMockLLM {
	return &DynamicMockLLM{
		model:           model,
		expectedOutputs: expectedOutputs,
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
	for _, message := range messages {
		if message.Role == llms.ChatMessageTypeHuman || message.Role == llms.ChatMessageTypeSystem {
			for _, part := range message.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					return textPart.Text
				}
			}
		}
	}
	return ""
}

// matchActionPattern matches text against known action patterns
func (m *DynamicMockLLM) matchActionPattern(text string) string {
	if text == "" {
		return ""
	}

	// Check patterns for each action
	if strings.Contains(text, "Read file content") || strings.Contains(text, "read_content") {
		return "read_content"
	}
	if strings.Contains(text, "Analyze the following Go code") || strings.Contains(text, "analyze") {
		return "analyze_content"
	}
	if strings.Contains(text, "Process city data") || strings.Contains(text, "process_city") {
		return "process_city"
	}
	if strings.Contains(text, "Analyze a single activity") || strings.Contains(text, "analyze_activity") {
		return "analyze_activity"
	}
	if strings.Contains(text, "Process a single collection item") || strings.Contains(text, "process_item") {
		return "process_item"
	}
	return ""
}

// Call implements the legacy Call interface
func (m *DynamicMockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	// For legacy interface, fall back to static mock
	return m.fallback.Call(ctx, prompt, options...)
}
