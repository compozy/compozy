package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func overviewTestPayload() contract.ObserveOverviewPayload {
	cost := 1.25
	return contract.ObserveOverviewPayload{
		SchemaVersion: contract.ObserveOverviewSchemaVersion,
		GeneratedAt:   time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
		Attention: contract.OverviewAttentionPayload{
			Total:  3,
			ByKind: map[string]int{"approval": 1, "needs_input": 1, "failure": 1},
			Items:  []contract.OverviewAttentionItemPayload{},
		},
		Today: contract.OverviewTodayPayload{RunsCompleted: 5, RunsFailed: 1, TasksClosed: 3},
		Outcomes: contract.OverviewOutcomesPayload{
			WindowDays: 14, Completed: 118, Failed: 7, Canceled: 4, SuccessPct: 91.5,
		},
		Usage: contract.OverviewUsagePayload{
			WindowDays:    30,
			RetentionDays: 7,
			Truncated:     true,
			TotalTokens:   1_400_000,
			EstimatedCost: &cost,
			CostCurrency:  "USD",
			CostStatus:    "estimated",
		},
		Pulse: contract.OverviewPulsePayload{
			WindowDays: 14,
			Busiest:    &contract.OverviewPulseBucketPayload{Weekday: 2, Hour: 14, Events: 42},
			LongestSession: &contract.OverviewLongestSessionPayload{
				SessionID:       "sess-long",
				AgentName:       "writer",
				DurationSeconds: 8040,
				Date:            "2026-07-22",
			},
		},
		Network: contract.OverviewNetworkPayload{MessagesToday: 128},
		System:  contract.OverviewSystemPayload{HookRunsToday: 24, HookFailuresToday: 0, RetentionDays: 7},
	}
}

func TestObserveOverviewCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should return the raw overview payload for json output", func(t *testing.T) {
		t.Parallel()

		var captured ObserveOverviewQuery
		client := &stubClient{
			observeOverviewFn: func(_ context.Context, query ObserveOverviewQuery) (contract.ObserveOverviewResponse, error) {
				captured = query
				return contract.ObserveOverviewResponse{Overview: overviewTestPayload()}, nil
			},
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(
			t, deps,
			"observe", "overview", "-o", "json", "--workspace", "launch-hq", "--usage-window", "7",
		)
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}

		if captured.Workspace != "launch-hq" || captured.UsageWindowDays != 7 {
			t.Fatalf("client query = %+v, want workspace and usage window forwarded", captured)
		}

		var decoded contract.ObserveOverviewPayload
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.SchemaVersion != contract.ObserveOverviewSchemaVersion {
			t.Fatalf("schema_version = %q, want %q", decoded.SchemaVersion, contract.ObserveOverviewSchemaVersion)
		}
		if decoded.Attention.Total != 3 || decoded.Network.MessagesToday != 128 {
			t.Fatalf("decoded = %+v, want stubbed counters", decoded)
		}
	})

	t.Run("Should render human sections with truthful counters", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			observeOverviewFn: func(context.Context, ObserveOverviewQuery) (contract.ObserveOverviewResponse, error) {
				return contract.ObserveOverviewResponse{Overview: overviewTestPayload()}, nil
			},
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(t, deps, "observe", "overview")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}

		for _, expected := range []string{
			"Needs you",
			"approvals",
			"runs completed",
			"Tue 14:00 · 42 events",
			"writer · 2h 14m · 2026-07-22",
			"bounded to 7 days",
			"24 runs · 0 failed",
		} {
			if !strings.Contains(stdout, expected) {
				t.Fatalf("human output missing %q in:\n%s", expected, stdout)
			}
		}
	})

	t.Run("Should emit one jsonl line per overview section", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			observeOverviewFn: func(context.Context, ObserveOverviewQuery) (contract.ObserveOverviewResponse, error) {
				return contract.ObserveOverviewResponse{Overview: overviewTestPayload()}, nil
			},
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(t, deps, "observe", "overview", "-o", "jsonl")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}

		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 9 {
			t.Fatalf("jsonl lines = %d, want meta + 8 sections:\n%s", len(lines), stdout)
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
			t.Fatalf("json.Unmarshal(meta line) error = %v", err)
		}
		if meta["section"] != "meta" || meta["schema_version"] != contract.ObserveOverviewSchemaVersion {
			t.Fatalf("meta line = %v, want section meta with schema_version", meta)
		}
		var first map[string]any
		if err := json.Unmarshal([]byte(lines[1]), &first); err != nil {
			t.Fatalf("json.Unmarshal(first section) error = %v", err)
		}
		if first["section"] != "attention" {
			t.Fatalf("first section = %v, want attention", first["section"])
		}
	})
}
