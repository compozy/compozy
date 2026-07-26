package extensionpkg

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
)

func TestHostAPIHandlerAuthoredContextSoulGrantsAndManagedWrites(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	soulAuthoring := &hostAPITestSoulAuthoring{
		result: hostAPITestSoulMutationResult(env.workspaceID, "coder", env.workspace.RootDir),
	}
	env.handler.soulAuthoring = soulAuthoring

	env.grant("ext-soul-read", []string{
		string(extensioncontract.HostAPIMethodAgentsSoulGet),
	}, []string{"soul.read"})

	_, err := env.handler.Handle(
		t.Context(),
		"ext-soul-read",
		string(extensioncontract.HostAPIMethodAgentsSoulPut),
		mustHostAPIAuthoredJSON(t, apicontract.AgentSoulPutRequest{
			WorkspaceID:    env.workspaceID,
			AgentName:      "coder",
			Body:           "new soul",
			ExpectedDigest: "soul-digest",
		}),
	)
	assertCapabilityDenied(t, err, string(extensioncontract.HostAPIMethodAgentsSoulPut))
	if soulAuthoring.putCalls != 0 {
		t.Fatalf("soul put calls = %d, want 0 when write grant is missing", soulAuthoring.putCalls)
	}

	getResult, err := env.handler.Handle(
		t.Context(),
		"ext-soul-read",
		string(extensioncontract.HostAPIMethodAgentsSoulGet),
		mustHostAPIAuthoredJSON(t, extensioncontract.AgentSoulGetParams{
			WorkspaceID: env.workspaceID,
			AgentName:   "coder",
		}),
	)
	if err != nil {
		t.Fatalf("Handle(agents/soul/get) error = %v", err)
	}
	var soulPayload apicontract.AgentSoulPayload
	decodeResult(t, getResult, &soulPayload)
	if soulPayload.Digest != "soul-digest" || soulPayload.Body != "bounded soul body" {
		t.Fatalf("soul payload = %#v, want managed read model", soulPayload)
	}

	env.grant("ext-soul-write", []string{
		string(extensioncontract.HostAPIMethodAgentsSoulPut),
	}, []string{"soul.write"})
	putResult, err := env.handler.Handle(
		t.Context(),
		"ext-soul-write",
		string(extensioncontract.HostAPIMethodAgentsSoulPut),
		mustHostAPIAuthoredJSON(t, apicontract.AgentSoulPutRequest{
			WorkspaceID:    env.workspaceID,
			AgentName:      "coder",
			Body:           "managed update",
			ExpectedDigest: "soul-digest",
		}),
	)
	if err != nil {
		t.Fatalf("Handle(agents/soul/put) error = %v", err)
	}
	var mutation apicontract.AgentSoulMutationResponse
	decodeResult(t, putResult, &mutation)
	if mutation.Revision.ID != "sr-1" || mutation.Soul.RevisionID != "sr-1" {
		t.Fatalf("soul mutation = %#v, want managed revision response", mutation)
	}
	if soulAuthoring.lastPut.ExpectedDigest != "soul-digest" ||
		soulAuthoring.lastPut.Actor.Kind != "extension" ||
		soulAuthoring.lastPut.Actor.Ref != "ext-soul-write" ||
		soulAuthoring.lastPut.Origin.Kind != "host_api" ||
		soulAuthoring.lastPut.Origin.Ref != string(extensioncontract.HostAPIMethodAgentsSoulPut) {
		t.Fatalf("soul put request = %#v, want extension actor and host_api origin", soulAuthoring.lastPut)
	}
	if !strings.HasSuffix(soulAuthoring.lastPut.Target.AgentPath, ".agh/agents/coder/AGENT.md") {
		t.Fatalf("soul target path = %q, want managed agent path", soulAuthoring.lastPut.Target.AgentPath)
	}
}

