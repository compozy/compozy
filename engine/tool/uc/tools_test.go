package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTool_Execute(t *testing.T) {
	t.Parallel()
	t.Run("Should return tool when found in workflows", func(t *testing.T) {
		t.Parallel()
		wf := &workflow.Config{Tools: []tool.Config{{ID: "fmt", Description: "Formatter"}}}
		uc := NewGetTool([]*workflow.Config{wf}, "fmt")
		got, err := uc.Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "fmt", got.ID)
		assert.Equal(t, "Formatter", got.Description)
	})
	t.Run("Should return error when tool is not found", func(t *testing.T) {
		t.Parallel()
		wf := &workflow.Config{Tools: []tool.Config{{ID: "fmt"}}}
		uc := NewGetTool([]*workflow.Config{wf}, "lint")
		got, err := uc.Execute(context.Background())
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrToolNotFound))
		assert.Nil(t, got)
	})
	t.Run("Should deterministically return first match when duplicate IDs across workflows", func(t *testing.T) {
		t.Parallel()
		wf1 := &workflow.Config{Tools: []tool.Config{{ID: "dup", Description: "from wf1"}}}
		wf2 := &workflow.Config{Tools: []tool.Config{{ID: "dup", Description: "from wf2"}}}
		uc := NewGetTool([]*workflow.Config{wf1, wf2}, "dup")
		got, err := uc.Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "from wf1", got.Description)
	})
}

func TestListTools_Execute(t *testing.T) {
	t.Parallel()
	t.Run("Should aggregate unique tools across workflows", func(t *testing.T) {
		t.Parallel()
		wf1 := &workflow.Config{Tools: []tool.Config{{ID: "a"}, {ID: "b", Description: "from wf1"}}}
		wf2 := &workflow.Config{Tools: []tool.Config{{ID: "b", Description: "from wf2"}, {ID: "c"}}}
		uc := NewListTools([]*workflow.Config{wf1, wf2})
		tools, err := uc.Execute(context.Background())
		require.NoError(t, err)
		assert.Len(t, tools, 3)
		ids := map[string]tool.Config{}
		for i := range tools {
			ids[tools[i].ID] = tools[i]
		}
		assert.Contains(t, ids, "a")
		assert.Contains(t, ids, "b")
		assert.Contains(t, ids, "c")
		assert.Equal(t, "from wf1", ids["b"].Description)
	})
	t.Run("Should return empty slice when no workflows provided", func(t *testing.T) {
		t.Parallel()
		uc := NewListTools(nil)
		tools, err := uc.Execute(context.Background())
		require.NoError(t, err)
		assert.Empty(t, tools)
	})
}
