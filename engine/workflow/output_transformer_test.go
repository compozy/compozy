package workflow

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNormCtx struct{ base map[string]any }

func (t *testNormCtx) BuildTemplateContext(_ context.Context) map[string]any { return t.base }

func TestOutputNormalizer_TransformWorkflowOutput(t *testing.T) {
	t.Run("Should return nil when no outputs configured", func(t *testing.T) {
		eng := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := NewOutputNormalizer(eng)
		st := &State{WorkflowID: "wf", WorkflowExecID: core.MustNewID(), Status: core.StatusSuccess}
		out, err := normalizer.TransformWorkflowOutput(
			t.Context(),
			st,
			nil,
			&testNormCtx{base: map[string]any{"name": "Ada"}},
		)
		require.NoError(t, err)
		assert.Nil(t, out)
		empty := core.Output{}
		out, err = normalizer.TransformWorkflowOutput(
			t.Context(),
			st,
			&empty,
			&testNormCtx{base: map[string]any{"name": "Ada"}},
		)
		require.NoError(t, err)
		assert.Nil(t, out)
	})
	t.Run("Should apply templates with workflow context and state fields", func(t *testing.T) {
		eng := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := NewOutputNormalizer(eng)
		execID := core.MustNewID()
		st := &State{WorkflowID: "wf-123", WorkflowExecID: execID, Status: core.StatusSuccess}
		outputs := core.Output{
			"msg":         "Hello, {{ .name }}",
			"status_copy": "{{ .status }}",
			"exec":        "{{ .workflow_exec_id }}",
			"static":      42,
			"nested":      map[string]any{"greet": "Hi {{ .name }}"},
		}
		ctx := &testNormCtx{base: map[string]any{"name": "Grace"}}
		got, err := normalizer.TransformWorkflowOutput(t.Context(), st, &outputs, ctx)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "Hello, Grace", (*got)["msg"])
		assert.Equal(t, core.StatusSuccess, (*got)["status_copy"])
		assert.Equal(t, execID, (*got)["exec"])
		assert.Equal(t, 42, (*got)["static"])
		nested, ok := (*got)["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Hi Grace", nested["greet"])
	})
	t.Run("Should return error when template references missing key", func(t *testing.T) {
		eng := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := NewOutputNormalizer(eng)
		st := &State{WorkflowID: "wf", WorkflowExecID: core.MustNewID(), Status: core.StatusSuccess}
		outputs := core.Output{"bad": "{{ .does_not_exist }}"}
		_, err := normalizer.TransformWorkflowOutput(
			t.Context(),
			st,
			&outputs,
			&testNormCtx{base: map[string]any{"ok": true}},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to transform workflow output field bad")
	})
}