func TestHostAPIHandlerSessionsSoulRefreshRequiresWorkspaceOwnership(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh soul only for owned workspace session", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		sess := env.createSession(t)
		mutation := hostAPITestSoulMutationResult(env.workspaceID, "coder", env.workspace.RootDir)
		refresher := &hostAPITestSoulRefresher{
			result: session.SoulRefreshResult{
				SessionID:  sess.ID,
				AgentName:  "coder",
				Snapshot:   &mutation.Snapshot,
				Soul:       &mutation.Soul,
				SoulDigest: mutation.Soul.Digest,
			},
		}
		env.handler.soulRefresher = refresher
		env.grant("ext-soul-refresh", []string{
			string(extensioncontract.HostAPIMethodSessionsSoulRefresh),
		}, []string{"soul.write"})

		result, err := env.handler.Handle(
			t.Context(),
			"ext-soul-refresh",
			string(extensioncontract.HostAPIMethodSessionsSoulRefresh),
			mustHostAPIAuthoredJSON(t, extensioncontract.SessionSoulRefreshParams{
				WorkspaceID: env.workspaceID,
				SessionID:   sess.ID,
				SessionSoulRefreshRequest: apicontract.SessionSoulRefreshRequest{
					ExpectedDigest: "soul-digest",
				},
			}),
		)
		if err != nil {
			t.Fatalf("Handle(sessions/soul/refresh) error = %v", err)
		}
		var payload apicontract.AgentSoulPayload
		decodeResult(t, result, &payload)
		if payload.AgentName != "coder" || payload.Digest != "soul-digest" {
			t.Fatalf("soul refresh payload = %#v, want coder soul digest", payload)
		}
		if refresher.calls != 1 || refresher.lastSessionID != sess.ID || refresher.lastDigest != "soul-digest" {
			t.Fatalf(
				"soul refresh call = (%d, %q, %q), want one owned session refresh",
				refresher.calls,
				refresher.lastSessionID,
				refresher.lastDigest,
			)
		}
	})

	t.Run("Should reject soul refresh from foreign workspace", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		sess := env.createSession(t)
		refresher := &hostAPITestSoulRefresher{}
		env.handler.soulRefresher = refresher
		env.grant("ext-soul-refresh", []string{
			string(extensioncontract.HostAPIMethodSessionsSoulRefresh),
		}, []string{"soul.write"})
		foreign := env.addForeignWorkspace(t)

		_, err := env.handler.Handle(
			t.Context(),
			"ext-soul-refresh",
			string(extensioncontract.HostAPIMethodSessionsSoulRefresh),
			mustHostAPIAuthoredJSON(t, extensioncontract.SessionSoulRefreshParams{
				WorkspaceID: foreign.WorkspaceID,
				SessionID:   sess.ID,
				SessionSoulRefreshRequest: apicontract.SessionSoulRefreshRequest{
					ExpectedDigest: "soul-digest",
				},
			}),
		)
		assertRPCErrorCode(t, err, HostAPINotFoundCode)
		if refresher.calls != 0 {
			t.Fatalf("soul refresh calls = %d, want unchanged after foreign workspace rejection", refresher.calls)
		}
	})
}

