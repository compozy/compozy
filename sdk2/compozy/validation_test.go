package compozy

import (
	"testing"

	enginecore "github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReferencesDetectsMissingTask(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	wf := &engineworkflow.Config{ID: "missing"}
	next := "ghost"
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "start",
				OnSuccess: &enginecore.SuccessTransition{Next: &next},
			},
		},
	}
	require.NoError(t, engine.RegisterWorkflow(wf))
	report, err := engine.ValidateReferences()
	require.NoError(t, err)
	assert.False(t, report.Valid)
	require.NotEmpty(t, report.MissingRefs)
	assert.Contains(t, report.MissingRefs[0].Reference, "ghost")
}

func TestValidateReferencesDetectsCycle(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	wf := &engineworkflow.Config{ID: "cycle"}
	nextB := "b"
	nextA := "a"
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "a",
				OnSuccess: &enginecore.SuccessTransition{Next: &nextB},
			},
		},
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "b",
				OnSuccess: &enginecore.SuccessTransition{Next: &nextA},
			},
		},
	}
	require.NoError(t, engine.RegisterWorkflow(wf))
	report, err := engine.ValidateReferences()
	require.NoError(t, err)
	assert.False(t, report.Valid)
	require.NotEmpty(t, report.CircularDeps)
	assert.Greater(t, len(report.CircularDeps[0].Chain), 0)
}

func TestRegisterWorkflowPreventsDuplicateIDs(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	wf := &engineworkflow.Config{ID: "dup"}
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{ID: "only"},
		},
	}
	require.NoError(t, engine.RegisterWorkflow(wf))
	err := engine.RegisterWorkflow(wf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}
