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
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDSessionStatus, toolspkg.ToolIDSessionCreate, toolspkg.ToolIDSessionArchive,
		toolspkg.ToolIDSessionUnarchive, toolspkg.ToolIDSessionLater, toolspkg.ToolIDSessionResumeLater,
		toolspkg.ToolIDSessionMarkSeen, toolspkg.ToolIDSessionRename, toolspkg.ToolIDSessionReparent,
		toolspkg.ToolIDSessionRuntimeSet, toolspkg.ToolIDSessionRuntimeClear,
		toolspkg.ToolIDSessionWorktreeBind, toolspkg.ToolIDSessionWorktreeUnbind,
	} {
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
	rows = append(rows, nativeSessionCompositeViewFixtures(t, &payload)...)
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

func nativeSessionCompositeViewFixtures(t *testing.T, source *contract.SessionPayload) []nativeResultViewFixture {
	t.Helper()
	payload := *source
	payload.FolderID = "folder-view"
	payload.Health = &contract.SessionHealthPayload{
		SessionID: payload.ID, WorkspaceID: "ws-view", AgentName: payload.AgentName,
		State: "stopped", Health: "failed", LastError: "runtime failed", UpdatedAt: payload.UpdatedAt,
	}
	catalog := contract.SessionCatalogResponse{
		Sessions: []contract.SessionPayload{payload, payload, payload, payload},
		Page:     contract.SessionCatalogPagePayload{NextCursor: "next-page", HasMore: true, Total: new(8), Limit: 4},
	}
	empty := contract.SessionCatalogResponse{
		Sessions: []contract.SessionPayload{}, Page: contract.SessionCatalogPagePayload{Total: new(0), Limit: 4},
	}
	rewind := contract.SessionConversationRewindResponse{
		Session: payload,
		Rewind: contract.SessionConversationRewindPayload{
			TranscriptEpoch: 2, Generation: 3, MaxSequence: 12, TargetMessageID: "msg-view",
			DraftText: "Keep the requested prompt whole.", ArchivedFrom: 8, ArchivedThrough: 11, ArchivedEvents: 4,
		},
	}
	move := sessionMoveToProfileResult{
		Session: payload, Original: payload, WorktreeID: "wt-copy", Branch: "feat/copy",
	}
	rows := []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDSessionList,
			Name:   "four records with health and cursor",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionListViews.result(view, catalog)
			},
			MustKeep: []string{
				"sessions.0.id",
				"sessions.0.folder_id",
				"sessions.0.health.last_error",
				"sessions.0.failure",
				"sessions.0.pending_interactions.0.interaction_id",
				"page.next_cursor",
				"page.has_more",
				"page.total",
				"page.limit",
			},
		},
		{ToolID: toolspkg.ToolIDSessionList, Name: "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionListViews.result(view, empty)
			},
			MustKeep: []string{"sessions", "page.has_more", "page.total", "page.limit"}},
		{ToolID: toolspkg.ToolIDSessionRewind, Name: "rewind fences and requested prompt",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionRewindViews.result(view, rewind)
			},
			MustKeep: []string{"session.id", "session.failure", "rewind"}},
		{
			ToolID: toolspkg.ToolIDSessionMoveToProfile,
			Name:   "copy and original with worktree",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionMoveToProfileViews.result(view, move)
			},
			MustKeep: []string{
				"session.id",
				"session.profile_id",
				"original.id",
				"original.profile_id",
				"original.failure",
				"worktree_id",
				"branch",
			},
		},
	}
	moveWithoutWorktree := sessionMoveToProfileResult{Session: payload, Original: payload}
	rows = append(rows, nativeResultViewFixture{
		ToolID: toolspkg.ToolIDSessionMoveToProfile, Name: "copy without worktree",
		Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
			return sessionMoveToProfileViews.result(view, moveWithoutWorktree)
		},
		MustKeep: []string{"session.id", "original.id"},
	})
	rows = append(rows, nativeSessionDescribeViewFixtures(t, &payload)...)
	rows = append(rows, nativeSessionDeriveViewFixtures(t, &payload)...)
	return rows
}

