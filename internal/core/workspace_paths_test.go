package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestResolveSyncTargetUsesWorkspaceRootWhenRootDirIsEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	tasksRoot := filepath.Join(root, model.TasksBaseDir())
	workflowDir := filepath.Join(tasksRoot, "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	target, singleWorkflow, err := resolveSyncTarget(SyncConfig{WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("resolveSyncTarget: %v", err)
	}
	if singleWorkflow {
		t.Fatal("expected workspace root sync target to scan the whole tasks root")
	}
	if target != tasksRoot {
		t.Fatalf("unexpected sync target\nwant: %q\ngot:  %q", tasksRoot, target)
	}
}
