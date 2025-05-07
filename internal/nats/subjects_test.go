package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubjects(t *testing.T) {
	t.Run("GenAgentRequestSubject", func(t *testing.T) {
		agentID := "agent123"
		requestID := "req123"
		expected := "compozy.agent.agent123.request.req123"
		result := GenAgentRequestSubject(agentID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("GenAgentResponseSubject", func(t *testing.T) {
		agentID := "agent123"
		requestID := "req123"
		expected := "compozy.agent.agent123.response.req123"
		result := GenAgentResponseSubject(agentID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("GenToolRequestSubject", func(t *testing.T) {
		toolID := "tool123"
		requestID := "req123"
		expected := "compozy.tool.tool123.request.req123"
		result := GenToolRequestSubject(toolID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("GenToolResponseSubject", func(t *testing.T) {
		toolID := "tool123"
		requestID := "req123"
		expected := "compozy.tool.tool123.response.req123"
		result := GenToolResponseSubject(toolID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("GenErrorSubject", func(t *testing.T) {
		requestID := "req123"
		expected := "compozy.error.req123"
		result := GenErrorSubject(requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("GenAgentRequestWildcard", func(t *testing.T) {
		agentID := "agent123"
		expected := "compozy.agent.agent123.request.>"
		result := GenAgentRequestWildcard(agentID)
		assert.Equal(t, expected, result)
	})

	t.Run("GenToolRequestWildcard", func(t *testing.T) {
		toolID := "tool123"
		expected := "compozy.tool.tool123.request.>"
		result := GenToolRequestWildcard(toolID)
		assert.Equal(t, expected, result)
	})
}
