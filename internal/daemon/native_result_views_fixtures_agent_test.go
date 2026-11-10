package daemon

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesAgent(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	agent := contract.AgentPayload{
		Name:               "reviewer",
		Label:              "Reviewer",
		Kind:               "always_on",
		Description:        "Review changes",
		Origin:             contract.AgentOriginWorkspace,
		WorkspaceID:        "ws-view",
		Layer:              "workspace",
		PackageOwned:       true,
		DefinitionDigest:   "sha256:definition",
		Prompt:             strings.Repeat("Review the requested changes. ", 80),
		Tools:              []string{"compozy__agent_list"},
		ResolvedAppearance: map[string]any{"icon": "review"},
		Diagnostics: []contract.AgentDiagnosticPayload{
			{Path: "AGENT.md", ErrorKind: "invalid_appearance", Message: "Icon unavailable"},
		},
		EffectiveRuntime: &contract.AgentEffectiveRuntimePayload{
			Provider:    "codex",
			Model:       "model",
			Permissions: "approve-reads",
		},
	}
	rows := []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDAgentCreate,
			Name:   "definition diagnostic",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentRecordViews.result(view, agent)
			},
			MustKeep: []string{
				"agent.name",
				"agent.definition_digest",
				"agent.diagnostics.0.error_kind",
				"agent.diagnostics.0.message",
			},
		},
		{
			ToolID: toolspkg.ToolIDAgentList,
			Name:   "definition diagnostic",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentListViews.result(view, []contract.AgentPayload{agent})
			},
			MustKeep: []string{
				"agents.0.name",
				"agents.0.origin",
				"agents.0.layer",
				"agents.0.workspace_id",
				"agents.0.diagnostics.0.message",
			},
		},
		{
			ToolID: toolspkg.ToolIDAgentList,
			Name:   "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentListViews.result(view, []contract.AgentPayload{})
			},
			MustKeep: []string{"agents"},
		},
		{
			ToolID: toolspkg.ToolIDAgentCreate,
			Name:   "optional fields absent",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentRecordViews.result(view, contract.AgentPayload{Name: "new"})
			},
			MustKeep: []string{"agent.name"},
		},
	}
	rows = append(rows, nativeAgentInstanceViewFixtures(t)...)
	rows = append(rows, nativeAgentHeartbeatViewFixtures(t)...)
	return rows
}

func nativeAgentInstanceViewFixtures(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	instance := contract.AgentInstancePayload{
		ID:                   "inst-view",
		AgentName:            "reviewer",
		Label:                "Reviewer",
		WorkspaceID:          "ws-view",
		ProfileID:            "prof-view",
		ProfileName:          "Default",
		Scope:                "workspace",
		MainSessionID:        "sess-view",
		State:                "paused",
		PauseReason:          "unattended",
		NeedsAttention:       true,
		AttentionDismissible: true,
		AuthorityStatus:      "invalid",
		Created:              true,
		ChecksEnabled:        true,
		UnattendedTurns:      300,
		UnattendedTurnLimit:  300,
		HeldWakes:            2,
		FailureStreak:        3,
		FatigueLimit:         5,
		NextWake:             &contract.AgentInstanceNextWake{Kind: "held", Description: "Waiting for resume"},
		Sources: []heartbeat.HeartbeatSourceStatus{
			{
				ID:               "inbox",
				Kind:             "extension/inbox",
				Status:           "failing",
				FailureStreak:    3,
				Error:            source.Failure("source_unavailable", "Source failed"),
				AwaitingApproval: true,
				NextPollAt:       "2026-11-10T12:00:00Z",
				ScriptPath:       "/scripts/inbox",
				ScriptSHA256:     "sha256:source",
			},
		},
		ResolvedAppearance: map[string]any{"color": "blue"},
	}
	var rows []nativeResultViewFixture
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstanceArchive, toolspkg.ToolIDAgentInstanceChecks, toolspkg.ToolIDAgentInstanceCreate, toolspkg.ToolIDAgentInstanceEnsureMain, toolspkg.ToolIDAgentInstanceGet, toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume} {
		views := agentInstanceRecordViews
		if id == toolspkg.ToolIDAgentInstanceCreate {
			views = agentInstanceCreateViews
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: id,
				Name:   "paused with source failure",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) { return views.result(view, instance) },
				MustKeep: []string{
					"instance.id",
					"instance.main_session_id",
					"instance.pause_reason",
					"instance.needs_attention",
					"instance.authority_status",
					"instance.created",
					"instance.held_wakes",
					"instance.failure_streak",
					"instance.unattended_turns",
					"instance.unattended_turn_limit",
					"instance.fatigue_limit",
					"instance.sources.0.error.code",
					"instance.sources.0.error.detail",
					"instance.sources.0.awaiting_approval",
					"instance.sources.0.next_poll_at",
				},
			},
			nativeResultViewFixture{
				ToolID: id,
				Name:   "optional fields absent",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return views.result(view, contract.AgentInstancePayload{ID: "inst-new"})
				},
				MustKeep: []string{"instance.id", "instance.created", "instance.sources"},
			},
		)
	}
	page := contract.AgentInstanceListResponse{
		Instances:          []contract.AgentInstancePayload{instance},
		ActiveCount:        5,
		MaxActiveInstances: 5,
		NextCursor:         "cursor-view",
	}
	rows = append(
		rows,
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAgentInstanceList,
			Name:   "failure and capacity",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentInstanceListViews.result(view, page)
			},
			MustKeep: []string{
				"active_count",
				"max_active_instances",
				"next_cursor",
				"instances.0.pause_reason",
				"instances.0.sources.0.error",
				"instances.0.sources.0.awaiting_approval",
				"instances.0.unattended_turn_limit",
			},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAgentInstanceList,
			Name:   "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentInstanceListViews.result(
					view,
					contract.AgentInstanceListResponse{
						Instances:          []contract.AgentInstancePayload{},
						MaxActiveInstances: 5,
					},
				)
			},
			MustKeep: []string{"instances", "active_count", "max_active_instances", "next_cursor"},
		},
	)
	peers := contract.AgentInstanceListResponse{
		Instances: []contract.AgentInstancePayload{},
		Peers: []contract.AgentInstancePeer{
			{Name: "reviewer", Description: "Review changes", Scope: "workspace", State: "paused"},
		},
		NextCursor: "peers-next",
	}
	rows = append(
		rows,
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAgentInstanceList,
			Name:   "reachable peers",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentInstancePeersViews.result(view, peers)
			},
			MustKeep: []string{"peers.0.name", "peers.0.state", "next_cursor", "active_count", "max_active_instances"},
		},
	)
	return rows
}

