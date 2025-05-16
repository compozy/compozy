package nats

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Subjects(t *testing.T) {
	t.Run("Should generate correct agent request subject", func(t *testing.T) {
		execID := "exec123"
		agentID := "agent123"
		expected := "compozy.exec123.agent.agent123.request"
		result := GenAgentRequestSubject(execID, agentID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct agent response subject", func(t *testing.T) {
		execID := "exec123"
		agentID := "agent123"
		expected := "compozy.exec123.agent.agent123.response"
		result := GenAgentResponseSubject(execID, agentID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct tool request subject", func(t *testing.T) {
		execID := "exec123"
		toolID := "tool123"
		expected := "compozy.exec123.tool.tool123.request"
		result := GenToolRequestSubject(execID, toolID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct tool response subject", func(t *testing.T) {
		execID := "exec123"
		toolID := "tool123"
		expected := "compozy.exec123.tool.tool123.response"
		result := GenToolResponseSubject(execID, toolID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct error subject", func(t *testing.T) {
		execID := "exec123"
		expected := "compozy.exec123.error"
		result := GenErrorSubject(execID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct agent request wildcard", func(t *testing.T) {
		execID := "exec123"
		agentID := "agent123"
		expected := "compozy.exec123.agent.agent123.request.>"
		result := GenAgentRequestWildcard(execID, agentID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct tool request wildcard", func(t *testing.T) {
		execID := "exec123"
		toolID := "tool123"
		expected := "compozy.exec123.tool.tool123.request.>"
		result := GenToolRequestWildcard(execID, toolID)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct log subject", func(t *testing.T) {
		execID := "exec123"
		level := InfoLevel
		expected := "compozy.exec123.log.info"
		result := GenLogSubject(execID, level)
		assert.Equal(t, expected, result)
	})

	t.Run("Should generate correct log wildcard subject", func(t *testing.T) {
		execID := "test-exec-6"
		subject := GenLogWildcard(execID)
		expected := fmt.Sprintf("%s.%s.%s.*", SubjectPrefix, execID, SubjectLog)
		assert.Equal(t, expected, subject)
	})

	t.Run("Should generate correct log level wildcard subject", func(t *testing.T) {
		execID := "exec123"
		level := InfoLevel
		subject := GenLogLevelWildcard(execID, level)
		expected := fmt.Sprintf("%s.%s.%s.%s", SubjectPrefix, execID, SubjectLog, level)
		assert.Equal(t, expected, subject)
	})
}
