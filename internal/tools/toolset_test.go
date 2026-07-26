package tools

import (
	"strings"
	"testing"
)

func TestToolsetCatalogExpansion(t *testing.T) {
	t.Parallel()

	universe := []ToolID{"compozy__skill_search", "compozy__skill_view", "compozy__task_read"}

	t.Run("Should expand nested toolsets into deterministic concrete atoms", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(
			Toolset{ID: "compozy__skills", Tools: []string{"compozy__skill_*"}},
			Toolset{
				ID:       "compozy__read_bundle",
				Tools:    []string{"compozy__task_read"},
				Toolsets: []ToolsetID{"compozy__skills"},
			},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		expanded, err := catalog.Expand("compozy__read_bundle", universe)
		if err != nil {
			t.Fatalf("ToolsetCatalog.Expand() error = %v", err)
		}
		const want = "compozy__skill_search,compozy__skill_view,compozy__task_read"
		if got := joinToolIDs(expanded); got != want {
			t.Fatalf("expanded toolset = %s, want %s", got, want)
		}
	})

	t.Run("Should match concrete tools through nested wildcard toolsets", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(
			Toolset{ID: "compozy__tasks", Tools: []string{"compozy__task_*"}},
			Toolset{ID: "compozy__worker", Toolsets: []ToolsetID{"compozy__tasks"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		matched, err := catalog.Contains("compozy__task_read", []ToolsetID{"compozy__worker"})
		if err != nil {
			t.Fatalf("ToolsetCatalog.Contains(task) error = %v", err)
		}
		if !matched {
			t.Fatal("ToolsetCatalog.Contains(task) = false, want true")
		}
		matched, err = catalog.Contains("compozy__session_list", []ToolsetID{"compozy__worker"})
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
			Toolset{ID: "compozy__alpha_match", Tools: []string{"compozy__task_read"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		matched, err := catalog.Contains(
			"compozy__task_read",
			[]ToolsetID{"compozy__z_missing", "compozy__alpha_match"},
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
			Toolset{ID: "compozy__alpha", Toolsets: []ToolsetID{"compozy__beta"}},
			Toolset{ID: "compozy__beta", Toolsets: []ToolsetID{"compozy__alpha"}},
		)
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		_, err = catalog.Expand("compozy__alpha", universe)
		requireReason(t, err, ReasonToolsetCycle)
		if !strings.Contains(err.Error(), "compozy__alpha -> compozy__beta -> compozy__alpha") {
			t.Fatalf("ToolsetCatalog.Expand() error = %v, want deterministic cycle path", err)
		}
		_, err = catalog.Contains("compozy__task_read", []ToolsetID{"compozy__alpha"})
		requireReason(t, err, ReasonToolsetCycle)
	})

	t.Run("Should reject unknown nested toolsets", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(Toolset{ID: "compozy__root", Toolsets: []ToolsetID{"compozy__missing"}})
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		_, err = catalog.Expand("compozy__root", universe)
		requireReason(t, err, ReasonToolsetUnknown)
	})

	t.Run("Should reject unknown concrete members", func(t *testing.T) {
		t.Parallel()

		catalog, err := NewToolsetCatalog(Toolset{ID: "compozy__root", Tools: []string{"compozy__missing_tool"}})
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		_, err = catalog.Expand("compozy__root", universe)
		requireReason(t, err, ReasonToolUnknown)
	})

	t.Run("Should reject invalid policy patterns deterministically", func(t *testing.T) {
		t.Parallel()

		_, err := NewToolsetCatalog(Toolset{ID: "compozy__root", Tools: []string{"*__search"}})
		requireReason(t, err, ReasonIDInvalidFormat)
	})
}

func TestToolPatternParsing(t *testing.T) {
	t.Parallel()

	t.Run("Should parse exact and wildcard patterns", func(t *testing.T) {
		t.Parallel()

		patterns, err := ParseToolPatterns([]string{"compozy__skill_view", "mcp__github__*"})
		if err != nil {
			t.Fatalf("ParseToolPatterns() error = %v", err)
		}
		if !patterns[0].Match("compozy__skill_view") || patterns[0].String() != "compozy__skill_view" {
			t.Fatalf("exact pattern = %#v, want matching string pattern", patterns[0])
		}
		if !patterns[1].Match("mcp__github__search") || patterns[1].Match("mcp__linear__search") {
			t.Fatalf("wildcard pattern = %#v, want github-only match", patterns[1])
		}
	})

	t.Run("Should reject malformed wildcard placement", func(t *testing.T) {
		t.Parallel()

		_, err := ParseToolPattern("compozy__*__view")
		requireReason(t, err, ReasonIDInvalidFormat)
		_, err = ParseToolPattern("compozy__skill*view")
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
