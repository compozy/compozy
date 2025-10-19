package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
)

func TestWorkflowsValidator(t *testing.T) {
	t.Run("Should pass when no workflows provided", func(t *testing.T) {
		validator := NewWorkflowsValidator(nil, nil)
		err := validator.Validate(t.Context())
		require.NoError(t, err)
	})

	t.Run("Should fail when source is empty", func(t *testing.T) {
		validator := NewWorkflowsValidator(nil, []*WorkflowSourceConfig{{Source: ""}})
		err := validator.Validate(t.Context())
		require.ErrorContains(t, err, "source is empty")
	})

	t.Run("Should fail when workflow file is missing", func(t *testing.T) {
		cwd, err := core.CWDFromPath(t.TempDir())
		require.NoError(t, err)
		validator := NewWorkflowsValidator(cwd, []*WorkflowSourceConfig{{Source: "missing.yaml"}})
		err = validator.Validate(t.Context())
		require.ErrorContains(t, err, "not found")
	})

	t.Run("Should fail when source points to a directory", func(t *testing.T) {
		dir := t.TempDir()
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		err = os.Mkdir(filepath.Join(dir, "workflows"), 0o755)
		require.NoError(t, err)
		validator := NewWorkflowsValidator(cwd, []*WorkflowSourceConfig{{Source: "workflows"}})
		err = validator.Validate(t.Context())
		require.ErrorContains(t, err, "points to a directory")
	})

	t.Run("Should pass with valid workflow files", func(t *testing.T) {
		dir := t.TempDir()
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		file := filepath.Join(dir, "workflow.yaml")
		err = os.WriteFile(file, []byte("name: test"), 0o644)
		require.NoError(t, err)
		validator := NewWorkflowsValidator(cwd, []*WorkflowSourceConfig{{Source: "workflow.yaml"}})
		err = validator.Validate(t.Context())
		require.NoError(t, err)
	})

	t.Run("Should use stat cache within validation", func(t *testing.T) {
		dir := t.TempDir()
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		file := filepath.Join(dir, "workflow.yaml")
		err = os.WriteFile(file, []byte("name: test"), 0o644)
		require.NoError(t, err)
		validator := NewWorkflowsValidator(cwd, []*WorkflowSourceConfig{{Source: "workflow.yaml"}})
		calls := 0
		validator.statFn = func(path string) (fs.FileInfo, error) {
			calls++
			return os.Stat(path)
		}
		err = validator.Validate(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})
}

func TestWebhookSlugsValidator(t *testing.T) {
	t.Run("Should pass with unique slugs", func(t *testing.T) {
		v := NewWebhookSlugsValidator([]string{"a", "b", "c"})
		err := v.Validate(t.Context())
		require.NoError(t, err)
	})

	t.Run("Should ignore empty slugs", func(t *testing.T) {
		v := NewWebhookSlugsValidator([]string{"", "x", ""})
		err := v.Validate(t.Context())
		require.NoError(t, err)
	})

	t.Run("Should fail on duplicate slugs", func(t *testing.T) {
		v := NewWebhookSlugsValidator([]string{"dup", "x", "dup"})
		err := v.Validate(t.Context())
		require.ErrorContains(t, err, "duplicate webhook slug 'dup'")
	})
}
