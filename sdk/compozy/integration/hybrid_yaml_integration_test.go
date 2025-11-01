//go:build integration
// +build integration

package compozy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	enginecore "github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHybridYAMLIntegration(t *testing.T) {
	t.Run("Should validate hybrid YAML integration", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		engine, err := New(ctx, WithWorkflow(programmaticWorkflow()))
		require.NoError(t, err)
		dir := t.TempDir()
		workflowDir := filepath.Join(dir, "workflows")
		require.NoError(t, os.MkdirAll(workflowDir, 0o755))
		yamlWorkflow := strings.TrimSpace(`id: yaml-workflow
tasks:
  - id: entry
    final: true
    tool:
      resource: tool
      id: yaml-tool
      type: http
`)
		file := filepath.Join(workflowDir, "workflow.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yamlWorkflow), 0o600))
		require.NoError(t, engine.LoadWorkflowsFromDir(ctx, workflowDir))
		report, err := engine.ValidateReferences()
		require.NoError(t, err)
		assert.True(t, report.Valid)
	})
}

func programmaticWorkflow() *engineworkflow.Config {
	next := "finish"
	cfg := &engineworkflow.Config{ID: "programmatic"}
	cfg.Tasks = []enginetask.Config{
		{
			ID:   "start",
			Tool: &enginetool.Config{ID: "prog-tool", Resource: "tool", Type: "http"},
			OnSuccess: &enginecore.SuccessTransition{
				Next: &next,
			},
		},
		{
			ID:    "finish",
			Final: true,
			Tool:  &enginetool.Config{ID: "prog-tool", Resource: "tool", Type: "http"},
		},
	}
	return cfg
}
