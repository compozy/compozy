package core

import (
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
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

func TestMemoryExplicitSearchFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should search only inside the selected profile store", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		store := memory.NewStore(
			homePaths.MemoryDir,
			memory.WithCatalogDatabasePath(homePaths.DatabaseFile),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.OpenCatalog(t.Context()); err != nil {
			t.Fatalf("Store.OpenCatalog() error = %v", err)
		}
		marketingStore := store.ForProfile(
			"profile-marketing",
			filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.MemoryDirName),
		)
		if err := marketingStore.EnsureDirs(); err != nil {
			t.Fatalf("marketing Store.EnsureDirs() error = %v", err)
		}
		for _, fixture := range []struct {
			store    *memory.Store
			filename string
			name     string
		}{
			{store: store, filename: "default.md", name: "Default fallback sentinel"},
			{store: marketingStore, filename: "marketing.md", name: "Marketing fallback sentinel"},
		} {
			contents := []byte(
				"---\nname: " + fixture.name + "\ntype: user\ndescription: fallback sentinel\n---\nprofile fallback sentinel",
			)
			if err := fixture.store.Write(
				t.Context(),
				memcontract.ScopeProfile,
				fixture.filename,
				contents,
			); err != nil {
				t.Fatalf("Store.Write(%s) error = %v", fixture.filename, err)
			}
		}

		handlers := &BaseHandlers{MemoryStore: store, HomePaths: homePaths}
		results, err := handlers.memoryExplicitSearchFallback(t.Context(), memorySelector{
			ProfileID: "profile-marketing", ProfileName: "marketing", Scope: memcontract.ScopeProfile,
		}, "fallback sentinel", 10)
		if err != nil {
			t.Fatalf("memoryExplicitSearchFallback() error = %v", err)
		}
		if len(results) != 1 || results[0].Filename != "marketing.md" {
			t.Fatalf("memoryExplicitSearchFallback() = %#v, want only marketing.md", results)
		}
	})
}
