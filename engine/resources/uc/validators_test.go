package uc

import (
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/require"
)

func TestValidateBody_Scenarios(t *testing.T) {
	t.Run("Should validate happy path with id in body", func(t *testing.T) {
		id, err := validateBody(resources.ResourceAgent, map[string]any{"id": "a", "type": "agent"}, "", true)
		require.NoError(t, err)
		require.Equal(t, "a", id)
	})
	t.Run("Should error when body is nil", func(t *testing.T) {
		_, err := validateBody(resources.ResourceTool, nil, "", true)
		require.ErrorIs(t, err, ErrInvalidPayload)
	})
	t.Run("Should error when project field is present", func(t *testing.T) {
		_, err := validateBody(
			resources.ResourceTool,
			map[string]any{"id": "t", "type": "tool", "project": "p"},
			"",
			true,
		)
		require.ErrorIs(t, err, ErrProjectInBody)
	})
	t.Run("Should error when id has whitespace", func(t *testing.T) {
		_, err := validateBody(resources.ResourceAgent, map[string]any{"id": "a 1", "type": "agent"}, "", true)
		require.ErrorIs(t, err, ErrInvalidID)
	})
	t.Run("Should error when type mismatches", func(t *testing.T) {
		_, err := validateBody(resources.ResourceAgent, map[string]any{"id": "a", "type": "tool"}, "", true)
		require.ErrorIs(t, err, ErrTypeMismatch)
	})
	t.Run("Should accept id from path when body id empty and not required", func(t *testing.T) {
		id, err := validateBody(resources.ResourceAgent, map[string]any{"type": "agent"}, "path-id", false)
		require.NoError(t, err)
		require.Equal(t, "path-id", id)
	})
	t.Run("Should fail when id required but missing", func(t *testing.T) {
		_, err := validateBody(resources.ResourceAgent, map[string]any{"type": "agent"}, "", true)
		require.ErrorIs(t, err, ErrMissingID)
	})
}

func TestValidateTypedResource_ByType(t *testing.T) {
	t.Run("Should accept agent/tool/mcp/workflow/project/model minimal bodies", func(t *testing.T) {
		require.NoError(
			t,
			validateTypedResource(
				resources.ResourceAgent,
				map[string]any{"id": "a", "type": "agent", "instructions": "x"},
			),
		)
		require.NoError(t, validateTypedResource(resources.ResourceTool, map[string]any{"id": "t", "type": "tool"}))
		require.NoError(t, validateTypedResource(resources.ResourceMCP, map[string]any{"id": "m", "type": "mcp"}))
		require.NoError(
			t,
			validateTypedResource(resources.ResourceWorkflow, map[string]any{"id": "w", "type": "workflow"}),
		)
		require.NoError(
			t,
			validateTypedResource(resources.ResourceProject, map[string]any{"name": "p", "version": "1.0"}),
		)
		require.NoError(
			t,
			validateTypedResource(
				resources.ResourceModel,
				map[string]any{"provider": "openai", "model": "gpt-4o-mini"},
			),
		)
	})
	t.Run("Should validate memory with required fields and reject invalid", func(t *testing.T) {
		ok := map[string]any{
			"resource":    "memory",
			"id":          "mem",
			"type":        "memory",
			"persistence": map[string]any{"type": "in_memory", "ttl": "1h"},
		}
		require.NoError(t, validateTypedResource(resources.ResourceMemory, ok))
		bad := map[string]any{"id": "mem", "type": "memory"}
		require.ErrorIs(t, validateTypedResource(resources.ResourceMemory, bad), ErrInvalidPayload)
	})
}