func TestHostAPIHandlerAuthoredContextHeartbeatHealthAndWake(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	now := time.Date(2026, 4, 29, 15, 0, 0, 0, time.UTC)
	sess := env.createSession(t)
	health := heartbeat.SessionHealth{
		SessionID:       sess.ID,
		WorkspaceID:     env.workspaceID,
		AgentName:       "coder",
		State:           heartbeat.SessionHealthStateIdle,
		Health:          heartbeat.SessionHealthHealthy,
		Attachable:      true,
		EligibleForWake: true,
		UpdatedAt:       now,
	}
	heartbeatStatus := &hostAPITestHeartbeatStatus{
		result: heartbeat.StatusResult{
			AgentName:    "coder",
			Enabled:      true,
			Present:      true,
			Active:       true,
			Valid:        true,
			Digest:       "hb-digest",
			ConfigDigest: "cfg-digest",
			SnapshotID:   "hbs-1",
			Summary:      "managed status",
			WakeState: &heartbeat.WakeState{
				WorkspaceID:      env.workspaceID,
				AgentName:        "coder",
				SessionID:        sess.ID,
				PolicySnapshotID: "hbs-1",
				LastResult:       heartbeat.WakeResultSkipped,
				LastReason:       heartbeat.WakeReasonQuietWindow,
				UpdatedAt:        now,
			},
			SessionHealth: &health,
		},
	}
	env.handler.heartbeatStatus = heartbeatStatus
	wake := &hostAPITestHeartbeatWake{
		decision: heartbeat.WakeDecision{
			WakeEventID:      "hwe-host",
			Result:           heartbeat.WakeResultSkipped,
			Reason:           heartbeat.WakeReasonSessionUnhealthy,
			PolicySnapshotID: "hbs-1",
			PolicyDigest:     "hb-digest",
			ConfigDigest:     "cfg-digest",
		},
	}
	env.handler.heartbeatWake = wake
	env.handler.sessionHealth = hostAPITestSessionHealth{health: health}
	env.handler.wakeEvents = hostAPITestWakeEvents{events: []heartbeat.WakeEvent{{
		ID:               "hwe-history",
		WorkspaceID:      env.workspaceID,
		AgentName:        "coder",
		SessionID:        sess.ID,
		PolicySnapshotID: "hbs-1",
		Source:           heartbeat.WakeSourceManual,
		Result:           heartbeat.WakeResultSkipped,
		Reason:           heartbeat.WakeReasonQuietWindow,
		CreatedAt:        now,
		ExpiresAt:        now.Add(time.Hour),
	}}}

	env.grant("ext-heartbeat-read", []string{
		string(extensioncontract.HostAPIMethodAgentsHeartbeatStatus),
	}, []string{"heartbeat.read"})
	_, err := env.handler.Handle(
		t.Context(),
		"ext-heartbeat-read",
		string(extensioncontract.HostAPIMethodAgentsHeartbeatWake),
		mustHostAPIAuthoredJSON(t, apicontract.HeartbeatWakeRequest{
			WorkspaceID: env.workspaceID,
			AgentName:   "coder",
			SessionID:   sess.ID,
			Source:      apicontract.HeartbeatWakeSourceManual,
		}),
	)
	assertCapabilityDenied(t, err, string(extensioncontract.HostAPIMethodAgentsHeartbeatWake))
	if wake.calls != 0 {
		t.Fatalf("heartbeat wake calls = %d, want 0 when wake grant is missing", wake.calls)
	}

	statusResult, err := env.handler.Handle(
		t.Context(),
		"ext-heartbeat-read",
		string(extensioncontract.HostAPIMethodAgentsHeartbeatStatus),
		mustHostAPIAuthoredJSON(t, apicontract.HeartbeatStatusRequest{
			WorkspaceID:             env.workspaceID,
			AgentName:               "coder",
			SessionID:               sess.ID,
			IncludeSessionHealth:    true,
			IncludeRecentWakeEvents: true,
		}),
	)
	if err != nil {
		t.Fatalf("Handle(agents/heartbeat/status) error = %v", err)
	}
	var status apicontract.HeartbeatStatusResponse
	decodeResult(t, statusResult, &status)
	if status.SnapshotID != "hbs-1" || status.SessionHealth == nil || len(status.WakeEvents) != 1 {
		t.Fatalf("heartbeat status = %#v, want policy, health, and wake audit", status)
	}
	if heartbeatStatus.calls != 1 {
		t.Fatalf("heartbeat status calls = %d, want 1 after owned session status", heartbeatStatus.calls)
	}

	foreign := env.addForeignWorkspace(t)
	_, err = env.handler.Handle(
		t.Context(),
		"ext-heartbeat-read",
		string(extensioncontract.HostAPIMethodAgentsHeartbeatStatus),
		mustHostAPIAuthoredJSON(t, apicontract.HeartbeatStatusRequest{
			WorkspaceID:             foreign.WorkspaceID,
			AgentName:               "coder",
			SessionID:               sess.ID,
			IncludeSessionHealth:    true,
			IncludeRecentWakeEvents: true,
		}),
	)
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
	if heartbeatStatus.calls != 1 {
		t.Fatalf("heartbeat status calls = %d, want unchanged after foreign workspace rejection", heartbeatStatus.calls)
	}

	env.grant("ext-runtime-read", []string{
		string(extensioncontract.HostAPIMethodSessionsHealthGet),
		string(extensioncontract.HostAPIMethodSessionsStatusGet),
	}, []string{"session.read"})
	healthResult, err := env.handler.Handle(
		t.Context(),
		"ext-runtime-read",
		string(extensioncontract.HostAPIMethodSessionsHealthGet),
		mustHostAPIAuthoredJSON(t, extensioncontract.SessionHealthGetParams{
			WorkspaceID: env.workspaceID,
			SessionID:   sess.ID,
		}),
	)
	if err != nil {
		t.Fatalf("Handle(sessions/health/get) error = %v", err)
	}
	var healthResponse apicontract.SessionHealthResponse
	decodeResult(t, healthResult, &healthResponse)
	if !healthResponse.Health.EligibleForWake {
		t.Fatalf("session health = %#v, want eligible managed health", healthResponse.Health)
	}

	statusGetResult, err := env.handler.Handle(
		t.Context(),
		"ext-runtime-read",
		string(extensioncontract.HostAPIMethodSessionsStatusGet),
		mustHostAPIAuthoredJSON(t, extensioncontract.SessionStatusGetParams{
			WorkspaceID: env.workspaceID,
			SessionID:   sess.ID,
		}),
	)
	if err != nil {
		t.Fatalf("Handle(sessions/status/get) error = %v", err)
	}
	var statusGetResponse apicontract.SessionStatusResponse
	decodeResult(t, statusGetResult, &statusGetResponse)
	if statusGetResponse.WorkspaceID != env.workspaceID || statusGetResponse.SessionID != sess.ID {
		t.Fatalf("session status = %#v, want workspace-scoped session status", statusGetResponse)
	}

	env.grant("ext-heartbeat-wake", []string{
		string(extensioncontract.HostAPIMethodAgentsHeartbeatWake),
	}, []string{"heartbeat.write"})
	wakeResult, err := env.handler.Handle(
		t.Context(),
		"ext-heartbeat-wake",
		string(extensioncontract.HostAPIMethodAgentsHeartbeatWake),
		mustHostAPIAuthoredJSON(t, apicontract.HeartbeatWakeRequest{
			WorkspaceID: env.workspaceID,
			AgentName:   "coder",
			SessionID:   sess.ID,
			Source:      apicontract.HeartbeatWakeSourceManual,
			DryRun:      true,
		}),
	)
	if err != nil {
		t.Fatalf("Handle(agents/heartbeat/wake) error = %v", err)
	}
	var wakeResponse apicontract.HeartbeatWakeResponse
	decodeResult(t, wakeResult, &wakeResponse)
	if wakeResponse.Decision.WakeEventID != "hwe-host" {
		t.Fatalf("heartbeat wake = %#v, want managed wake decision", wakeResponse)
	}
	if wake.last.WorkspaceID != env.workspaceID ||
		wake.last.AgentName != "coder" ||
		wake.last.SessionID != sess.ID ||
		wake.last.Source != heartbeat.WakeSourceManual ||
		!wake.last.DryRun {
		t.Fatalf("heartbeat wake request = %#v, want managed service request", wake.last)
	}
	if wake.calls != 1 {
		t.Fatalf("heartbeat wake calls = %d, want 1 after owned session wake", wake.calls)
	}
	_, err = env.handler.Handle(
		t.Context(),
		"ext-heartbeat-wake",
		string(extensioncontract.HostAPIMethodAgentsHeartbeatWake),
		mustHostAPIAuthoredJSON(t, apicontract.HeartbeatWakeRequest{
			WorkspaceID: foreign.WorkspaceID,
			AgentName:   "coder",
			SessionID:   sess.ID,
			Source:      apicontract.HeartbeatWakeSourceManual,
			DryRun:      true,
		}),
	)
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
	if wake.calls != 1 {
		t.Fatalf("heartbeat wake calls = %d, want unchanged after foreign workspace rejection", wake.calls)
	}
}

