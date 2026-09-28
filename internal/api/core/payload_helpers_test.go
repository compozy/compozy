package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	observepkg "github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestCoreConversionHelpers(t *testing.T) {
	t.Parallel()
	t.Run("Should carry usage metadata and ledger sequence across response payloads", func(t *testing.T) {
		t.Parallel()
		usage := &acp.TokenUsage{TurnID: "turn", Sequence: 19, Meta: map[string]any{"origin": "result"}}
		shared := TokenUsagePayloadFromUsage(usage)
		prompt := promptTokenUsagePayloadFromUsage(usage)
		if shared.Sequence == nil || *shared.Sequence != 19 || shared.Meta["origin"] != "result" ||
			prompt.Sequence == nil ||
			*prompt.Sequence != 19 ||
			prompt.Meta["origin"] != "result" {
			t.Fatalf("usage payloads = %#v / %#v", shared, prompt)
		}
		for _, value := range []any{TokenUsagePayloadFromUsage(&acp.TokenUsage{}), promptTokenUsagePayloadFromUsage(&acp.TokenUsage{})} {
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["meta"]; ok {
				t.Fatal("unreported metadata must be omitted")
			}
			if _, ok := fields["sequence"]; ok {
				t.Fatal("unknown sequence must be omitted")
			}
		}
	})

	t.Run("Should convert core payload helpers", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

		usage := TokenUsagePayloadFromUsage(&acp.TokenUsage{
			TurnID:           "turn-1",
			InputTokens:      new(int64(10)),
			OutputTokens:     new(int64(20)),
			TotalTokens:      new(int64(30)),
			ThoughtTokens:    new(int64(3)),
			CacheReadTokens:  new(int64(4)),
			CacheWriteTokens: new(int64(5)),
			ContextUsed:      new(int64(6)),
			ContextSize:      new(int64(7)),
			CostAmount:       new(1.23),
			CostCurrency:     new("USD"),
			Timestamp:        now,
		})
		if usage == nil || usage.TotalTokens == nil || *usage.TotalTokens != 30 || usage.CostCurrency == nil ||
			*usage.CostCurrency != "USD" {
			t.Fatalf("TokenUsagePayloadFromUsage() = %#v", usage)
		}
		if TokenUsagePayloadFromUsage(nil) != nil {
			t.Fatal("TokenUsagePayloadFromUsage(nil) != nil")
		}

		if got := string(PayloadJSON("  ")); got != "null" {
			t.Fatalf("PayloadJSON(blank) = %s, want null", got)
		}
		if got := string(PayloadJSON(`{"ok":true}`)); got != `{"ok":true}` {
			t.Fatalf("PayloadJSON(valid json) = %s", got)
		}
		if got := string(PayloadJSON("not-json")); got != `"not-json"` {
			t.Fatalf("PayloadJSON(string) = %s, want quoted string", got)
		}

		if workspaceID, workspace := sessionWorkspaceFromInfo(
			&session.Info{WorkspaceID: " ws-1 ", Workspace: " /tmp/ws "},
		); workspaceID != "ws-1" ||
			workspace != "/tmp/ws" {
			t.Fatalf("sessionWorkspaceFromInfo() = %q/%q", workspaceID, workspace)
		}
		if workspaceID, workspace := sessionWorkspaceFromInfo(nil); workspaceID != "" || workspace != "" {
			t.Fatalf("sessionWorkspaceFromInfo(nil) = %q/%q", workspaceID, workspace)
		}
	})
}

func TestCoreTimeAndSessionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize time and session helpers", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.FixedZone("offset", -3*60*60))
		got := timePointerFromMap(map[string]*time.Time{"sess-1": &now}, "sess-1")
		if got == nil || !got.Equal(now.UTC()) || got.Location() != time.UTC {
			t.Fatalf("timePointerFromMap() = %#v, want UTC copy", got)
		}
		if timePointerFromMap(nil, "sess-1") != nil {
			t.Fatal("timePointerFromMap(nil) != nil")
		}
		if timePointerFromMap(map[string]*time.Time{"sess-1": nil}, "sess-1") != nil {
			t.Fatal("timePointerFromMap(nil entry) != nil")
		}
	})
}

