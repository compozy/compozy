package tools

import (
	"strings"
	"testing"
)

func TestToolsetCatalogExpansion(t *testing.T) {
	t.Parallel()

	universe := []ToolID{"agh__skill_search", "agh__skill_view", "agh__task_read"}

	t.Run("Should expand nested toolsets into deterministic concrete atoms", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(
			Toolset{ID: "agh__skills", Tools: []string{"agh__skill_*"}},
			Toolset{ID: "agh__read_bundle", Tools: []string{"agh__task_read"}, Toolsets: []ToolsetID{"agh__skills"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		expanded, err := catalog.Expand("agh__read_bundle", universe)
		if err != nil {
			t.Fatalf("ToolsetCatalog.Expand() error = %v", err)
		}
		if got, want := joinToolIDs(expanded), "agh__skill_search,agh__skill_view,agh__task_read"; got != want {
			t.Fatalf("expanded toolset = %s, want %s", got, want)
		}
	})

	t.Run("Should match concrete tools through nested wildcard toolsets", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(
			Toolset{ID: "agh__tasks", Tools: []string{"agh__task_*"}},
			Toolset{ID: "agh__worker", Toolsets: []ToolsetID{"agh__tasks"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		matched, err := catalog.Contains("agh__task_read", []ToolsetID{"agh__worker"})
		if err != nil {
			t.Fatalf("ToolsetCatalog.Contains(task) error = %v", err)
		}
		if !matched {
			t.Fatal("ToolsetCatalog.Contains(task) = false, want true")
		}
		matched, err = catalog.Contains("agh__session_list", []ToolsetID{"agh__worker"})
		if err != nil {
			t.Fatalf("ToolsetCatalog.Contains(session) error = %v", err)
		}
		if matched {
			t.Fatal("ToolsetCatalog.Contains(session) = true, want false")
		}
	})

	t.Run("Should stop after a matching toolset before resolving later invalid IDs", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(
			Toolset{ID: "agh__alpha_match", Tools: []string{"agh__task_read"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		matched, err := catalog.Contains(
			"agh__task_read",
			[]ToolsetID{"agh__z_missing", "agh__alpha_match"},
		)
		if err != nil {
			t.Fatalf("ToolsetCatalog.Contains() error = %v", err)
		}
		if !matched {
			t.Fatal("ToolsetCatalog.Contains() = false, want true")
		}
	})

	t.Run("Should reject recursive toolset cycles", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(
			Toolset{ID: "agh__alpha", Toolsets: []ToolsetID{"agh__beta"}},
			Toolset{ID: "agh__beta", Toolsets: []ToolsetID{"agh__alpha"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		_, err = catalog.Expand("agh__alpha", universe)
		requireReason(t, err, ReasonToolsetCycle)
		if !strings.Contains(err.Error(), "agh__alpha -> agh__beta -> agh__alpha") {
			t.Fatalf("ToolsetCatalog.Expand() error = %v, want deterministic cycle path", err)
		}
		_, err = catalog.Contains("agh__task_read", []ToolsetID{"agh__alpha"})
		requireReason(t, err, ReasonToolsetCycle)
	})

	t.Run("Should reject unknown nested toolsets", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(Toolset{ID: "agh__root", Toolsets: []ToolsetID{"agh__missing"}})
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		_, err = catalog.Expand("agh__root", universe)
		requireReason(t, err, ReasonToolsetUnknown)
	})

	t.Run("Should reject unknown concrete members", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(Toolset{ID: "agh__root", Tools: []string{"agh__missing_tool"}})
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		_, err = catalog.Expand("agh__root", universe)
		requireReason(t, err, ReasonToolUnknown)
	})

	t.Run("Should reject invalid policy patterns deterministically", func(t *testing.T) {
		t.Parallel()

		_, err := NewToolsetCatalog(Toolset{ID: "agh__root", Tools: []string{"*__search"}})
		requireReason(t, err, ReasonIDInvalidFormat)
	})
}

func TestToolPatternParsing(t *testing.T) {
	t.Parallel()

	t.Run("Should parse exact and wildcard patterns", func(t *testing.T) {
		t.Parallel()

		patterns, err := ParseToolPatterns([]string{"agh__skill_view", "mcp__github__*"})
		if err != nil {
			t.Fatalf("ParseToolPatterns() error = %v", err)
		}
		if !patterns[0].Match("agh__skill_view") || patterns[0].String() != "agh__skill_view" {
			t.Fatalf("exact pattern = %#v, want matching string pattern", patterns[0])
		}
		if !patterns[1].Match("mcp__github__search") || patterns[1].Match("mcp__linear__search") {
			t.Fatalf("wildcard pattern = %#v, want github-only match", patterns[1])
		}
	})

	t.Run("Should reject malformed wildcard placement", func(t *testing.T) {
		t.Parallel()

		_, err := ParseToolPattern("agh__*__view")
		requireReason(t, err, ReasonIDInvalidFormat)
		_, err = ParseToolPattern("agh__skill*view")
		requireReason(t, err, ReasonIDInvalidFormat)
	})
}

func joinToolIDs(ids []ToolID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id.String())
	}
	return strings.Join(parts, ",")
}
