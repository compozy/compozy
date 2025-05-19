package tool

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/google/uuid"
)

// Request represents a request to execute a tool
type Request struct {
	ID           string          `json:"id"`
	ToolID       string          `json:"tool_id"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}

// NewToolRequest creates a new tool request
func NewToolRequest(toolID, description string, inputSchema, outputSchema, input any) (*Request, error) {
	var inputSchemaJSON, outputSchemaJSON, inputJSON json.RawMessage
	var err error

	if inputSchema != nil {
		inputSchemaJSON, err = json.Marshal(inputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input schema: %w", err)
		}
	}

	if outputSchema != nil {
		outputSchemaJSON, err = json.Marshal(outputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal output schema: %w", err)
		}
	}

	if input != nil {
		inputJSON, err = json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input: %w", err)
		}
	}

	return &Request{
		ID:           uuid.New().String(),
		ToolID:       toolID,
		Description:  description,
		InputSchema:  inputSchemaJSON,
		OutputSchema: outputSchemaJSON,
		Input:        inputJSON,
	}, nil
}

// Response represents a response from a tool execution
type Response struct {
	ID     string                `json:"id"`
	ToolID string                `json:"tool_id"`
	Output json.RawMessage       `json:"output"`
	Status common.ResponseStatus `json:"status"`
}
