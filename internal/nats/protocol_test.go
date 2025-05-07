package nats

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocol(t *testing.T) {
	t.Run("NewMessage", func(t *testing.T) {
		payload := map[string]string{"key": "value"}
		msg, err := NewMessage(TypeAgentRequest, payload)
		assert.NoError(t, err)
		assert.Equal(t, TypeAgentRequest, msg.Type)
		var result map[string]string
		err = msg.UnmarshalPayload(&result)
		assert.NoError(t, err)
		assert.Equal(t, payload, result)
	})

	t.Run("NewMessage_InvalidPayload", func(t *testing.T) {
		invalidPayload := make(chan int) // Unmarshalable type
		_, err := NewMessage(TypeAgentRequest, invalidPayload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal payload")
	})

	t.Run("UnmarshalPayload", func(t *testing.T) {
		payload := map[string]string{"key": "value"}
		msg, _ := NewMessage(TypeAgentRequest, payload)
		var result map[string]string
		err := msg.UnmarshalPayload(&result)
		assert.NoError(t, err)
		assert.Equal(t, payload, result)
	})

	t.Run("UnmarshalPayload_InvalidJSON", func(t *testing.T) {
		msg := &Message{Type: TypeAgentRequest, Payload: []byte("invalid json")}
		var result map[string]string
		err := msg.UnmarshalPayload(&result)
		assert.Error(t, err)
	})

	t.Run("NewAgentRequest", func(t *testing.T) {
		agentID := "agent123"
		instructions := "Process data"
		action := AgentActionRequest{ActionID: "action1", Prompt: "Do it"}
		config := map[string]any{"setting": "value"}
		tools := []ToolRequest{{ID: "tool1", ToolID: "tool123"}}
		req := NewAgentRequest(agentID, instructions, action, config, tools)
		assert.NotEmpty(t, req.ID)
		assert.Equal(t, agentID, req.AgentID)
		assert.Equal(t, instructions, req.Instructions)
		assert.Equal(t, action, req.Action)
		assert.Equal(t, config, req.Config)
		assert.Equal(t, tools, req.Tools)
	})

	t.Run("NewToolRequest", func(t *testing.T) {
		toolID := "tool123"
		description := "Tool description"
		inputSchema := map[string]string{"type": "string"}
		outputSchema := map[string]string{"type": "number"}
		input := map[string]string{"data": "input"}
		req, err := NewToolRequest(toolID, description, inputSchema, outputSchema, input)
		assert.NoError(t, err)
		assert.NotEmpty(t, req.ID)
		assert.Equal(t, toolID, req.ToolID)
		assert.Equal(t, description, req.Description)
		var parsedInputSchema, parsedOutputSchema, parsedInput map[string]string
		assert.NoError(t, json.Unmarshal(req.InputSchema, &parsedInputSchema))
		assert.NoError(t, json.Unmarshal(req.OutputSchema, &parsedOutputSchema))
		assert.NoError(t, json.Unmarshal(req.Input, &parsedInput))
		assert.Equal(t, inputSchema, parsedInputSchema)
		assert.Equal(t, outputSchema, parsedOutputSchema)
		assert.Equal(t, input, parsedInput)
	})

	t.Run("NewToolRequest_InvalidSchema", func(t *testing.T) {
		invalidSchema := make(chan int)
		_, err := NewToolRequest("tool123", "desc", invalidSchema, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal input schema")
	})

	t.Run("NewErrorMessage", func(t *testing.T) {
		message := "Something went wrong"
		requestID := "req123"
		stack := "stack trace"
		data := map[string]string{"error": "details"}
		errMsg, err := NewErrorMessage(message, requestID, stack, data)
		assert.NoError(t, err)
		assert.Equal(t, requestID, errMsg.RequestID)
		assert.Equal(t, message, errMsg.Message)
		assert.Equal(t, stack, errMsg.Stack)
		var parsedData map[string]string
		assert.NoError(t, json.Unmarshal(errMsg.Data, &parsedData))
		assert.Equal(t, data, parsedData)
	})

	t.Run("NewErrorMessage_InvalidData", func(t *testing.T) {
		invalidData := make(chan int)
		_, err := NewErrorMessage("error", "req123", "", invalidData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal error data")
	})
}
