//go:build integration

package memory_test

import (
	"path/filepath"
	"testing"

	memorypkg "github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

// Canonical integration suite: worktree-bound and root-bound sessions share one workspace memory brain.
func TestWorkspaceMemoryBrainAcrossSessionEnvironments(t *testing.T) {
	t.Parallel()
	t.Run("Should share workspace memory across root and worktree sessions", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		base := memorypkg.NewStore(filepath.Join(t.TempDir(), "global-memory"))
		boundSessionStore := base.ForWorkspace(workspaceRoot)
		rootSessionStore := base.ForWorkspace(workspaceRoot)
		if err := boundSessionStore.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs(bound session store) error = %v", err)
		}

		const filename = "shared-workspace-memory.md"
		boundPayload := []byte(
			"---\nname: Shared workspace memory\ntype: project\n---\nWritten from a bound session.\n",
		)
		if err := boundSessionStore.Write(t.Context(), memcontract.ScopeWorkspace, filename, boundPayload); err != nil {
			t.Fatalf("Write(bound session store) error = %v", err)
		}
		readFromRoot, err := rootSessionStore.Read(t.Context(), memcontract.ScopeWorkspace, filename)
		if err != nil {
			t.Fatalf("Read(root session store) error = %v", err)
		}
		if string(readFromRoot) != string(boundPayload) {
			t.Fatalf("root session memory = %q, want %q", readFromRoot, boundPayload)
		}

		rootPayload := []byte(
			"---\nname: Root workspace memory\ntype: project\n---\nWritten from the root session.\n",
		)
		if err := rootSessionStore.Write(t.Context(), memcontract.ScopeWorkspace, filename, rootPayload); err != nil {
			t.Fatalf("Write(root session store) error = %v", err)
		}
		readFromBound, err := boundSessionStore.Read(t.Context(), memcontract.ScopeWorkspace, filename)
		if err != nil {
			t.Fatalf("Read(bound session store) error = %v", err)
		}
		if string(readFromBound) != string(rootPayload) {
			t.Fatalf("bound session memory = %q, want %q", readFromBound, rootPayload)
		}
	})
}