func nativeAgentHeartbeatViewFixtures(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	status := contract.HeartbeatStatusResponse{
		AgentName:        "reviewer",
		Enabled:          true,
		Present:          true,
		ValidationStatus: "invalid",
		Summary:          "Policy invalid",
		SourcePath:       "/agent/HEARTBEAT.md",
		Digest:           "sha256:policy",
		ConfigDigest:     "sha256:config",
		SnapshotID:       "snapshot-view",
		RevisionCursor:   "cursor-view",
		Diagnostics: []contract.AuthoredContextDiagnosticPayload{
			{
				Code:     "invalid_policy",
				Severity: "error",
				Message:  "Source unavailable",
				Field:    "context.sources",
				Line:     2,
			},
		},
		WakeState: &contract.HeartbeatWakeStatePayload{
			SessionID:      "sess-view",
			LastResult:     "failed",
			LastReason:     "invalid_policy",
			CoalescedCount: 2,
			UpdatedAt:      now,
		},
		WakeEvents: []contract.HeartbeatWakeEventPayload{
			{
				ID:        "wake-view",
				SessionID: "sess-view",
				Result:    "failed",
				Reason:    "invalid_policy",
				Detail:    strings.Repeat("Requested audit detail. ", 50),
				CreatedAt: now,
				ExpiresAt: now,
			},
		},
		SessionHealth: &contract.SessionHealthPayload{
			SessionID:   "sess-view",
			WorkspaceID: "ws-view",
			AgentName:   "reviewer",
			LastError:   "Provider failed",
			UpdatedAt:   now,
		},
	}
	catalog := contract.HeartbeatSourceCatalogPayload{
		Agent:          "reviewer",
		ScriptsEnabled: true,
		Limits:         contract.HeartbeatSourceLimitsPayload{MaxPerAgent: 4, MaxItems: 20},
		Kinds: []source.KindDescriptor{
			{
				Kind:        "extension/inbox",
				Extension:   "inbox",
				Title:       "Inbox",
				Description: "Read pending work",
				ParamsSchema: []byte(
					`{"type":"object","properties":{"query":{"type":"string"}},"additionalProperties":false}`,
				),
			},
		},
		Scripts: []contract.HeartbeatSourceScriptPayload{
			{
				Name:       "inbox.sh",
				Path:       "/agent/scripts/inbox.sh",
				Executable: true,
				SHA256:     "sha256:source",
				Approved:   false,
			},
		},
	}
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDAgentHeartbeatStatus,
			Name:   "failure and requested audit",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentHeartbeatStatusViews.result(view, status)
			},
			MustKeep: []string{
				"heartbeat.valid",
				"heartbeat.validation_status",
				"heartbeat.snapshot_id",
				"heartbeat.diagnostics.0.message",
				"heartbeat.wake_state.last_reason",
				"heartbeat.wake_events.0.detail",
				"heartbeat.session_health.last_error",
				"heartbeat.revision_cursor",
			},
		},
		{
			ToolID: toolspkg.ToolIDAgentHeartbeatStatus,
			Name:   "optional fields absent",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentHeartbeatStatusViews.result(view, contract.HeartbeatStatusResponse{AgentName: "reviewer"})
			},
			MustKeep: []string{"heartbeat.agent_name", "heartbeat.valid", "heartbeat.enabled"},
		},
		{
			ToolID: toolspkg.ToolIDAgentHeartbeatSources,
			Name:   "pending script approval",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentHeartbeatSourcesViews.result(view, catalog)
			},
			MustKeep: []string{
				"agent",
				"scripts_enabled",
				"limits.max_per_agent",
				"limits.max_items",
				"kinds.0.kind",
				"scripts.0.name",
				"scripts.0.approved",
				"scripts.0.executable",
			},
		},
		{
			ToolID: toolspkg.ToolIDAgentHeartbeatSources,
			Name:   "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return agentHeartbeatSourcesViews.result(
					view,
					contract.HeartbeatSourceCatalogPayload{
						Agent:   "reviewer",
						Kinds:   []source.KindDescriptor{},
						Scripts: []contract.HeartbeatSourceScriptPayload{},
					},
				)
			},
			MustKeep: []string{"kinds", "scripts", "limits.max_per_agent", "limits.max_items"},
		},
	}
}
