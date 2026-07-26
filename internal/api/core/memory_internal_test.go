package core

import (
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func TestMemorySearchResultPayloadsFromSearchResults(t *testing.T) {
	t.Parallel()

	t.Run("Should return an empty JSON array payload for no fallback search results", func(t *testing.T) {
		t.Parallel()

		results := memorySearchResultPayloadsFromSearchResults(nil, "ws_alpha")
		if results == nil {
			t.Fatal("memorySearchResultPayloadsFromSearchResults() = nil, want empty slice")
		}
		if len(results) != 0 {
			t.Fatalf("len(results) = %d, want 0", len(results))
		}
	})

	t.Run("Should use the canonical workspace ID instead of the fallback workspace path", func(t *testing.T) {
		t.Parallel()

		modTime := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
		results := memorySearchResultPayloadsFromSearchResults([]memcontract.SearchResult{{
			Filename:    "workspace.md",
			Scope:       memcontract.ScopeWorkspace,
			Workspace:   "/tmp/workspace-root",
			Type:        memcontract.TypeProject,
			Name:        "Workspace",
			Description: "Fallback result",
			Score:       0.75,
			Snippet:     "workspace memory",
			ModTime:     modTime,
		}}, "ws_alpha")

		if got, want := len(results), 1; got != want {
			t.Fatalf("len(results) = %d, want %d", got, want)
		}
		if got, want := results[0].Memory.WorkspaceID, "ws_alpha"; got != want {
			t.Fatalf("results[0].Memory.WorkspaceID = %q, want %q", got, want)
		}
		if got, want := results[0].Memory.ModTime, modTime; !got.Equal(want) {
			t.Fatalf("results[0].Memory.ModTime = %v, want %v", got, want)
		}
	})
}
