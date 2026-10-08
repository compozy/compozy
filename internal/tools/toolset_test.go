package tools

import (
	"slices"
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

func TestDropRetiredToolReferences(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve surviving tools and toolsets after dropping retired references", func(t *testing.T) {
		t.Parallel()
		policy := ToolPolicy{Toolsets: []string{"compozy__sessions", "compozy__memory"}, Tools: []string{"compozy__memory_list", "compozy__session_list"}}
		filtered, dropped := DropRetiredToolReferences(policy)
		if !slices.Equal(filtered.Toolsets, []string{"compozy__sessions"}) || !slices.Equal(filtered.Tools, []string{"compozy__session_list"}) || !slices.Equal(dropped, []string{"compozy__memory", "compozy__memory_list"}) {
			t.Fatalf("DropRetiredToolReferences() = %#v, %#v", filtered, dropped)
		}
		catalog, err := NewToolsetCatalog(Toolset{ID: "compozy__sessions", Tools: []string{"compozy__session_list"}})
		if err != nil {
			t.Fatal(err)
		}
		patterns, err := ParseToolPatterns(filtered.Tools)
		if err != nil {
			t.Fatal(err)
		}
		ids, err := catalog.ExpandPatterns(patterns, []ToolsetID{ToolsetID(filtered.Toolsets[0])}, []ToolID{"compozy__session_list"})
		if err != nil || !slices.Equal(ids, []ToolID{"compozy__session_list"}) {
			t.Fatalf("ExpandPatterns() = %#v, %v", ids, err)
		}
		filtered.Tools[0] = "compozy__changed"
		if policy.Tools[1] != "compozy__session_list" {
			t.Fatalf("input mutated: %#v", policy)
		}
	})
	t.Run("Should drop every retired catalog atom without treating unknown IDs or wildcards as retired", func(t *testing.T) {
		t.Parallel()
		policy := ToolPolicy{Tools: []string{"compozy__typo", "compozy__*"}}
		for _, id := range RetiredMemoryToolIDs {
			policy.Tools = append(policy.Tools, id.String())
		}
		for _, id := range RetiredMemoryToolsetIDs {
			policy.Toolsets = append(policy.Toolsets, id.String())
		}
		policy.DenyTools = []string{"compozy__memory_list", "compozy__task_*"}
		filtered, dropped := DropRetiredToolReferences(policy)
		if len(dropped) != 37 || !slices.Equal(filtered.Tools, []string{"compozy__typo", "compozy__*"}) || len(filtered.Toolsets) != 0 || !slices.Equal(filtered.DenyTools, []string{"compozy__task_*"}) {
			t.Fatalf("filtered = %#v, dropped = %#v", filtered, dropped)
		}
		patterns, err := ParseToolPatterns(filtered.Tools)
		if err != nil {
			t.Fatal(err)
		}
		_, err = (ToolsetCatalog{}).ExpandPatterns(patterns, nil, []ToolID{"compozy__session_list"})
		if err == nil || !strings.Contains(err.Error(), "unknown tool") {
			t.Fatalf("unknown policy error = %v", err)
		}
		again, dropped := DropRetiredToolReferences(filtered)
		if len(dropped) != 0 || !slices.Equal(again.Tools, filtered.Tools) {
			t.Fatalf("repeat filter = %#v, %#v", again, dropped)
		}
	})
	t.Run("Should preserve an enforced empty policy when every allowed tool is retired", func(t *testing.T) {
		t.Parallel()
		retired, err := ParseToolPatterns([]string{"compozy__memory_list"})
		if err != nil {
			t.Fatal(err)
		}

		for _, inputs := range []PolicyInputs{
			{SystemPermissionMode: PermissionModeApproveAll, Agent: AgentToolPolicy{Tools: retired, Toolsets: []ToolsetID{"compozy__memory"}}},
			{SystemPermissionMode: PermissionModeApproveAll, Session: SessionToolPolicy{Enforced: true, Tools: []ToolID{"compozy__memory_list"}}},
		} {
			evaluator, err := NewEffectivePolicyEvaluator(inputs, ToolsetCatalog{}, []ToolID{"compozy__session_list"})
			if err != nil {
				t.Fatal(err)
			}
			decision, err := evaluator.Evaluate(t.Context(), Scope{}, descriptorWithID("compozy__session_list", "List sessions"))
			if err != nil {
				t.Fatal(err)
			}
			if decision.Callable || decision.VisibleToSession {
				t.Fatalf("retired-only policy granted an unrelated tool: %#v", decision)
			}
			if !slices.Contains(decision.ReasonCodes, ReasonPolicyDenied) && !slices.Contains(decision.ReasonCodes, ReasonSessionDenied) {
				t.Fatalf("unexpected denial reason: %#v", decision)
			}
		}

	})

}
