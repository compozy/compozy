package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Subjects(t *testing.T) {
	t.Run("Should generate correct agent request subject", func(t *testing.T) {
		agentID := "agent123"
		requestID := "req123"
		expected := "compozy.agent.agent123.request.req123"
		result := GenAgentRequestSubject(agentID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct agent response subject", func(t *testing.T) {
		agentID := "agent123"
		requestID := "req123"
		expected := "compozy.agent.agent123.response.req123"
		result := GenAgentResponseSubject(agentID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct tool request subject", func(t *testing.T) {
		toolID := "tool123"
		requestID := "req123"
		expected := "compozy.tool.tool123.request.req123"
		result := GenToolRequestSubject(toolID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct tool response subject", func(t *testing.T) {
		toolID := "tool123"
		requestID := "req123"
		expected := "compozy.tool.tool123.response.req123"
		result := GenToolResponseSubject(toolID, requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct error subject", func(t *testing.T) {
		requestID := "req123"
		expected := "compozy.error.req123"
		result := GenErrorSubject(requestID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct agent request wildcard", func(t *testing.T) {
		agentID := "agent123"
		expected := "compozy.agent.agent123.request.>"
		result := GenAgentRequestWildcard(agentID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct tool request wildcard", func(t *testing.T) {
		toolID := "tool123"
		expected := "compozy.tool.tool123.request.>"
		result := GenToolRequestWildcard(toolID)
		assert.Equal(t, expected, result)
	})
}