func TestHostAPIAuthoredContextDoesNotExposeDirectFileOrSoulResourceBypass(t *testing.T) {
	t.Parallel()

	for _, method := range envHostAPIMethodNamesForBypassTest() {
		lower := strings.ToLower(method)
		if strings.Contains(lower, "soul.md") ||
			strings.Contains(lower, "heartbeat.md") ||
			strings.Contains(lower, "file/write") ||
			strings.Contains(lower, "files/write") {
			t.Fatalf("Host API method %q exposes a direct authored-context file bypass", method)
		}
	}
}

func envHostAPIMethodNamesForBypassTest() []string {
	methods := make([]string, 0, len(extensioncontract.HostAPIMethodSpecs()))
	for _, spec := range extensioncontract.HostAPIMethodSpecs() {
		methods = append(methods, string(spec.Method))
	}
	return methods
}

func mustHostAPIAuthoredJSON(t testing.TB, value any) json.RawMessage {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}

type hostAPITestSoulAuthoring struct {
	result   soul.MutationResult
	putCalls int
	lastPut  soul.PutRequest
}

func (s *hostAPITestSoulAuthoring) Validate(
	context.Context,
	soul.ValidateRequest,
) (soul.ValidateResult, error) {
	return soul.ValidateResult{Soul: s.result.Soul}, nil
}

