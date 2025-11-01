package compozy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineproject "github.com/compozy/compozy/engine/project"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
)

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("Should construct engine with workflow", func(t *testing.T) {
		t.Parallel()
		workflowCfg := &engineworkflow.Config{ID: "greeting"}
		engine, err := New(t.Context(), WithWorkflow(workflowCfg))
		require.NoError(t, err)
		require.NotNil(t, engine)
		require.Len(t, engine.workflows, 1)
		assert.NotSame(t, workflowCfg, engine.workflows[0])
		assert.Equal(t, workflowCfg, engine.workflows[0])
		assert.Equal(t, ModeStandalone, engine.mode)
		assert.Equal(t, defaultHost, engine.host)
	})
	t.Run("Should honor project overrides", func(t *testing.T) {
		t.Parallel()
		projectCfg := &engineproject.Config{Name: "demo"}
		engine, err := New(
			t.Context(),
			WithProject(projectCfg),
			WithHost(" "),
			WithPort(9090),
			WithMode(ModeDistributed),
		)
		require.NoError(t, err)
		require.NotNil(t, engine)
		assert.Same(t, projectCfg, engine.project)
		assert.Equal(t, ModeDistributed, engine.mode)
		assert.Equal(t, defaultHost, engine.host)
		assert.Equal(t, 9090, engine.port)
	})
	t.Run("Should error when context is nil", func(t *testing.T) {
		t.Parallel()
		var nilCtx context.Context
		_, err := New(nilCtx, WithPort(8080))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})
	t.Run("Should require at least one resource", func(t *testing.T) {
		t.Parallel()
		_, err := New(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one resource")
	})
}
