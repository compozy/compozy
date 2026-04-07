package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestResolveWorkflowTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	tasksRoot := filepath.Join(root, model.TasksBaseDir())
	workflowDir := filepath.Join(tasksRoot, "demo")
	reviewsDir := filepath.Join(workflowDir, "reviews-001")
	for _, dir := range []string{tasksRoot, workflowDir, reviewsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	tests := []struct {
		name            string
		cfg             workflowTargetOptions
		wantTarget      string
		wantRoot        string
		wantSingleScope bool
	}{
		{
			name: "sync workspace root",
			cfg: workflowTargetOptions{
				command:       "sync",
				workspaceRoot: root,
				selectorFlags: "--name or --tasks-dir",
			},
			wantTarget:      tasksRoot,
			wantRoot:        tasksRoot,
			wantSingleScope: false,
		},
		{
			name: "archive named workflow",
			cfg: workflowTargetOptions{
				command:       "archive",
				workspaceRoot: root,
				name:          "demo",
				selectorFlags: "--name or --tasks-dir",
			},
			wantTarget:      workflowDir,
			wantRoot:        tasksRoot,
			wantSingleScope: true,
		},
		{
			name: "migrate reviews dir",
			cfg: workflowTargetOptions{
				command:       "migrate",
				workspaceRoot: root,
				reviewsDir:    reviewsDir,
				selectorFlags: "--name, --tasks-dir, or --reviews-dir",
			},
			wantTarget:      reviewsDir,
			wantRoot:        workflowDir,
			wantSingleScope: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveWorkflowTarget(tt.cfg)
			if err != nil {
				t.Fatalf("resolveWorkflowTarget: %v", err)
			}
			if got.target != tt.wantTarget {
				t.Fatalf("unexpected target\nwant: %q\ngot:  %q", tt.wantTarget, got.target)
			}
			if got.rootDir != tt.wantRoot {
				t.Fatalf("unexpected root dir\nwant: %q\ngot:  %q", tt.wantRoot, got.rootDir)
			}
			if got.specificTarget != tt.wantSingleScope {
				t.Fatalf(
					"unexpected specificTarget\nwant: %t\ngot:  %t",
					tt.wantSingleScope,
					got.specificTarget,
				)
			}
		})
	}
}
