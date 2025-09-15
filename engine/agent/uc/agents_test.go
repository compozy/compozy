package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAgent_Execute(t *testing.T) {
	t.Parallel()
	t.Run("Should return agent when ID exists across workflows", func(t *testing.T) {
		wf1 := &workflow.Config{Agents: []agent.Config{{ID: "a1"}}}
		wf2 := &workflow.Config{Agents: []agent.Config{{ID: "a2"}}}
		usecase := NewGetAgent([]*workflow.Config{wf1, wf2}, "a2")
		got, err := usecase.Execute(context.TODO())
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "a2", got.ID)
	})

	t.Run("Should return error when agent is not found", func(t *testing.T) {
		wf := &workflow.Config{Agents: []agent.Config{{ID: "a1"}}}
		usecase := NewGetAgent([]*workflow.Config{wf}, "missing")
		got, err := usecase.Execute(context.TODO())
		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent not found")
	})
}

func TestListAgents_Execute(t *testing.T) {
	t.Parallel()
	t.Run("Should return unique agents across workflows", func(t *testing.T) {
		wf1 := &workflow.Config{Agents: []agent.Config{{ID: "a1"}, {ID: "a2"}}}
		wf2 := &workflow.Config{Agents: []agent.Config{{ID: "a2"}, {ID: "a3"}}}
		usecase := NewListAgents([]*workflow.Config{wf1, wf2})
		got, err := usecase.Execute(context.TODO())
		require.NoError(t, err)
		require.Len(t, got, 3)
		ids := map[string]bool{}
		for i := range got {
			ids[got[i].ID] = true
		}
		assert.True(t, ids["a1"])
		assert.True(t, ids["a2"])
		assert.True(t, ids["a3"])
	})
}