func (s *hostAPITestSoulAuthoring) Put(
	_ context.Context,
	req soul.PutRequest,
) (soul.MutationResult, error) {
	s.putCalls++
	s.lastPut = req
	return s.result, nil
}

func (s *hostAPITestSoulAuthoring) Delete(context.Context, soul.DeleteRequest) (soul.MutationResult, error) {
	return s.result, nil
}

func (s *hostAPITestSoulAuthoring) History(context.Context, soul.HistoryRequest) (soul.HistoryResult, error) {
	return soul.HistoryResult{Revisions: []soul.Revision{s.result.Revision}}, nil
}

func (s *hostAPITestSoulAuthoring) Rollback(
	context.Context,
	soul.RollbackRequest,
) (soul.MutationResult, error) {
	return s.result, nil
}

type hostAPITestSoulRefresher struct {
	result        session.SoulRefreshResult
	calls         int
	lastSessionID string
	lastDigest    string
}

func (s *hostAPITestSoulRefresher) RefreshSoulWithExpectedDigest(
	_ context.Context,
	sessionID string,
	expectedDigest string,
) (session.SoulRefreshResult, error) {
	s.calls++
	s.lastSessionID = sessionID
	s.lastDigest = expectedDigest
	return s.result, nil
}

type hostAPITestHeartbeatStatus struct {
	result heartbeat.StatusResult
	calls  int
	last   heartbeat.StatusRequest
}

func (s *hostAPITestHeartbeatStatus) Inspect(
	context.Context,
	heartbeat.InspectRequest,
) (heartbeat.InspectResult, error) {
	return heartbeat.InspectResult{}, nil
}

func (s *hostAPITestHeartbeatStatus) Status(
	_ context.Context,
	req heartbeat.StatusRequest,
) (heartbeat.StatusResult, error) {
	s.calls++
	s.last = req
	return s.result, nil
}

type hostAPITestHeartbeatWake struct {
	decision heartbeat.WakeDecision
	last     heartbeat.WakeRequest
	calls    int
}

func (s *hostAPITestHeartbeatWake) Wake(
	_ context.Context,
	req heartbeat.WakeRequest,
) (heartbeat.WakeDecision, error) {
	s.calls++
	s.last = req
	return s.decision, nil
}

type hostAPITestSessionHealth struct {
	health heartbeat.SessionHealth
}

func (s hostAPITestSessionHealth) GetSessionHealth(
	context.Context,
	string,
) (heartbeat.SessionHealth, error) {
	return s.health, nil
}

type hostAPITestWakeEvents struct {
	events []heartbeat.WakeEvent
}

func (s hostAPITestWakeEvents) ListHeartbeatWakeEvents(
	context.Context,
	heartbeat.WakeEventListQuery,
) ([]heartbeat.WakeEvent, error) {
	return append([]heartbeat.WakeEvent(nil), s.events...), nil
}

func hostAPITestSoulMutationResult(workspaceID string, agentName string, workspaceRoot string) soul.MutationResult {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	sourcePath := strings.TrimRight(workspaceRoot, "/") + "/.agh/agents/" + agentName + "/SOUL.md"
	return soul.MutationResult{
		Soul: soul.ResolvedSoul{
			Enabled:    true,
			Present:    true,
			Active:     true,
			Valid:      true,
			SourcePath: sourcePath,
			Digest:     "soul-digest",
			ReadModel: soul.ReadModel{
				Enabled:                true,
				Present:                true,
				Active:                 true,
				Valid:                  true,
				SourcePath:             sourcePath,
				Digest:                 "soul-digest",
				Body:                   "bounded soul body",
				MaxBodyBytes:           65536,
				ContextProjectionBytes: 2048,
			},
		},
		Snapshot: soul.Snapshot{
			ID:          "ss-1",
			WorkspaceID: workspaceID,
			AgentName:   agentName,
			SourcePath:  sourcePath,
			Digest:      "soul-digest",
			CreatedAt:   now,
		},
		Revision: soul.Revision{
			ID:             "sr-1",
			WorkspaceID:    workspaceID,
			AgentName:      agentName,
			SourcePath:     sourcePath,
			Action:         soul.RevisionActionPut,
			PreviousDigest: "old-digest",
			NewDigest:      "soul-digest",
			ActorKind:      "extension",
			ActorID:        "ext-soul-write",
			OriginKind:     "host_api",
			OriginRef:      string(extensioncontract.HostAPIMethodAgentsSoulPut),
			CreatedAt:      now,
		},
	}
}