func nativeSessionDeriveViewFixtures(t *testing.T, payload *contract.SessionPayload) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	for _, item := range []struct {
		id   toolspkg.ToolID
		kind string
	}{
		{toolspkg.ToolIDSessionContinue, "continue"}, {toolspkg.ToolIDSessionFork, "fork"},
	} {
		derived := contract.SessionDerivedPayload{
			Kind: item.kind, SourceSessionID: "sess-source", OriginAgentName: "general", ThroughTurnID: "turn-source",
			Seed: "replay", NativeForkError: "native fork unavailable", Truncated: true, OmittedCount: 3,
			SourceTurnInProgress: true, FirstPrompt: "staged", Replayed: true, ChildSessionID: payload.ID,
			ReplayMessageCount: new(4), ReplayBytes: new(1024),
		}
		p := contract.SessionDeriveResponse{Session: payload, Derived: derived}
		rows = append(rows, nativeResultViewFixture{
			ToolID: item.id,
			Name:   "fallback and truncated replay",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) { return sessionDeriveViews.result(view, p) },
			MustKeep: []string{
				"session.id",
				"session.failure",
				"derived.native_fork_error",
				"derived.truncated",
				"derived.omitted_count",
				"derived.source_turn_in_progress",
				"derived.child_session_id",
				"derived.first_prompt",
				"derived.replayed",
			},
		})
		derived.ChildDeleted = true
		deleted := contract.SessionDeriveResponse{Derived: derived}
		rows = append(rows, nativeResultViewFixture{
			ToolID: item.id, Name: "deleted child replay",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionDeriveViews.result(view, deleted)
			},
			MustKeep: []string{"derived.child_session_id", "derived.child_deleted", "derived.replayed"},
		})
	}
	return rows
}

func nativeSessionDescribeViewFixtures(t *testing.T, payload *contract.SessionPayload) []nativeResultViewFixture {
	t.Helper()
	event := contract.SessionEventPayload{
		ID:         "evt-view",
		SessionID:  payload.ID,
		Sequence:   12,
		TurnID:     "turn-view",
		Type:       "error",
		AgentName:  payload.AgentName,
		Content:    []byte(`{"provider_payload":{"unknown_key":[{"text":"requested content"}]}}`),
		Timestamp:  payload.UpdatedAt,
		StopReason: "failed",
		StopDetail: "provider failed",
		Failure:    payload.Failure,
		Origin: &contract.PromptOriginMeta{
			Kind:        "session",
			SessionID:   "sess-source",
			WorkspaceID: "ws-view",
			Hop:         1,
		},
		Goal: &contract.GoalPromptMeta{Kind: "initial", RunID: "run-view", NodeID: "node-view", Turn: new(1)},
	}
	page := nativeSessionPage{Limit: 1, HasMore: true, NextBeforeSequence: 12, NextAfterSequence: 13}
	populated := sessionDescribeResult{Session: *payload, Events: []any{event}, EventsPage: page,
		History: []any{map[string]any{"turn_id": "turn-view", "events": []any{event}}}, HistoryPage: page}
	empty := sessionDescribeResult{Session: *payload, Events: []any{}, History: []any{},
		EventsPage: nativeSessionPage{Limit: 1}, HistoryPage: nativeSessionPage{Limit: 1}}
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDSessionDescribe,
			Name:   "requested content and failures",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionDescribeViews.result(view, populated)
			},
			MustKeep: []string{
				"session.id",
				"session.failure",
				"events.0.content.provider_payload.unknown_key.0.text",
				"events.0.failure",
				"events.0.origin",
				"events.0.goal",
				"history.0.events.0.content.provider_payload.unknown_key.0.text",
				"history.0.events.0.failure",
				"events_page.next_before_sequence",
				"history_page.next_after_sequence",
			},
		},
		{ToolID: toolspkg.ToolIDSessionDescribe, Name: "empty content pages",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return sessionDescribeViews.result(view, empty)
			},
			MustKeep: []string{"session.id", "events", "history", "events_page.has_more", "history_page.has_more"}},
	}
}
