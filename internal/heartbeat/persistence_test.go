package heartbeat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestHeartbeatPersistenceSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should create a durable snapshot envelope from a resolved policy", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		resolved, err := Parse(context.Background(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
version: 1
enabled: true
summary: "Inspect state before waking work."
preferences:
  min_interval: "45m"
---
Read current context and avoid duplicate prompts.
`),
			Config: aghconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		createdAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshot, err := SnapshotFromResolved("hb-snapshot", "ws-heartbeat", "coder", &resolved, createdAt)
		if err != nil {
			t.Fatalf("SnapshotFromResolved() error = %v", err)
		}
		if snapshot.SchemaVersion != 1 ||
			snapshot.Digest != resolved.Digest ||
			snapshot.ConfigDigest != resolved.ConfigDigest ||
			snapshot.Body != resolved.GuidanceMarkdown ||
			!snapshot.CreatedAt.Equal(createdAt) {
			t.Fatalf("SnapshotFromResolved() = %#v, want resolver digests/body/timestamp", snapshot)
		}

		envelope, err := snapshot.ResolvedEnvelope()
		if err != nil {
			t.Fatalf("ResolvedEnvelope() error = %v", err)
		}
		if !envelope.Valid || !envelope.Active || envelope.ConfigProvenance.Digest != resolved.ConfigDigest {
			t.Fatalf("ResolvedEnvelope() = %#v, want active valid envelope with config provenance", envelope)
		}
		if envelope.Prompt.Summary != resolved.Prompt.Summary ||
			envelope.Status.Digest != resolved.Status.Digest {
			t.Fatalf(
				"ResolvedEnvelope() prompt/status = %#v/%#v, want resolver prompt/status",
				envelope.Prompt,
				envelope.Status,
			)
		}

		var frontmatter Frontmatter
		if err := json.Unmarshal(snapshot.FrontmatterJSON, &frontmatter); err != nil {
			t.Fatalf("Unmarshal(FrontmatterJSON) error = %v", err)
		}
		if frontmatter.Version != 1 || !frontmatter.Enabled {
			t.Fatalf("frontmatter = %#v, want version 1 enabled policy", frontmatter)
		}
	})
}

func TestHeartbeatPersistenceValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate persistence rows and closed enum members", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshot := heartbeatSnapshotForPersistenceTest(createdAt)
		if err := snapshot.Validate(); err != nil {
			t.Fatalf("Snapshot.Validate() error = %v", err)
		}
		snapshot.ResolvedJSON = json.RawMessage(`{`)
		if !errors.Is(snapshot.Validate(), ErrInvalidSnapshot) {
			t.Fatal("Snapshot.Validate(malformed JSON) error does not wrap ErrInvalidSnapshot")
		}

		revision := heartbeatRevisionForPersistenceTest(createdAt)
		if err := revision.Validate(); err != nil {
			t.Fatalf("Revision.Validate() error = %v", err)
		}
		revision.Operation = RevisionOperation("replace")
		if !errors.Is(revision.Validate(), ErrInvalidRevision) {
			t.Fatal("Revision.Validate(invalid operation) error does not wrap ErrInvalidRevision")
		}

		health := heartbeatSessionHealthForPersistenceTest(createdAt)
		if err := health.Validate(); err != nil {
			t.Fatalf("SessionHealth.Validate() error = %v", err)
		}
		health.Health = SessionHealthStatus("sleeping")
		if !errors.Is(health.Validate(), ErrInvalidSessionHealth) {
			t.Fatal("SessionHealth.Validate(invalid health) error does not wrap ErrInvalidSessionHealth")
		}
		health = heartbeatSessionHealthForPersistenceTest(createdAt)
		health.EligibleForWake = false
		health.IneligibilityReason = string(SessionHealthReasonHung)
		if err := health.Validate(); err != nil {
			t.Fatalf("SessionHealth.Validate(valid ineligibility reason) error = %v", err)
		}
		health.IneligibilityReason = "task_lease_renewed"
		if !errors.Is(health.Validate(), ErrInvalidSessionHealth) {
			t.Fatal("SessionHealth.Validate(invalid ineligibility reason) error does not wrap ErrInvalidSessionHealth")
		}

		wakeState := heartbeatWakeStateForPersistenceTest(createdAt)
		if err := wakeState.Validate(); err != nil {
			t.Fatalf("WakeState.Validate() error = %v", err)
		}
		wakeState.LastReason = WakeReason("queued")
		if !errors.Is(wakeState.Validate(), ErrInvalidWakeState) {
			t.Fatal("WakeState.Validate(invalid reason) error does not wrap ErrInvalidWakeState")
		}

		wakeEvent := heartbeatWakeEventForPersistenceTest(createdAt)
		if err := wakeEvent.Validate(); err != nil {
			t.Fatalf("WakeEvent.Validate() error = %v", err)
		}
		wakeEvent.Source = WakeSource("worker")
		if !errors.Is(wakeEvent.Validate(), ErrInvalidWakeEvent) {
			t.Fatal("WakeEvent.Validate(invalid source) error does not wrap ErrInvalidWakeEvent")
		}

		if !ValidRevisionOperation(RevisionOperationWrite) ||
			!ValidActorKind(ActorKindSystem) ||
			!ValidSessionHealthState(SessionHealthStatePrompting) ||
			!ValidSessionHealthStatus(SessionHealthStale) ||
			!ValidSessionHealthIneligibilityReason(string(SessionHealthReasonPromptActive)) ||
			!ValidWakeSource(WakeSourceHarnessReentry) ||
			!ValidWakeResult(WakeResultRateLimited) ||
			!ValidWakeReason(WakeReasonSessionPromptRace) {
			t.Fatal("valid closed enum member rejected")
		}
		if ValidRevisionOperation("patch") ||
			ValidActorKind("bot") ||
			ValidSessionHealthState("busy") ||
			ValidSessionHealthStatus("paused") ||
			ValidSessionHealthIneligibilityReason("claim_token") ||
			ValidWakeSource("queue") ||
			ValidWakeResult("claimed") ||
			ValidWakeReason("claim_token") {
			t.Fatal("invalid closed enum member accepted")
		}
	})

	t.Run("Should validate list filters and rollback lookup inputs", func(t *testing.T) {
		t.Parallel()

		if err := (SnapshotListQuery{Limit: -1}).Validate(); !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("SnapshotListQuery.Validate() error = %v, want ErrInvalidSnapshot", err)
		}
		if err := (RevisionListQuery{Operation: RevisionOperation("rewrite")}).Validate(); !errors.Is(
			err,
			ErrInvalidRevision,
		) {
			t.Fatalf("RevisionListQuery.Validate() error = %v, want ErrInvalidRevision", err)
		}
		if err := (RollbackLookup{}).Validate(); !errors.Is(err, ErrInvalidRevision) {
			t.Fatalf("RollbackLookup.Validate() error = %v, want ErrInvalidRevision", err)
		}
		if err := (SessionHealthListQuery{State: SessionHealthState("active")}).Validate(); !errors.Is(
			err,
			ErrInvalidSessionHealth,
		) {
			t.Fatalf("SessionHealthListQuery.Validate() error = %v, want ErrInvalidSessionHealth", err)
		}
		if err := (WakeStateListQuery{Limit: -1}).Validate(); !errors.Is(err, ErrInvalidWakeState) {
			t.Fatalf("WakeStateListQuery.Validate() error = %v, want ErrInvalidWakeState", err)
		}
		if err := (WakeEventListQuery{Reason: WakeReason("queue")}).Validate(); !errors.Is(
			err,
			ErrInvalidWakeEvent,
		) {
			t.Fatalf("WakeEventListQuery.Validate() error = %v, want ErrInvalidWakeEvent", err)
		}
	})
}

func heartbeatSnapshotForPersistenceTest(createdAt time.Time) Snapshot {
	return Snapshot{
		ID:              "hb-snapshot",
		WorkspaceID:     "ws-heartbeat",
		AgentName:       "coder",
		SourcePath:      ".agh/agents/coder/HEARTBEAT.md",
		SchemaVersion:   1,
		Digest:          "sha256:heartbeat",
		ConfigDigest:    "sha256:config",
		Body:            "wake carefully",
		FrontmatterJSON: json.RawMessage(`{"version":1}`),
		ResolvedJSON:    json.RawMessage(`{"schema_version":1,"valid":true}`),
		DiagnosticsJSON: json.RawMessage(`[]`),
		CreatedAt:       createdAt,
	}
}

func heartbeatRevisionForPersistenceTest(createdAt time.Time) Revision {
	return Revision{
		ID:             "hb-revision",
		WorkspaceID:    "ws-heartbeat",
		AgentName:      "coder",
		SourcePath:     ".agh/agents/coder/HEARTBEAT.md",
		Operation:      RevisionOperationWrite,
		PreviousDigest: "sha256:previous",
		NewDigest:      "sha256:heartbeat",
		NewSnapshotID:  "hb-snapshot",
		Body:           "wake carefully",
		ActorKind:      ActorKindAgent,
		ActorID:        "coder",
		CreatedAt:      createdAt,
	}
}

func heartbeatSessionHealthForPersistenceTest(updatedAt time.Time) SessionHealth {
	return SessionHealth{
		SessionID:           "sess-heartbeat",
		WorkspaceID:         "ws-heartbeat",
		AgentName:           "coder",
		State:               SessionHealthStateIdle,
		Health:              SessionHealthHealthy,
		ActivePrompt:        false,
		Attachable:          true,
		EligibleForWake:     true,
		IneligibilityReason: "",
		LastActivityAt:      updatedAt.Add(-time.Minute),
		LastPresenceAt:      updatedAt.Add(-time.Minute),
		UpdatedAt:           updatedAt,
	}
}

func heartbeatWakeStateForPersistenceTest(updatedAt time.Time) WakeState {
	return WakeState{
		WorkspaceID:      "ws-heartbeat",
		AgentName:        "coder",
		SessionID:        "sess-heartbeat",
		PolicySnapshotID: "hb-snapshot",
		LastWakeAt:       updatedAt.Add(-time.Minute),
		NextAllowedAt:    updatedAt.Add(time.Hour),
		CoalescedCount:   1,
		LastResult:       WakeResultSent,
		LastReason:       WakeReasonSent,
		UpdatedAt:        updatedAt,
	}
}

func heartbeatWakeEventForPersistenceTest(createdAt time.Time) WakeEvent {
	return WakeEvent{
		ID:                "hb-event",
		WorkspaceID:       "ws-heartbeat",
		AgentName:         "coder",
		SessionID:         "sess-heartbeat",
		PolicySnapshotID:  "hb-snapshot",
		Source:            WakeSourceScheduler,
		Result:            WakeResultSent,
		Reason:            WakeReasonSent,
		SyntheticPromptID: "prompt-heartbeat",
		CreatedAt:         createdAt,
		ExpiresAt:         createdAt.Add(24 * time.Hour),
	}
}