func TestObserveHealthPayloadConversions(t *testing.T) {
	t.Run("Should include runtime activity", func(t *testing.T) {
		t.Parallel()

		lastActivityAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
		lastSweepAt := lastActivityAt.Add(-time.Hour)
		lastCutoffAt := lastSweepAt.AddDate(0, 0, -14)
		input := observepkg.Health{
			Status:         "ok",
			ActiveSessions: 1,
			Persistence: observepkg.PersistenceHealth{
				Status:             " degraded ",
				GlobalDBSizeBytes:  4096,
				SessionDBSizeBytes: 2048,
			},
			Retention: observepkg.RetentionHealth{
				Enabled:                  true,
				RetentionDays:            14,
				SweepIntervalSeconds:     86400,
				LastSweepStatus:          " ok ",
				LastSweepAt:              &lastSweepAt,
				LastCutoffAt:             &lastCutoffAt,
				LastSweepError:           " cleared ",
				DeletedEventSummaries:    3,
				DeletedTokenStats:        2,
				DeletedPermissionLogRows: 1,
			},
			Failures: observepkg.FailureHealth{
				Status: " degraded ",
				Total:  1,
				ByKind: map[store.FailureKind]int{store.FailureProtocol: 1},
				Recent: []observepkg.SessionFailureHealth{{
					SessionID:       " sess-protocol ",
					AgentName:       " coder ",
					Provider:        " claude ",
					WorkspaceID:     " ws-1 ",
					State:           " stopped ",
					FailureKind:     store.FailureProtocol,
					Summary:         "bad frame token=super-secret",
					CrashBundlePath: "/tmp/crash-token=super-secret.json",
					UpdatedAt:       lastActivityAt,
				}},
			},
			AgentProbes: []acp.ProbeResult{{
				AgentName:  " coder ",
				Provider:   " claude ",
				Command:    "agent --api-key=super-secret",
				Executable: " /usr/bin/agent ",
				Status:     " missing ",
				Error:      "resolve failed token=super-secret",
				CheckedAt:  lastActivityAt,
				DurationMS: 12,
			}},
			Activities: []observepkg.SessionActivityHealth{{
				SessionID:        " sess-activity ",
				TurnID:           " turn-activity ",
				LastActivityAt:   &lastActivityAt,
				LastActivityKind: "warning",
				CurrentTool:      "delegate_task",
				IdleSeconds:      900,
				Status:           "warning",
			}},
		}
		health := ObserveHealthPayloadFromHealth(&input)

		if got, want := len(health.Activities), 1; got != want {
			t.Fatalf("len(Activities) = %d, want %d", got, want)
		}
		activity := health.Activities[0]
		if activity.SessionID != "sess-activity" ||
			activity.TurnID != "turn-activity" ||
			activity.Status != "warning" ||
			activity.CurrentTool != "delegate_task" ||
			activity.IdleSeconds != 900 {
			t.Fatalf("Activities[0] = %#v, want trimmed runtime activity", activity)
		}
		if health.Persistence.Status != "degraded" ||
			health.Persistence.GlobalDBSizeBytes != 4096 ||
			health.Persistence.SessionDBSizeBytes != 2048 {
			t.Fatalf("Persistence = %#v, want typed persistence health", health.Persistence)
		}
		if !health.Retention.Enabled ||
			health.Retention.RetentionDays != 14 ||
			health.Retention.SweepIntervalSeconds != 86400 ||
			health.Retention.LastSweepStatus != "ok" ||
			health.Retention.LastSweepError != "cleared" ||
			health.Retention.DeletedEventSummaries != 3 ||
			health.Retention.DeletedTokenStats != 2 ||
			health.Retention.DeletedPermissionLogRows != 1 {
			t.Fatalf("Retention = %#v, want typed retention health", health.Retention)
		}
		if health.Retention.LastSweepAt == nil ||
			health.Retention.LastSweepAt == &lastSweepAt ||
			!health.Retention.LastSweepAt.Equal(lastSweepAt) {
			t.Fatalf("Retention.LastSweepAt = %#v, want cloned %s", health.Retention.LastSweepAt, lastSweepAt)
		}
		if health.Retention.LastCutoffAt == nil ||
			health.Retention.LastCutoffAt == &lastCutoffAt ||
			!health.Retention.LastCutoffAt.Equal(lastCutoffAt) {
			t.Fatalf("Retention.LastCutoffAt = %#v, want cloned %s", health.Retention.LastCutoffAt, lastCutoffAt)
		}
		if health.Failures.Status != "degraded" ||
			health.Failures.Total != 1 ||
			health.Failures.ByKind[store.FailureProtocol] != 1 ||
			len(health.Failures.Recent) != 1 {
			t.Fatalf("Failures = %#v, want classified lifecycle failure payload", health.Failures)
		}
		failure := health.Failures.Recent[0]
		if failure.SessionID != "sess-protocol" ||
			failure.FailureKind != store.FailureProtocol ||
			strings.Contains(failure.Summary, "super-secret") ||
			strings.Contains(failure.CrashBundlePath, "super-secret") {
			t.Fatalf("Failures.Recent[0] = %#v, want trimmed and redacted payload", failure)
		}
		if got, want := len(health.AgentProbes), 1; got != want {
			t.Fatalf("len(AgentProbes) = %d, want %d", got, want)
		}
		probe := health.AgentProbes[0]
		if probe.AgentName != "coder" ||
			probe.Provider != "claude" ||
			probe.Executable != "/usr/bin/agent" ||
			probe.Status != "missing" ||
			probe.DurationMS != 12 ||
			strings.Contains(probe.Command, "super-secret") ||
			strings.Contains(probe.Error, "super-secret") {
			t.Fatalf("AgentProbes[0] = %#v, want trimmed and redacted probe payload", probe)
		}
	})

	t.Run("Should encode task run statuses with their public names", func(t *testing.T) {
		t.Parallel()

		payload := TaskHealthPayloadFromObserve(observepkg.TaskHealth{
			StuckRuns: []observepkg.StuckTaskRun{{
				TaskID: "task-1",
				RunID:  "run-1",
				Status: taskpkg.TaskRunStatusRunning,
			}},
			RunTotals: []observepkg.TaskRunTotal{{
				Status: taskpkg.TaskRunStatusNeedsAttention,
				Count:  1,
			}},
		})

		if got, want := payload.StuckRuns[0].Status, taskpkg.TaskRunStatusRunning.String(); got != want {
			t.Fatalf("StuckRuns[0].Status = %q, want %q", got, want)
		}
		if got, want := payload.RunTotals[0].Status, taskpkg.TaskRunStatusNeedsAttention.String(); got != want {
			t.Fatalf("RunTotals[0].Status = %q, want %q", got, want)
		}
	})
}
