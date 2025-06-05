package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// MockLLM is a mock implementation of the LLM interface for testing
type MockLLM struct {
	model string
}

// NewMockLLM creates a new mock LLM
func NewMockLLM(model string) *MockLLM {
	return &MockLLM{
		model: model,
	}
}

// GenerateContent implements the LLM interface with predictable responses
func (m *MockLLM) GenerateContent(
	ctx context.Context,
	messages []llms.MessageContent,
	_ ...llms.CallOption,
) (*llms.ContentResponse, error) {
	// Extract the human message content to generate a response based on it
	var prompt string
	for _, message := range messages {
		if message.Role == llms.ChatMessageTypeHuman {
			for _, part := range message.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					prompt = textPart.Text
				}
			}
		}
	}

	// Simulate long execution for cancellation testing scenarios only
	if strings.Contains(prompt, "duration: 10s") ||
		strings.Contains(prompt, "Think deeply") ||
		strings.Contains(prompt, "cancellation-test") {
		// Simulate a long-running task for cancellation testing
		delay := 500 * time.Millisecond
		if strings.Contains(prompt, "duration: 10s") || strings.Contains(prompt, "Think deeply") {
			delay = 2 * time.Second // Longer delay for explicit long-running tasks
		}

		select {
		case <-time.After(delay):
			// Task completed normally
		case <-ctx.Done():
			// Task was canceled
			return nil, ctx.Err()
		}
	}

	// Generate a predictable response based on the prompt
	var responseText string
	if prompt != "" {
		responseText = fmt.Sprintf("Mock response for: %s", prompt)
	} else {
		responseText = "Mock agent response: task completed successfully"
	}

	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: responseText,
			},
		},
	}

	return response, nil
}

// Call implements the legacy Call interface
func (m *MockLLM) Call(_ context.Context, prompt string, _ ...llms.CallOption) (string, error) {
	response := fmt.Sprintf("Mock response for: %s", prompt)
	return response, nil
}
