package nats

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ResponseStatus represents the status of a response
type ResponseStatus string

const (
	StatusSuccess ResponseStatus = "success"
	StatusError   ResponseStatus = "error"
)

// MessageType represents the type of message
type MessageType string

const (
	TypeAgentRequest  MessageType = "AgentRequest"
	TypeAgentResponse MessageType = "AgentResponse"
	TypeToolRequest   MessageType = "ToolRequest"
	TypeToolResponse  MessageType = "ToolResponse"
	TypeError         MessageType = "Error"
)

// Message is the base structure for all protocol messages
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(messageType MessageType, payload any) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return &Message{
		Type:    messageType,
		Payload: payloadBytes,
	}, nil
}

// UnmarshalPayload unmarshals the payload into the given target
func (m *Message) UnmarshalPayload(target any) error {
	return json.Unmarshal(m.Payload, target)
}

// AgentRequest represents a request to execute an agent
type AgentRequest struct {
	ID           string             `json:"id"`
	AgentID      string             `json:"agent_id"`
	Instructions string             `json:"instructions"`
	Action       AgentActionRequest `json:"action"`
	Config       map[string]any     `json:"config"`
	Tools        []ToolRequest      `json:"tools"`
}

// NewAgentRequest creates a new agent request
func NewAgentRequest(agentID, instructions string, action AgentActionRequest, config map[string]any, tools []ToolRequest) *AgentRequest {
	return &AgentRequest{
		ID:           uuid.New().String(),
		AgentID:      agentID,
		Instructions: instructions,
		Action:       action,
		Config:       config,
		Tools:        tools,
	}
}

// AgentActionRequest represents a request to execute an agent action
type AgentActionRequest struct {
	ActionID     string         `json:"action_id"`
	Prompt       string         `json:"prompt"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

// AgentResponse represents a response from an agent execution
type AgentResponse struct {
	ID      string          `json:"id"`
	AgentID string          `json:"agent_id"`
	Output  json.RawMessage `json:"output"`
	Status  ResponseStatus  `json:"status"`
}

// ToolRequest represents a request to execute a tool
type ToolRequest struct {
	ID           string          `json:"id"`
	ToolID       string          `json:"tool_id"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}

// NewToolRequest creates a new tool request
func NewToolRequest(toolID, description string, inputSchema, outputSchema, input any) (*ToolRequest, error) {
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

	return &ToolRequest{
		ID:           uuid.New().String(),
		ToolID:       toolID,
		Description:  description,
		InputSchema:  inputSchemaJSON,
		OutputSchema: outputSchemaJSON,
		Input:        inputJSON,
	}, nil
}

// ToolResponse represents a response from a tool execution
type ToolResponse struct {
	ID     string          `json:"id"`
	ToolID string          `json:"tool_id"`
	Output json.RawMessage `json:"output"`
	Status ResponseStatus  `json:"status"`
}

// ErrorMessage represents an error message
type ErrorMessage struct {
	RequestID string          `json:"request_id,omitempty"`
	Message   string          `json:"message"`
	Stack     string          `json:"stack,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewErrorMessage creates a new error message
func NewErrorMessage(message string, requestID string, stack string, data any) (*ErrorMessage, error) {
	var dataJSON json.RawMessage
	var err error

	if data != nil {
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal error data: %w", err)
		}
	}

	return &ErrorMessage{
		RequestID: requestID,
		Message:   message,
		Stack:     stack,
		Data:      dataJSON,
	}, nil
}
