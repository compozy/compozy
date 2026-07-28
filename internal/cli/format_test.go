package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHumanOutputProducesStyledTable(t *testing.T) {
	t.Parallel()

	t.Run("Should produce styled table", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listAgentsFn: func(_ context.Context, _ AgentQuery) ([]AgentRecord, error) {
				return []AgentRecord{{
					Name:        "coder",
					Provider:    "codex",
					Model:       "gpt-5.4",
					Tools:       []string{"shell", "edit"},
					Permissions: "approve-reads",
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "human")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if !strings.Contains(stdout, "Agents") || !strings.Contains(stdout, "Provider") ||
			!strings.Contains(stdout, "----") {
			t.Fatalf("human output = %q, want styled table", stdout)
		}
	})
}

func TestJSONOutputProducesValidJSON(t *testing.T) {
	t.Parallel()

	t.Run("Should produce valid JSON", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listSessionsFn: func(_ context.Context, _ SessionListQuery) ([]SessionRecord, error) {
				return []SessionRecord{{
					ID:            "sess-1",
					Name:          "demo",
					AgentName:     "coder",
					WorkspaceID:   "ws-1",
					WorkspacePath: "/workspace/project",
					State:         "active",
					CreatedAt:     fixedTestNow.Add(-time.Minute),
					UpdatedAt:     fixedTestNow,
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "list", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}

		var decoded SessionListPage
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(decoded.Sessions) != 1 || decoded.Sessions[0].ID != "sess-1" || decoded.Page.Total != 1 {
			t.Fatalf("decoded = %#v, want one session", decoded)
		}
	})
}

func TestToonOutputProducesToonDocument(t *testing.T) {
	t.Parallel()

	t.Run("Should produce TOON document", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listAgentsFn: func(_ context.Context, _ AgentQuery) ([]AgentRecord, error) {
				return []AgentRecord{{Name: "coder", Provider: "codex", Tools: []string{"shell"}}}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "toon")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if !strings.Contains(
			stdout,
			"agents[1]{name,provider,model,category,origin,workspace_id,disabled_skills,definition_digest,tool_count,permissions}:",
		) {
			t.Fatalf("toon output = %q, want TOON header", stdout)
		}
	})
}

func TestAgentCategoryLabel(t *testing.T) {
	t.Parallel()

	t.Run("Should render category path with single space delimiters", func(t *testing.T) {
		t.Parallel()

		if got := agentCategoryLabel([]string{"Marketing", "Sales"}); got != "Marketing / Sales" {
			t.Fatalf("agentCategoryLabel() = %q, want %q", got, "Marketing / Sales")
		}
	})
}

func TestHumanFormattingUsesRuneWidths(t *testing.T) {
	t.Parallel()

	t.Run("Should size human section underlines with rune widths", func(t *testing.T) {
		t.Parallel()

		rendered, err := renderHumanSectionResult("Agênts", []keyValue{{Label: "Status", Value: "ready"}})
		if err != nil {
			t.Fatalf("renderHumanSectionResult() error = %v", err)
		}

		lines := strings.Split(rendered, "\n")
		if len(lines) < 2 {
			t.Fatalf("renderHumanSectionResult() lines = %#v, want title and underline", lines)
		}
		if got, want := lines[1], strings.Repeat("=", humanTableCellWidth("Agênts")); got != want {
			t.Fatalf("section underline = %q, want %q", got, want)
		}
	})

	t.Run("Should size human table title and separators with rune widths", func(t *testing.T) {
		t.Parallel()

		rendered := renderHumanTable("Agênts", []string{"Náme", "Status"}, [][]string{{"bot", "ready"}})
		lines := strings.Split(rendered, "\n")
		if len(lines) < 4 {
			t.Fatalf("renderHumanTable() lines = %#v, want title, underline, header, and separator", lines)
		}
		if got, want := lines[1], strings.Repeat("=", humanTableCellWidth("Agênts")); got != want {
			t.Fatalf("table underline = %q, want %q", got, want)
		}
		if got, want := lines[3], "----  ------"; got != want {
			t.Fatalf("table separator = %q, want %q", got, want)
		}
	})
}
