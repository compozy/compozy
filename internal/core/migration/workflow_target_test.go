package migration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkflowTargetRejectsEscapingWorkflowNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rootDir := filepath.Join(root, ".compozy", "tasks")

	cases := []struct {
		name         string
		workflowName string
	}{
		{name: "Should reject parent directory traversal", workflowName: "../other"},
		{name: "Should reject nested workflow paths", workflowName: "demo/nested"},
		{name: "Should reject absolute workflow paths", workflowName: filepath.Join(root, "elsewhere")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := resolveWorkflowTarget(workflowTargetOptions{
				command:       "migrate",
				workspaceRoot: root,
				rootDir:       rootDir,
				name:          tc.workflowName,
				selectorFlags: "--name",
			})
			if err == nil {
				t.Fatal("expected invalid workflow name error")
			}
			if !strings.Contains(err.Error(), "single active workflow directory name") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
