package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func TestSessionPayloadFromInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should map session info into a sanitized session payload", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
		ttl := now.Add(time.Hour)
		payload := core.SessionPayloadFromInfo(&session.Info{
			ID:                   "sess-1",
			Name:                 "demo",
			AgentName:            "coder",
			Provider:             "fake",
			Model:                "  gpt-test  ",
			ReasoningEffort:      "  high  ",
			WorkspaceID:          "ws_alpha",
			Workspace:            "/workspace",
			NetworkParticipation: testLiveParticipation("ws_alpha", "builders"),
			Type:                 session.SessionTypeDream,
			Lineage: &store.SessionLineage{
				ParentSessionID:  "sess-root",
				RootSessionID:    "sess-root",
				SpawnDepth:       1,
				SpawnRole:        "worker",
				TTLExpiresAt:     &ttl,
				AutoStopOnParent: true,
				SpawnBudget: store.SessionSpawnBudget{
					MaxChildren:           2,
					MaxDepth:              1,
					TTLSeconds:            3600,
					MaxActivePerWorkspace: 3,
				},
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:           []string{"edit"},
					NetworkChannels: []string{"coord"},
				},
			},
			State:      session.StateActive,
			StopReason: store.StopTimeout,
			StopDetail: "deadline exceeded",
			Failure: &store.SessionFailure{
				Kind:            store.FailureTimeout,
				Summary:         "deadline exceeded",
				CrashBundlePath: "/tmp/agh-crash.json",
			},
			ACPSessionID: "acp-123",
			AdvertisedCommands: []store.SessionAdvertisedCommand{{
				Name:        "compact",
				Description: "Compact context",
				Input:       &store.SessionAdvertisedCommandInput{Hint: "optional focus"},
			}},
			Sandbox: &store.SessionSandboxMeta{
				SandboxID:     "env-1",
				Backend:       "local",
				Profile:       "local",
				State:         "prepared",
				InstanceID:    "instance-1",
				ProviderState: json.RawMessage(`{"sandbox_id":"sb-123","token":"secret"}`),
				LastSyncError: "sync failed",
			},
			Liveness: &store.SessionLivenessMeta{
				Activity: &store.SessionActivityMeta{
					TurnID: "turn-1",
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
			ACPCaps: acp.Caps{
				SupportsLoadSession: true,
				SupportedModes:      []string{"chat"},
				ConfigOptions: []acp.SessionConfigOption{
					{
						ID:      "reasoning_effort",
						Label:   "Reasoning effort",
						Kind:    acp.SessionConfigOptionKindSelect,
						Current: "high",
						Values: []acp.SessionConfigOptionValue{
							{Value: "low", Label: "Low"},
							{Value: "high", Label: "High"},
						},
					},
				},
			},
		})

		if payload.ID != "sess-1" || payload.WorkspaceID != "ws_alpha" || payload.WorkspacePath != "/workspace" ||
			resolvedParticipationChannelID(payload.ResolvedNetworkParticipation) != "builders" {
			t.Fatalf("payload = %#v", payload)
		}
		wantParticipation := testLiveParticipation("ws_alpha", "builders")
		if payload.ResolvedNetworkParticipation == nil || *payload.ResolvedNetworkParticipation != wantParticipation {
			t.Fatalf(
				"payload.ResolvedNetworkParticipation = %#v, want %#v",
				payload.ResolvedNetworkParticipation,
				wantParticipation,
			)
		}
		if payload.Provider != "fake" {
			t.Fatalf("payload.Provider = %q, want %q", payload.Provider, "fake")
		}
		if payload.Model != "gpt-test" {
			t.Fatalf("payload.Model = %q, want %q", payload.Model, "gpt-test")
		}
		if payload.ReasoningEffort != "high" {
			t.Fatalf("payload.ReasoningEffort = %q, want %q", payload.ReasoningEffort, "high")
		}
		if payload.State != session.StateActive || payload.ACPSessionID != "acp-123" {
			t.Fatalf("payload session fields = %#v", payload)
		}
		if payload.Type != session.SessionTypeDream {
			t.Fatalf("payload.Type = %q, want %q", payload.Type, session.SessionTypeDream)
		}
		if payload.Lineage == nil ||
			payload.Lineage.ParentSessionID != "sess-root" ||
			payload.Lineage.RootSessionID != "sess-root" ||
			payload.Lineage.SpawnDepth != 1 ||
			payload.Lineage.SpawnRole != "worker" ||
			payload.Lineage.SpawnBudget.MaxChildren != 2 ||
			payload.Lineage.PermissionPolicy.Tools[0] != "edit" {
			t.Fatalf("payload.Lineage = %#v", payload.Lineage)
		}
		if payload.Activity == nil || payload.Activity.TurnID != "turn-1" {
			t.Fatalf("activity = %#v", payload.Activity)
		}
		if payload.StopReason != store.StopTimeout || payload.StopDetail != "deadline exceeded" {
			t.Fatalf("payload stop fields = %#v", payload)
		}
		if payload.Failure == nil ||
			payload.Failure.Kind != store.FailureTimeout ||
			payload.Failure.Summary != "deadline exceeded" ||
			payload.Failure.CrashBundlePath != "/tmp/agh-crash.json" {
			t.Fatalf("payload failure = %#v", payload.Failure)
		}
		if payload.ACPCaps == nil || !payload.ACPCaps.SupportsLoadSession {
			t.Fatalf("caps = %#v", payload.ACPCaps)
		}
		if len(payload.AvailableCommands) != 1 || payload.AvailableCommands[0].Name != "compact" ||
			payload.AvailableCommands[0].Input == nil ||
			payload.AvailableCommands[0].Input.Hint != "optional focus" {
			t.Fatalf("payload.AvailableCommands = %#v", payload.AvailableCommands)
		}
		if got := payload.ACPCaps.SupportedModes; len(got) != 1 || got[0] != "chat" {
			t.Fatalf("supported modes = %#v, want [chat]", got)
		}
		if len(payload.ACPCaps.ConfigOptions) != 1 {
			t.Fatalf("config options = %#v", payload.ACPCaps.ConfigOptions)
		}
		if got := payload.ACPCaps.ConfigOptions[0]; got.ID != "reasoning_effort" || got.Current != "high" ||
			got.Kind != "select" || len(got.Values) != 2 {
			t.Fatalf("config option payload = %#v", got)
		}
		if got := payload.ACPCaps.ConfigOptions[0].Values; got[0].Value != "low" || got[1].Value != "high" {
			t.Fatalf("config option values = %#v, want [low high]", got)
		}
		if payload.Sandbox == nil || payload.Sandbox.SandboxID != "env-1" ||
			payload.Sandbox.Backend != "local" ||
			payload.Sandbox.Profile != "local" ||
			payload.Sandbox.State != "prepared" ||
			payload.Sandbox.InstanceID != "instance-1" ||
			payload.Sandbox.LastSyncError != "sync failed" {
			t.Fatalf("sandbox = %#v", payload.Sandbox)
		}
		if payload.Sandbox.ProviderStateJSON != nil {
			t.Fatalf("sandbox provider state = %s, want omitted", string(payload.Sandbox.ProviderStateJSON))
		}
	})

	t.Run("Should mark stalled session payloads as hung and not attachable", func(t *testing.T) {
		t.Parallel()

		payload := core.SessionPayloadFromInfo(&session.Info{
			ID:        "sess-stalled",
			AgentName: "coder",
			State:     session.StateActive,
			Liveness: &store.SessionLivenessMeta{
				StallState:  store.SessionStallStateDetected,
				StallReason: store.SessionStallReasonActivityTimeout,
			},
		})
		if payload.Badge != session.BadgeHung {
			t.Fatalf("payload.Badge = %q, want %q", payload.Badge, session.BadgeHung)
		}
		if payload.Attachable {
			t.Fatalf("payload.Attachable = true, want false for stalled session")
		}
	})
}

