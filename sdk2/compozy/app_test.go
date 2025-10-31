package compozy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineproject "github.com/compozy/compozy/engine/project"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
)

func TestNew_SuccessWithWorkflow(t *testing.T) {
	t.Parallel()
	workflowCfg := &engineworkflow.Config{ID: "greeting"}
	engine, err := New(t.Context(), WithWorkflow(workflowCfg))
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.Same(t, workflowCfg, engine.workflows[0])
	assert.Equal(t, ModeStandalone, engine.mode)
	assert.Equal(t, defaultHost, engine.host)
}

func TestNew_SuccessWithProjectOnly(t *testing.T) {
	t.Parallel()
	projectCfg := &engineproject.Config{Name: "demo"}
	engine, err := New(t.Context(), WithProject(projectCfg), WithHost(" "), WithPort(9090), WithMode(ModeDistributed))
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.Same(t, projectCfg, engine.project)
	assert.Equal(t, ModeDistributed, engine.mode)
	assert.Equal(t, defaultHost, engine.host)
	assert.Equal(t, 9090, engine.port)
}

func TestNew_ErrorNilContext(t *testing.T) {
	t.Parallel()
	_, err := New(nil, WithPort(8080))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context is required")
}

func TestNew_ErrorNoResources(t *testing.T) {
	t.Parallel()
	_, err := New(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one resource")
}
