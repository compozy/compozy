package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestHumanOutputProducesStyledTable(t *testing.T) {
	t.Parallel()

	t.Run("Should produce styled table", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
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

		deps := newWorkspaceTestDeps(t, &stubClient{
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

		deps := newWorkspaceTestDeps(t, &stubClient{
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
			"agents[1]{name,provider,model,category,origin,workspace_id,disabled_skills,layer,shadows,definition_digest,tool_count,permissions}:",
		) {
			t.Fatalf("toon output = %q, want TOON header", stdout)
		}
	})
}

func TestWorkspaceResolutionFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
		items  []map[string]string
	}{
		{name: "Should annotate JSON output", format: "json", items: []map[string]string{{"id": "one"}}},
		{name: "Should preserve literal HTML characters", format: "json", items: []map[string]string{{"id": "<one>"}}},
		{name: "Should annotate populated JSONL output", format: "jsonl", items: []map[string]string{{"id": "one"}}},
		{name: "Should annotate empty JSONL output", format: "jsonl"},
		{name: "Should prefix TOON output", format: "toon", items: []map[string]string{{"id": "one"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "format-test"}
			cmd.SetContext(context.Background())
			cmd.Flags().String(outputFlagName, tt.format, "")
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			recordWorkspaceResolution(cmd, workspaceResolution{ID: "ws-1", Source: workspaceResolutionCWD})
			bundle := listBundle(
				map[string]any{"items": tt.items},
				tt.items,
				"Items",
				[]string{"ID"},
				"items",
				[]string{"id"},
				func(item map[string]string) []string { return []string{item["id"]} },
				func(item map[string]string) []string { return []string{item["id"]} },
			)
			if err := writeCommandOutput(cmd, bundle); err != nil {
				t.Fatalf("writeCommandOutput(%s) error = %v", tt.format, err)
			}
			if !strings.Contains(stdout.String(), "resolution_source") ||
				!strings.Contains(stdout.String(), workspaceResolutionCWD) {
				t.Fatalf("%s output = %q, want cwd resolution provenance", tt.format, stdout.String())
			}
			if tt.format == "jsonl" && len(tt.items) > 0 {
				var row map[string]any
				if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &row); err != nil {
					t.Fatalf("json.Unmarshal(populated JSONL) error = %v", err)
				}
				if row["resolution_source"] != workspaceResolutionCWD {
					t.Fatalf("JSONL resolution_source = %#v, want %q", row["resolution_source"], workspaceResolutionCWD)
				}
			}
			if len(tt.items) > 0 && tt.items[0]["id"] == "<one>" {
				if !strings.Contains(stdout.String(), "<one>") || strings.Contains(stdout.String(), `\u003c`) {
					t.Fatalf("JSON output = %q, want unescaped literal HTML characters", stdout.String())
				}
			}
		})
	}
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
