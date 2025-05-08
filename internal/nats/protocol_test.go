package nats

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Protocol(t *testing.T) {
	t.Run("Should create new message with valid payload", func(t *testing.T) {
		payload := map[string]string{"key": "value"}
		msg, err := NewMessage(TypeAgentRequest, payload)
		assert.NoError(t, err)
		assert.Equal(t, TypeAgentRequest, msg.Type)
		var result map[string]string
		err = msg.UnmarshalPayload(&result)
		assert.NoError(t, err)
		assert.Equal(t, payload, result)
	})

	t.Run("Should return error when creating message with invalid payload", func(t *testing.T) {
		invalidPayload := make(chan int) // Unmarshalable type
		_, err := NewMessage(TypeAgentRequest, invalidPayload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal payload")
	})

	t.Run("Should unmarshal payload correctly", func(t *testing.T) {
		payload := map[string]string{"key": "value"}
		msg, _ := NewMessage(TypeAgentRequest, payload)
		var result map[string]string
		err := msg.UnmarshalPayload(&result)
		assert.NoError(t, err)
		assert.Equal(t, payload, result)
	})

	t.Run("Should return error when unmarshaling invalid JSON", func(t *testing.T) {
		msg := &Message{Type: TypeAgentRequest, Payload: []byte("invalid json")}
		var result map[string]string
		err := msg.UnmarshalPayload(&result)
		assert.Error(t, err)
	})

	t.Run("Should create new agent request with all fields", func(t *testing.T) {
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

	t.Run("Should create new tool request with all fields", func(t *testing.T) {
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

	t.Run("Should return error when creating tool request with invalid schema", func(t *testing.T) {
		invalidSchema := make(chan int)
		_, err := NewToolRequest("tool123", "desc", invalidSchema, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal input schema")
	})

	t.Run("Should create new error message with all fields", func(t *testing.T) {
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

	t.Run("Should return error when creating error message with invalid data", func(t *testing.T) {
		invalidData := make(chan int)
		_, err := NewErrorMessage("error", "req123", "", invalidData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal error data")
	})

	t.Run("Should create new log message with all fields", func(t *testing.T) {
		level := InfoLevel
		message := "Test log message"
		context := map[string]any{"key": "value"}
		timestamp := time.Now()

		logMsg, err := NewLogLevel(level, message, context, timestamp)
		assert.NoError(t, err)
		assert.Equal(t, level, logMsg.Level)
		assert.Equal(t, message, logMsg.Message)
		assert.Equal(t, context, logMsg.Context)
		assert.Equal(t, timestamp, logMsg.Timestamp)
	})

	t.Run("Should return error when creating log message with empty level", func(t *testing.T) {
		_, err := NewLogLevel("", "message", nil, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "log level cannot be empty")
	})

	t.Run("Should return error when creating log message with empty message", func(t *testing.T) {
		_, err := NewLogLevel(InfoLevel, "", nil, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "log message cannot be empty")
	})

	t.Run("Should return error when creating log message with zero timestamp", func(t *testing.T) {
		_, err := NewLogLevel(InfoLevel, "message", nil, time.Time{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp cannot be zero")
	})

	t.Run("Should marshal log message to JSON correctly", func(t *testing.T) {
		level := DebugLevel
		message := "Debug message"
		context := map[string]any{"debug": true}
		timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		logMsg, err := NewLogLevel(level, message, context, timestamp)
		assert.NoError(t, err)

		jsonData, err := json.Marshal(logMsg)
		assert.NoError(t, err)

		var result LogMessage
		err = json.Unmarshal(jsonData, &result)
		assert.NoError(t, err)
		assert.Equal(t, level, result.Level)
		assert.Equal(t, message, result.Message)
		assert.Equal(t, context, result.Context)
		assert.Equal(t, timestamp, result.Timestamp)
	})
}

func Test_LogSubjects(t *testing.T) {
	t.Run("Should generate correct log subject", func(t *testing.T) {
		subject := GenLogSubject(InfoLevel)
		expected := fmt.Sprintf("%s.%s.%s", SubjectPrefix, SubjectLog, InfoLevel)
		assert.Equal(t, expected, subject)
	})

	t.Run("Should generate correct log wildcard subject", func(t *testing.T) {
		subject := GenLogWildcard()
		expected := fmt.Sprintf("%s.%s.>", SubjectPrefix, SubjectLog)
		assert.Equal(t, expected, subject)
	})
}

func Test_LogServer(t *testing.T) {
	server, err := NewNatsServer(DefaultServerOptions())
	assert.NoError(t, err)
	defer server.Shutdown()

	t.Run("Should publish and receive log messages", func(t *testing.T) {
		received := make(chan *LogMessage, 1)
		sub, err := server.SubscribeToLogs(func(msg *LogMessage) {
			received <- msg
		})
		assert.NoError(t, err)
		defer sub.Unsubscribe()

		logMsg, err := NewLogLevel(InfoLevel, "Test log", map[string]any{"test": true}, time.Now())
		assert.NoError(t, err)

		err = server.PublishLog(logMsg)
		assert.NoError(t, err)

		select {
		case msg := <-received:
			assert.Equal(t, logMsg.Level, msg.Level)
			assert.Equal(t, logMsg.Message, msg.Message)
			assert.Equal(t, logMsg.Context, msg.Context)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for log message")
		}
	})

	t.Run("Should subscribe to specific log level", func(t *testing.T) {
		received := make(chan *LogMessage, 1)
		sub, err := server.SubscribeToLogLevel(ErrorLevel, func(msg *LogMessage) {
			received <- msg
		})
		assert.NoError(t, err)
		defer sub.Unsubscribe()

		// Publish error log
		errorLog, err := NewLogLevel(ErrorLevel, "Error log", nil, time.Now())
		assert.NoError(t, err)
		err = server.PublishLog(errorLog)
		assert.NoError(t, err)

		// Publish info log (should not be received)
		infoLog, err := NewLogLevel(InfoLevel, "Info log", nil, time.Now())
		assert.NoError(t, err)
		err = server.PublishLog(infoLog)
		assert.NoError(t, err)

		select {
		case msg := <-received:
			assert.Equal(t, ErrorLevel, msg.Level)
			assert.Equal(t, "Error log", msg.Message)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for error log message")
		}
	})
}