func TestRuntimeActivityPayloadFromSessionMeta(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	turnStartedAt := now.Add(-2 * time.Minute)
	lastActivityAt := now.Add(-45 * time.Second)
	lastProgressAt := now.Add(-15 * time.Second)

	tests := []struct {
		name   string
		meta   *store.SessionLivenessMeta
		assert func(*testing.T, *contract.RuntimeActivityPayload)
	}{
		{
			name: "Should return nil for nil liveness metadata",
			assert: func(t *testing.T, payload *contract.RuntimeActivityPayload) {
				t.Helper()
				if payload != nil {
					t.Fatalf("RuntimeActivityPayloadFromSessionMeta(nil) = %#v, want nil", payload)
				}
			},
		},
		{
			name: "Should return nil for empty liveness metadata",
			meta: &store.SessionLivenessMeta{},
			assert: func(t *testing.T, payload *contract.RuntimeActivityPayload) {
				t.Helper()
				if payload != nil {
					t.Fatalf("RuntimeActivityPayloadFromSessionMeta(empty) = %#v, want nil", payload)
				}
			},
		},
		{
			name: "Should map populated activity metadata",
			meta: &store.SessionLivenessMeta{
				Activity: &store.SessionActivityMeta{
					TurnID:             " turn-1 ",
					TurnSource:         " prompt ",
					TurnStartedAt:      &turnStartedAt,
					LastActivityAt:     &lastActivityAt,
					LastActivityKind:   " tool_call ",
					LastActivityDetail: " running ",
					CurrentTool:        " edit ",
					ToolCallID:         " call-1 ",
					LastProgressAt:     &lastProgressAt,
					IterationCurrent:   2,
					IterationMax:       5,
				},
			},
			assert: func(t *testing.T, payload *contract.RuntimeActivityPayload) {
				t.Helper()
				if payload == nil {
					t.Fatal("RuntimeActivityPayloadFromSessionMeta() = nil, want payload")
					return
				}
				if payload.TurnID != "turn-1" ||
					payload.TurnSource != "prompt" ||
					payload.LastActivityKind != "tool_call" ||
					payload.CurrentTool != "edit" ||
					payload.ToolCallID != "call-1" ||
					payload.IterationCurrent != 2 ||
					payload.IterationMax != 5 {
					t.Fatalf("payload = %#v", payload)
				}
				if payload.IdleSeconds != 45 || payload.ElapsedSeconds != 120 {
					t.Fatalf("payload timing = idle %d elapsed %d", payload.IdleSeconds, payload.ElapsedSeconds)
				}
				if payload.TurnStartedAt == nil || !payload.TurnStartedAt.Equal(turnStartedAt) ||
					payload.LastActivityAt == nil || !payload.LastActivityAt.Equal(lastActivityAt) ||
					payload.LastProgressAt == nil || !payload.LastProgressAt.Equal(lastProgressAt) {
					t.Fatalf("payload time pointers = %#v", payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := core.RuntimeActivityPayloadFromSessionMeta(tt.meta, now)
			tt.assert(t, payload)
		})
	}
}

func TestAgentPayloadFromDef(t *testing.T) {
	t.Parallel()

	t.Run("Should convert agent definition to payload", func(t *testing.T) {
		t.Parallel()

		payload := core.AgentPayloadFromDef(aghconfig.AgentDef{
			Name:        "coder",
			Provider:    "fake",
			Command:     "codex",
			Model:       "gpt-test",
			Tools:       []string{"agh__skill_view"},
			Toolsets:    []string{"agh__catalog"},
			DenyTools:   []string{"agh__task_*"},
			Permissions: "approve-reads",
			Prompt:      "hello",
			MCPServers: []aghconfig.MCPServer{{
				Name:    "memory",
				Command: "memoryd",
				Args:    []string{"serve"},
				Env:     map[string]string{"TOKEN": "secret"},
			}},
		})

		if payload.Name != "coder" || payload.Provider != "fake" || len(payload.MCPServers) != 1 {
			t.Fatalf("payload = %#v", payload)
		}
		if got, want := strings.Join(payload.Tools, ","), "agh__skill_view"; got != want {
			t.Fatalf("payload tools = %#v, want %q", payload.Tools, want)
		}
		if got, want := strings.Join(payload.Toolsets, ","), "agh__catalog"; got != want {
			t.Fatalf("payload toolsets = %#v, want %q", payload.Toolsets, want)
		}
		if got, want := strings.Join(payload.DenyTools, ","), "agh__task_*"; got != want {
			t.Fatalf("payload deny_tools = %#v, want %q", payload.DenyTools, want)
		}
		if payload.MCPServers[0].Env["TOKEN"] != aghconfig.RedactedValue() {
			t.Fatalf("payload mcp env = %#v", payload.MCPServers[0].Env)
		}
	})
}

func TestSessionEventPayloadFromEventIncludesStopDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should include stop diagnostics from session info", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
		payload := core.SessionEventPayloadFromEvent(
			store.SessionEvent{
				ID:        "ev-1",
				SessionID: "sess-1",
				Sequence:  7,
				TurnID:    "turn-1",
				Type:      session.EventTypeSessionStopped,
				AgentName: "coder",
				Content:   `{"ok":true}`,
				Timestamp: now,
			},
			&session.Info{
				WorkspaceID: "ws-alpha",
				Workspace:   "/workspace",
				Lineage: &store.SessionLineage{
					ParentSessionID: "sess-parent",
					RootSessionID:   "sess-root",
					SpawnDepth:      2,
				},
				StopReason: store.StopAgentCrashed,
				StopDetail: "driver failed",
				Failure: &store.SessionFailure{
					Kind:    store.FailureProcess,
					Summary: "driver failed",
				},
			},
		)

		if payload.WorkspaceID != "ws-alpha" || payload.WorkspacePath != "/workspace" {
			t.Fatalf("workspace payload = %#v", payload)
		}
		if payload.ParentSessionID != "sess-parent" ||
			payload.RootSessionID != "sess-root" ||
			payload.SpawnDepth != 2 {
			t.Fatalf("lineage payload = %#v", payload)
		}
		if payload.StopReason != store.StopAgentCrashed || payload.StopDetail != "driver failed" {
			t.Fatalf("stop payload = %#v", payload)
		}
		if payload.Failure == nil || payload.Failure.Kind != store.FailureProcess {
			t.Fatalf("failure payload = %#v", payload.Failure)
		}
	})

	t.Run("Should preserve correlation when unrelated payload fields are malformed", func(t *testing.T) {
		t.Parallel()

		leaseUntil := time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC)
		payload := core.SessionEventPayloadFromEvent(
			store.SessionEvent{
				ID:        "ev-correlation",
				SessionID: "sess-1",
				Sequence:  8,
				TurnID:    "turn-2",
				Type:      "tool_call",
				AgentName: "coder",
				Content:   `{"type":"tool_call","task_id":"task-1","claim_token_hash":"agh_claim_hash_123","lease_until":"2026-04-03T12:05:00Z","timestamp":"not-a-time"}`,
				Timestamp: time.Date(2026, 4, 3, 12, 1, 0, 0, time.UTC),
			},
			nil,
		)

		if got, want := payload.TaskID, "task-1"; got != want {
			t.Fatalf("payload.TaskID = %q, want %q", got, want)
		}
		if got, want := payload.ClaimTokenHash, "agh_claim_hash_123"; got != want {
			t.Fatalf("payload.ClaimTokenHash = %q, want %q", got, want)
		}
		if payload.LeaseUntil == nil || !payload.LeaseUntil.Equal(leaseUntil) {
			t.Fatalf("payload.LeaseUntil = %v, want %v", payload.LeaseUntil, leaseUntil)
		}
	})

	t.Run("Should omit lease until when correlation has no lease", func(t *testing.T) {
		t.Parallel()

		payload := core.SessionEventPayloadFromEvent(
			store.SessionEvent{
				ID:        "ev-no-lease",
				SessionID: "sess-1",
				Sequence:  9,
				TurnID:    "turn-3",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"type":"agent_message","task_id":"task-1"}`,
				Timestamp: time.Date(2026, 4, 3, 12, 2, 0, 0, time.UTC),
			},
			nil,
		)

		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(raw), `"lease_until"`) {
			t.Fatalf("payload JSON = %s, want lease_until to be omitted", raw)
		}
	})
}

func TestLogEventPayloadFromSummaryIncludesLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should include persisted observe lineage fields", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC)
		payload := core.LogEventPayloadFromSummary(store.EventSummary{
			ID:              "sum-1",
			SessionID:       "sess-child",
			Type:            "agent_message",
			AgentName:       "coder",
			Provider:        "codex",
			Outcome:         "info",
			ParentSessionID: "sess-parent",
			RootSessionID:   "sess-root",
			SpawnDepth:      1,
			Summary:         "hello",
			Timestamp:       now,
		})

		if payload.ParentSessionID != "sess-parent" ||
			payload.RootSessionID != "sess-root" ||
			payload.SpawnDepth != 1 ||
			payload.Provider != "codex" ||
			payload.Outcome != "info" ||
			payload.Component != "session" {
			t.Fatalf("payload lineage = %#v", payload)
		}
	})

	t.Run("Should preserve observe event content for global events", func(t *testing.T) {
		t.Parallel()

		payload := core.LogEventPayloadFromSummary(store.EventSummary{
			ID:      "sum-global",
			Type:    "settings.changed",
			Content: []byte(`{"section":"skills","source":"http","operation":"patch"}`),
		})

		if got, want := string(
			payload.Content,
		), `{"section":"skills","source":"http","operation":"patch"}`; got != want {
			t.Fatalf("payload.Content = %q, want %q", got, want)
		}
	})
}

func TestJobPayloadFromJobCopiesNestedOptionalFields(t *testing.T) {
	t.Parallel()

	t.Run("Should copy nested optional automation job fields", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
		nextRun := now.Add(time.Hour)
		schedule := automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeEvery, Interval: "10m"}
		owner := taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"}
		jobTask := automationpkg.JobTaskConfig{
			Title:                "Review queue",
			Owner:                &owner,
			NetworkParticipation: testNamedParticipationRequest("builders"),
		}
		payload := core.JobPayloadFromJob(automationpkg.Job{
			ID:        "job-1",
			Scope:     automationpkg.AutomationScopeWorkspace,
			Name:      "review",
			AgentName: "coder",
			Schedule:  &schedule,
			Task:      &jobTask,
			Enabled:   true,
			Source:    automationpkg.JobSourceDynamic,
			CreatedAt: now,
			UpdatedAt: now,
		}, &nextRun, &contract.AutomationSchedulerStatePayload{Registered: true})

		if payload.Schedule == nil || payload.Schedule.Interval != "10m" {
			t.Fatalf("schedule payload = %#v", payload.Schedule)
		}
		if payload.Schedule == &schedule {
			t.Fatal("JobPayloadFromJob reused schedule input pointer")
		}
		if payload.Task == nil || payload.Task.Owner == nil || payload.Task.Owner.Ref != "triage" {
			t.Fatalf("task payload = %#v", payload.Task)
		}
		if payload.Task == &jobTask || payload.Task.Owner == &owner {
			t.Fatal("JobPayloadFromJob reused nested input pointers")
		}
		if payload.NextRun == nil || !payload.NextRun.Equal(nextRun) || payload.Scheduler == nil {
			t.Fatalf("scheduler payload = next %v scheduler %#v", payload.NextRun, payload.Scheduler)
		}
	})
}

func TestParseSessionEventQueryAndHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should parse event and logs helper query parameters", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		ginCtx.Request = httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/events?type=agent_message&agent_name=coder&turn_id=turn-1&after_sequence=5&limit=10&since=2026-04-03T12:00:00Z&run=run-1&actor_kind=agent&actor_id=agent:coder&provider=codex&outcome=failure&component=task&error_only=true",
			http.NoBody,
		)

		query, err := core.ParseSessionEventQuery(ginCtx)
		if err != nil {
			t.Fatalf("ParseSessionEventQuery() error = %v", err)
		}
		if query.Type != "agent_message" || query.AgentName != "coder" || query.TurnID != "turn-1" ||
			query.AfterSequence != 5 ||
			query.Limit != 10 {
			t.Fatalf("query = %#v", query)
		}

		if _, err := core.ParseOptionalTime(""); err != nil {
			t.Fatalf("ParseOptionalTime(empty) error = %v", err)
		}
		if parsed, err := core.ParseOptionalTime("2026-04-03T12:00:00Z"); err != nil || parsed.IsZero() {
			t.Fatalf("ParseOptionalTime(valid) = %v, %v", parsed, err)
		}
		if _, err := core.ParseOptionalTime("bad"); err == nil {
			t.Fatal("ParseOptionalTime(bad) error = nil, want non-nil")
		}
		if value, err := core.ParseOptionalInt("7"); err != nil || value != 7 {
			t.Fatalf("ParseOptionalInt() = %d, %v", value, err)
		}
		if value, err := core.ParseOptionalInt64("9"); err != nil || value != 9 {
			t.Fatalf("ParseOptionalInt64() = %d, %v", value, err)
		}
		if _, err := core.ParseLogsCursor("2026-04-03T12:00:00Z|ev-1"); err != nil {
			t.Fatalf("ParseLogsCursor() error = %v", err)
		}
		observeQuery, err := core.ParseLogsQuery(ginCtx)
		if err != nil {
			t.Fatalf("ParseLogsQuery() error = %v", err)
		}
		if observeQuery.AgentName != "coder" ||
			observeQuery.RunID != "run-1" ||
			observeQuery.ActorKind != "agent" ||
			observeQuery.ActorID != "agent:coder" ||
			observeQuery.Provider != "codex" ||
			observeQuery.Outcome != "failure" ||
			observeQuery.Component != "task" ||
			!observeQuery.ErrorOnly ||
			observeQuery.AfterSequence != 5 {
			t.Fatalf("observe query = %#v", observeQuery)
		}

		invalidRecorder := httptest.NewRecorder()
		invalidContext, _ := gin.CreateTestContext(invalidRecorder)
		invalidContext.Request = httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/events?since=bad",
			http.NoBody,
		)
		if _, err := core.ParseSessionEventQuery(invalidContext); err == nil {
			t.Fatal("ParseSessionEventQuery(invalid) error = nil, want non-nil")
		}
	})
}

func TestSkillShadowEntryPayloadFromDomain(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize domain tiers and preserve zero detected timestamps", func(t *testing.T) {
		t.Parallel()

		payload := core.SkillShadowEntryPayloadFromDomain(skills.ShadowEntry{
			Path:             " /tmp/skill/SKILL.md ",
			Tier:             "agent-local",
			ResolvedToWinner: true,
		})
		if payload.Path != "/tmp/skill/SKILL.md" {
			t.Fatalf("Path = %q, want %q", payload.Path, "/tmp/skill/SKILL.md")
		}
		if payload.Tier != "agent_local" {
			t.Fatalf("Tier = %q, want %q", payload.Tier, "agent_local")
		}
		if !payload.DetectedAt.IsZero() {
			t.Fatalf("DetectedAt = %v, want zero time", payload.DetectedAt)
		}
	})
}

func TestRespondErrorMaskingModes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		mask    bool
		wantErr string
	}{
		{name: "mask", mask: true, wantErr: http.StatusText(http.StatusInternalServerError)},
		{name: "expose", mask: false, wantErr: "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			ginCtx, _ := gin.CreateTestContext(recorder)

			core.RespondError(ginCtx, http.StatusInternalServerError, errors.New("boom"), tc.mask)

			var payload contract.ErrorPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if payload.Error != tc.wantErr {
				t.Fatalf("payload.Error = %q, want %q", payload.Error, tc.wantErr)
			}
		})
	}
}

func TestPrepareSSESetsHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should prepare SSE responses with streaming headers", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		ginCtx.Request = httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/stream",
			http.NoBody,
		)

		writer, err := core.PrepareSSE(ginCtx)
		if err != nil {
			t.Fatalf("PrepareSSE() error = %v", err)
		}
		if writer == nil {
			t.Fatal("PrepareSSE() writer = nil")
		}
		if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("Content-Type = %q, want text/event-stream", got)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("Cache-Control = %q, want no-cache", got)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})
}
