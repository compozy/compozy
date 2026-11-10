package daemon

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
)

func nativeResultViewFixturesSession(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	payload := core.SessionPayloadFromInfo(
		&session.Info{
			ID:            "sess-view",
			ProfileID:     store.DefaultProfileID,
			AgentName:     "general",
			RuntimeStatus: session.RuntimeStatusFailed,
			State:         session.StateStopped,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	)
	payload.StopReason = "failed"
	payload.StopDetail = "provider failed"
	payload.StopCause = "runtime_exit"
	payload.Failure = &contract.SessionFailurePayload{Kind: "runtime", Summary: "provider failed"}
	payload.PermissionModeDegraded = true
	payload.AttentionDismissible = true
	payload.Lineage = &contract.SessionLineagePayload{
		ParentSessionID:  "sess-parent",
		RootSessionID:    "sess-root",
		Kind:             "spawn",
		TTLExpiresAt:     &now,
		SpawnBudget:      contract.SpawnBudgetPayload{MaxChildren: 5, MaxDepth: 2, TTLSeconds: 600},
		PermissionPolicy: nativeResultViewPermissions(),
	}
	payload.PendingInteractions = []contract.PendingInteractionPayload{
		{
			SessionID:         payload.ID,
			InteractionID:     "int-view",
			Kind:              "permission",
			ProviderRequestID: "req-view",
			Status:            "pending",
			CreatedAt:         now,
		},
	}
	payload.Runtime.Generation = 2
	payload.TranscriptEpoch = 1
	payload.Runtime.Failure = "provider failed"
	payload.Runtime.Effective = &contract.RuntimeSelectionPayload{
		Provider:   "codex",
		Model:      "model",
		ACPOptions: []contract.AgentACPOptionSelection{{ID: "option", ValueID: "value"}},
	}
	payload.Queue = &contract.SessionQueueSummaryPayload{Entries: 1, Cap: 10}
	payload.Children = &contract.SessionChildrenPayload{Total: 2, NeedsYou: 1}
	payload.SubagentSummary = &contract.SubagentSummaryPayload{
		Live:       1,
		Total:      2,
		Failed:     1,
		Attention:  1,
		MostUrgent: "failed",
	}
	payload.Later = &contract.SessionLaterPayload{ParkedAt: now, PendingMessages: 1}
	var rows []nativeResultViewFixture
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDSessionStatus, toolspkg.ToolIDSessionCreate, toolspkg.ToolIDSessionArchive} {
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: id,
				Name:   "failure and interaction",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return sessionRecordViews.result(view, payload)
				},
				MustKeep: []string{
					"session.stop_reason",
					"session.stop_detail",
					"session.failure",
					"session.stop_cause",
					"session.pending_interactions.0.interaction_id",
					"session.pending_interactions.0.kind",
					"session.permission_mode_degraded",
					"session.attention_dismissible",
					"session.lineage.ttl_expires_at",
					"session.lineage.spawn_budget",
					"session.runtime.generation",
					"session.transcript_epoch",
					"session.runtime.selection_revision",
				},
			},
		)
		plain := core.SessionPayloadFromInfo(
			&session.Info{
				ID:            "sess-new",
				ProfileID:     store.DefaultProfileID,
				AgentName:     "general",
				RuntimeStatus: session.RuntimeStatusUnbound,
				State:         session.StateActive,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		)
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: id,
				Name:   "never bound",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return sessionRecordViews.result(view, plain)
				},
				MustKeep: []string{"session.id", "session.runtime.selection_revision"},
			},
		)
	}
	spawn := map[string]any{
		"session_id":     "sess-child",
		"spawn_role":     "worker",
		"spawn_depth":    1,
		"ttl_expires_at": now,
		"permissions":    nativeResultViewPermissions(),
		"prompt":         map[string]any{"message_id": "message-1", "status": "failed", "error": "prompt refused"},
		"worktree": map[string]any{
			"id":      "wt-1",
			"name":    "work",
			"branch":  "feat/work",
			"path":    "/work",
			"created": true,
		},
	}
	rows = append(
		rows,
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDSessionSpawn,
			Name:   "failed first prompt",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionSpawnViews.result(view, spawn)
			},
			MustKeep: []string{
				"session_id",
				"ttl_expires_at",
				"spawn_depth",
				"prompt.status",
				"prompt.error",
				"worktree.id",
			},
		},
	)
	return rows
}

func nativeResultViewPermissions() contract.SpawnPermissionPolicyPayload {
	permissions := contract.SpawnPermissionPolicyPayload{
		Tools:          []string{},
		Skills:         []string{},
		MCPServers:     []string{},
		WorkspacePaths: []string{},
	}
	for _, d := range builtintools.NativeDescriptors() {
		permissions.Tools = append(permissions.Tools, d.ID.String())
	}
	return permissions
}
