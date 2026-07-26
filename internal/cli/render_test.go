package cli

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestBundlesRenderHumanAndToon(t *testing.T) {
	t.Parallel()

	sessionInfo := SessionRecord{
		ID:            "sess-1",
		Name:          "demo",
		AgentName:     "coder",
		WorkspaceID:   "ws-1",
		WorkspacePath: "/workspace/project",
		State:         "active",
		ACPSessionID:  "acp-1",
		ACPCaps: &ACPCapsRecord{
			SupportsLoadSession: true,
			SupportedModes:      []string{"chat"},
		},
		CreatedAt: fixedTestNow.Add(-10 * time.Minute),
		UpdatedAt: fixedTestNow,
	}
	sessionEvents := []SessionEventRecord{{
		ID:        "evt-1",
		SessionID: "sess-1",
		Sequence:  1,
		TurnID:    "turn-1",
		Type:      "agent_message",
		AgentName: "coder",
		Content:   mustJSON(t, map[string]string{"text": "hello"}),
		Timestamp: fixedTestNow,
	}}
	history := []TurnHistoryRecord{{TurnID: "turn-1", Events: sessionEvents}}
	agentEvents := []AgentEventRecord{{
		Type:      "agent_message",
		Timestamp: fixedTestNow,
		Text:      "hello",
	}}
	listLogs := []LogEventRecord{{
		ID:        "sum-1",
		SessionID: "sess-1",
		Type:      "done",
		AgentName: "coder",
		Summary:   "finished",
		Timestamp: fixedTestNow,
	}}

	bundles := []struct {
		name   string
		bundle outputBundle
	}{
		{
			name: "agent",
			bundle: agentBundle(
				AgentRecord{Name: "coder", Provider: "fake", Prompt: "You are coder.", Tools: []string{"shell"}},
			),
		},
		{name: "session", bundle: sessionBundle(sessionInfo, func() time.Time { return fixedTestNow })},
		{name: "sessionEvents", bundle: sessionEventsBundle(sessionEvents)},
		{name: "sessionHistory", bundle: sessionHistoryBundle(history)},
		{name: "agentEvents", bundle: agentEventsBundle(agentEvents)},
		{name: "listLogs", bundle: logsBundle(listLogs)},
		{
			name: "daemonStatus",
			bundle: daemonStatusBundle(
				DaemonStatus{
					Status:         "running",
					PID:            10,
					StartedAt:      fixedTestNow.Add(-time.Minute),
					Socket:         "/tmp/agh.sock",
					HTTPHost:       "localhost",
					HTTPPort:       2123,
					ActiveSessions: 1,
					TotalSessions:  2,
					Version:        "dev",
				},
				func() time.Time { return fixedTestNow },
			),
		},
		{
			name:   "whoami",
			bundle: whoamiBundle(IdentityRecord{SessionID: "sess-1", Agent: "agent-1", AgentName: "coder"}),
		},
	}

	for _, item := range bundles {
		t.Run(item.name, func(t *testing.T) {
			human, err := item.bundle.human()
			if err != nil {
				t.Fatalf("human() error = %v", err)
			}
			if strings.TrimSpace(human) == "" {
				t.Fatal("human() returned empty output")
			}

			toon, err := item.bundle.toon()
			if err != nil {
				t.Fatalf("toon() error = %v", err)
			}
			if strings.TrimSpace(toon) == "" {
				t.Fatal("toon() returned empty output")
			}
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	t.Parallel()

	if got := compactJSON([]byte("{\n  \"x\": 1\n}")); got != `{"x":1}` {
		t.Fatalf("compactJSON() = %q, want compact JSON", got)
	}
	if got := formatTime(fixedTestNow); got != "2026-04-03T12:00:00Z" {
		t.Fatalf("formatTime() = %q, want RFC3339", got)
	}
	if got := formatAge(func() time.Time { return fixedTestNow }, fixedTestNow.Add(-2*time.Hour)); got != "2h" {
		t.Fatalf("formatAge() = %q, want %q", got, "2h")
	}
	if got := intOrDash(0); got != "--" {
		t.Fatalf("intOrDash(0) = %q, want --", got)
	}
	if got := int64OrDash(2); got != "2" {
		t.Fatalf("int64OrDash(2) = %q, want 2", got)
	}
	if got := firstNonEmpty("", "alpha", "beta"); got != "alpha" {
		t.Fatalf("firstNonEmpty() = %q, want alpha", got)
	}
	if got := renderHumanBlocks("", "one", "two"); got != "one\n\ntwo" {
		t.Fatalf("renderHumanBlocks() = %q, want two blocks", got)
	}
	if got := renderToonObject("demo", []string{"id"}, []string{"value"}); !strings.Contains(got, "demo{id}:") {
		t.Fatalf("renderToonObject() = %q, want TOON object", got)
	}
	if got := renderToonArray(
		"demo",
		[]string{"id", "text"},
		[][]string{{"1", `hello, "world"`}},
	); got != "demo[1]{id,text}:\n  1,\"hello, \\\"world\\\"\"" {
		t.Fatalf("renderToonArray() = %q, want escaped TOON row", got)
	}
	if got := renderToonArray("demo", []string{"id"}, nil); got != "demo[0]{id}:\n  (empty)" {
		t.Fatalf("renderToonArray() empty = %q, want empty marker", got)
	}
}

func TestListBundleRendersJSONHumanAndToon(t *testing.T) {
	t.Parallel()

	type demoRow struct {
		ID    string `json:"id"`
		Count int    `json:"count"`
	}

	bundle := listBundle(
		[]demoRow{{ID: "row-1", Count: 2}},
		[]demoRow{{ID: "row-1", Count: 2}},
		"Demo Rows",
		[]string{"ID", "Count"},
		"demo_rows",
		[]string{"id", "count"},
		func(item demoRow) []string {
			return []string{item.ID, strconv.Itoa(item.Count)}
		},
		func(item demoRow) []string {
			return []string{item.ID, strconv.Itoa(item.Count)}
		},
	)

	for _, mode := range []OutputFormat{OutputJSON, OutputHuman, OutputToon} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			cmd, output := newOutputTestCommand(t, mode)
			if err := writeCommandOutput(cmd, bundle); err != nil {
				t.Fatalf("writeCommandOutput(%s) error = %v", mode, err)
			}

			rendered := output.String()
			if rendered == "" {
				t.Fatalf("writeCommandOutput(%s) output = empty", mode)
			}

			switch mode {
			case OutputJSON:
				if !strings.Contains(rendered, `"id": "row-1"`) {
					t.Fatalf("json output = %q, want serialized row", rendered)
				}
			case OutputHuman:
				if !strings.Contains(rendered, "Demo Rows") || !strings.Contains(rendered, "row-1") {
					t.Fatalf("human output = %q, want title and row", rendered)
				}
			case OutputToon:
				if !strings.Contains(rendered, "demo_rows[1]{id,count}:") || !strings.Contains(rendered, "row-1") {
					t.Fatalf("toon output = %q, want TOON array", rendered)
				}
			}
		})
	}
}

func TestVersionCommandFormats(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})

	humanOut, _, err := executeRootCommand(t, deps, "version", "-o", "human")
	if err != nil {
		t.Fatalf("version human error = %v", err)
	}
	if !strings.Contains(humanOut, "agh ") {
		t.Fatalf("version human output = %q, want agh prefix", humanOut)
	}

	toonOut, _, err := executeRootCommand(t, deps, "version", "-o", "toon")
	if err != nil {
		t.Fatalf("version toon error = %v", err)
	}
	if !strings.Contains(toonOut, "version{version,commit,build_date}:") {
		t.Fatalf("version toon output = %q, want TOON object", toonOut)
	}
}

func newOutputTestCommand(t *testing.T, mode OutputFormat) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.Flags().String(outputFlagName, string(OutputHuman), "output format")
	_ = cmd.Flags().Set(outputFlagName, string(mode))
	return cmd, output
}
