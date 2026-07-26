package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func TestNetworkPublicSurfaceCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should print coordination status as structured JSON", func(t *testing.T) {
		t.Parallel()

		updatedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		deps := newTestDeps(t, &stubClient{
			getNetworkCoordinationFn: func(
				_ context.Context,
				workspaceRef string,
				ref NetworkCoordinationRef,
			) (NetworkCoordinationRecord, error) {
				if workspaceRef != "ws-alpha" {
					t.Fatalf("workspace = %q, want ws-alpha", workspaceRef)
				}
				if ref != (NetworkCoordinationRef{Scope: "workspace"}) {
					t.Fatalf("ref = %#v, want explicit workspace scope", ref)
				}
				return NetworkCoordinationRecord{
					WorkspaceID: "ws-alpha",
					Scope:       "workspace",
					Enabled:     true,
					Revision:    4,
					UpdatedAt:   &updatedAt,
					UpdatedBy:   "operator",
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"coordination",
			"status",
			"--json",
		)
		if err != nil {
			t.Fatalf("network coordination status error = %v", err)
		}
		var envelope struct {
			Coordination NetworkCoordinationRecord `json:"coordination"`
		}
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("decode stdout: %v\nstdout=%s", err, stdout)
		}
		payload := envelope.Coordination
		if !payload.Enabled || payload.Revision != 4 {
			t.Fatalf("payload = %#v, want enabled revision=4", payload)
		}
	})

	t.Run("Should enable coordination through CLI PUT", func(t *testing.T) {
		t.Parallel()

		var captured PutNetworkCoordinationRequest
		deps := newTestDeps(t, &stubClient{
			getNetworkCoordinationFn: func(
				_ context.Context,
				workspaceRef string,
				ref NetworkCoordinationRef,
			) (NetworkCoordinationRecord, error) {
				if workspaceRef != "ws-alpha" || ref.Scope != "task" ||
					ref.TaskID != "task-1" || ref.RunID != "run-1" {
					t.Fatalf("workspace/ref = %q/%#v, want task scope", workspaceRef, ref)
				}
				return NetworkCoordinationRecord{Revision: 4}, nil
			},
			putNetworkCoordinationFn: func(
				_ context.Context,
				_ string,
				request PutNetworkCoordinationRequest,
			) (NetworkCoordinationRecord, error) {
				captured = request
				updatedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
				return NetworkCoordinationRecord{
					WorkspaceID: "ws-alpha",
					Scope:       request.Scope,
					TaskID:      request.TaskID,
					Enabled:     request.Enabled != nil && *request.Enabled,
					Revision:    5,
					UpdatedAt:   &updatedAt,
					UpdatedBy:   "operator",
				}, nil
			},
		})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"coordination",
			"enable",
			"--scope",
			"task",
			"--task-id",
			"task-1",
			"--run-id",
			"run-1",
			"--json",
		); err != nil {
			t.Fatalf("network coordination enable error = %v", err)
		}
		if captured.Enabled == nil || !*captured.Enabled || captured.ExpectedRevision == nil ||
			*captured.ExpectedRevision != 4 || captured.Scope != "task" ||
			captured.TaskID != "task-1" || captured.RunID != "run-1" {
			t.Fatalf("captured = %#v, want task-scoped CAS enable", captured)
		}
	})

	t.Run("Should dismiss one task invitation through the same scoped CAS", func(t *testing.T) {
		t.Parallel()

		var captured PutNetworkCoordinationInvitationRequest
		deps := newTestDeps(t, &stubClient{
			getNetworkCoordinationFn: func(
				_ context.Context,
				_ string,
				ref NetworkCoordinationRef,
			) (NetworkCoordinationRecord, error) {
				if ref.TaskID != "task-2" || ref.RunID != "run-2" || ref.Scope != "task" {
					t.Fatalf("ref = %#v, want authoritative task invitation scope", ref)
				}
				return NetworkCoordinationRecord{Revision: 7}, nil
			},
			putNetworkCoordinationInvitationFn: func(
				_ context.Context,
				_ string,
				request PutNetworkCoordinationInvitationRequest,
			) (NetworkCoordinationRecord, error) {
				captured = request
				return NetworkCoordinationRecord{Revision: 8}, nil
			},
		})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"invitation",
			"dismiss",
			"--scope",
			"task",
			"--task-id",
			"task-2",
			"--run-id",
			"run-2",
			"--json",
		); err != nil {
			t.Fatalf("network invitation dismiss error = %v", err)
		}
		if captured.Dismissed == nil || !*captured.Dismissed ||
			captured.ExpectedRevision == nil || *captured.ExpectedRevision != 7 ||
			captured.Scope != "task" || captured.TaskID != "task-2" || captured.RunID != "run-2" {
			t.Fatalf("captured = %#v, want task-scoped CAS dismissal", captured)
		}
	})

	t.Run("Should render every coordination command as TOON", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getNetworkCoordinationFn: func(
				context.Context,
				string,
				NetworkCoordinationRef,
			) (NetworkCoordinationRecord, error) {
				return NetworkCoordinationRecord{
					WorkspaceID: "ws-alpha",
					Scope:       "workspace",
					Enabled:     true,
					Revision:    4,
				}, nil
			},
			putNetworkCoordinationFn: func(
				_ context.Context,
				_ string,
				request PutNetworkCoordinationRequest,
			) (NetworkCoordinationRecord, error) {
				return NetworkCoordinationRecord{
					WorkspaceID: "ws-alpha",
					Scope:       request.Scope,
					Enabled:     request.Enabled != nil && *request.Enabled,
					Revision:    5,
				}, nil
			},
		})

		for _, action := range []string{"status", "enable", "disable"} {
			stdout, _, err := executeRootCommand(
				t,
				deps,
				"network", "--workspace", "ws-alpha", "coordination", action, "-o", "toon",
			)
			if err != nil {
				t.Fatalf("network coordination %s -o toon error = %v", action, err)
			}
			if !strings.Contains(stdout, "network_coordination{") || !strings.Contains(stdout, "ws-alpha") {
				t.Fatalf("network coordination %s TOON = %q, want coordination payload", action, stdout)
			}
		}
	})

	t.Run("Should print network usage totals as structured JSON", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getNetworkUsageFn: func(
				_ context.Context,
				workspaceRef string,
				query NetworkUsageQuery,
			) (NetworkUsageRecord, error) {
				if workspaceRef != "ws-alpha" {
					t.Fatalf("workspace = %q, want ws-alpha", workspaceRef)
				}
				if query.OwnerKind != "task_run" || query.OwnerID != "run-1" ||
					query.Channel != "builders" || query.Cursor != "cursor-prev" || query.Limit != 25 {
					t.Fatalf("query = %#v, want CLI usage filters", query)
				}
				return NetworkUsageRecord{
					WorkspaceID: "ws-alpha",
					NextCursor:  "cursor-next",
					Details: []contract.NetworkUsageDetailPayload{{
						WakeID:      "wake-1",
						WorkspaceID: "ws-alpha",
						Channel:     "builders",
						State:       "settled",
						UsageState:  "actual",
						ReservedAt:  time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
					}},
					Total: contract.NetworkUsageSummaryPayload{
						WakeCount:       2,
						ActualWakeCount: 2,
						InputTokens:     9,
						OutputTokens:    3,
					},
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"usage",
			"--owner-kind",
			"task_run",
			"--owner-id",
			"run-1",
			"--channel",
			"builders",
			"--cursor",
			"cursor-prev",
			"--limit",
			"25",
			"--json",
		)
		if err != nil {
			t.Fatalf("network usage error = %v", err)
		}
		if !strings.Contains(stdout, "ws-alpha") {
			t.Fatalf("stdout = %s, want usage payload", stdout)
		}
		var payload NetworkUsageRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode stdout: %v\nstdout=%s", err, stdout)
		}
		if payload.Total.ActualWakeCount != 2 {
			t.Fatalf("total = %#v, want actual_wake_count=2", payload.Total)
		}
		if payload.NextCursor != "cursor-next" {
			t.Fatalf("next_cursor = %q, want cursor-next", payload.NextCursor)
		}

		toon, _, err := executeRootCommand(
			t,
			deps,
			"network", "--workspace", "ws-alpha", "usage",
			"--owner-kind", "task_run", "--owner-id", "run-1",
			"--channel", "builders", "--cursor", "cursor-prev", "--limit", "25",
			"-o", "toon",
		)
		if err != nil {
			t.Fatalf("network usage -o toon error = %v", err)
		}
		if !strings.Contains(toon, "network_usage{") ||
			!strings.Contains(toon, "network_usage_details[") ||
			!strings.Contains(toon, "wake-1") {
			t.Fatalf("network usage TOON = %q, want summary and detail rows", toon)
		}
	})

	t.Run("Should describe current Network status without removed queue metrics", func(t *testing.T) {
		t.Parallel()

		stdout, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), "network", "status", "--help")
		if err != nil {
			t.Fatalf("network status --help error = %v", err)
		}
		if !strings.Contains(stdout, "Network availability, Live participation, and runtime metrics") ||
			strings.Contains(strings.ToLower(stdout), "queue metrics") {
			t.Fatalf("network status help = %q, want current availability/Live metrics", stdout)
		}
	})
}
