package task

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestClaimCriteriaValidationAndTokenHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	base := ClaimCriteria{
		Scope:                ScopeGlobal,
		ClaimerSessionID:     "sess-claim",
		RequiredCapabilities: []string{"golang"},
		LeaseDuration:        time.Minute,
		Now:                  now,
	}

	tests := []struct {
		name     string
		criteria ClaimCriteria
		wantErr  error
	}{
		{
			name:     "missing claimer session",
			criteria: ClaimCriteria{Scope: ScopeGlobal, LeaseDuration: time.Minute, Now: now},
			wantErr:  ErrValidation,
		},
		{
			name: "workspace scope requires workspace id",
			criteria: ClaimCriteria{
				Scope:            ScopeWorkspace,
				ClaimerSessionID: "sess-claim",
				LeaseDuration:    time.Minute,
				Now:              now,
			},
			wantErr: ErrInvalidScopeBinding,
		},
		{
			name: "capability ids reject whitespace",
			criteria: ClaimCriteria{
				Scope:                ScopeGlobal,
				ClaimerSessionID:     "sess-claim",
				RequiredCapabilities: []string{"golang sqlite"},
				LeaseDuration:        time.Minute,
				Now:                  now,
			},
			wantErr: ErrValidation,
		},
		{
			name: "lease duration is bounded",
			criteria: ClaimCriteria{
				Scope:            ScopeGlobal,
				ClaimerSessionID: "sess-claim",
				LeaseDuration:    MaxRunLeaseDuration + time.Nanosecond,
				Now:              now,
			},
			wantErr: ErrValidation,
		},
		{
			name:     "valid criteria defaults claimed by",
			criteria: base,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.criteria.Normalize(now)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Normalize() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got.ClaimedBy == nil || got.ClaimedBy.Kind != ActorKindAgentSession ||
				got.ClaimedBy.Ref != "sess-claim" {
				t.Fatalf("ClaimedBy = %#v, want agent session claimer", got.ClaimedBy)
			}
		})
	}

	rawToken := "agh_claim_unit_token"
	hash, err := ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash() error = %v", err)
	}
	if !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("ClaimTokenHash() = %q, want sha256 prefix", hash)
	}
	if strings.Contains(hash, rawToken) {
		t.Fatalf("ClaimTokenHash() = %q contains raw token", hash)
	}
	if !VerifyClaimToken(rawToken, hash) {
		t.Fatal("VerifyClaimToken(raw, hash) = false, want true")
	}
	if !VerifyClaimToken(" "+rawToken+" ", strings.TrimPrefix(hash, "sha256:")) {
		t.Fatal("VerifyClaimToken() should accept canonical hash without prefix and trim raw token")
	}
	if VerifyClaimToken("wrong-token", hash) {
		t.Fatal("VerifyClaimToken(wrong, hash) = true, want false")
	}
	if _, err := ClaimTokenHash(" "); !errors.Is(err, ErrValidation) {
		t.Fatalf("ClaimTokenHash(empty) error = %v, want %v", err, ErrValidation)
	}

	redacted := RedactClaimTokens("bad token agh_claim_secret-123 and agh_claim_other_456")
	if strings.Contains(redacted, "agh_claim_secret-123") ||
		strings.Contains(redacted, "agh_claim_other_456") ||
		strings.Count(redacted, "agh_claim_[REDACTED]") != 2 {
		t.Fatalf("RedactClaimTokens() = %q, want both raw claim tokens redacted", redacted)
	}
}

func TestRedactClaimTokenJSON(t *testing.T) {
	t.Parallel()

	t.Run("Should remove token fields and redact token values recursively", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(
			`{"keep":"safe","claim_token":"agh_claim_top-secret","nested":{"note":"uses agh_claim_nested-secret"},` +
				`"items":[{"proof":"agh_claim_array-secret"}]}`,
		)
		redacted := RedactClaimTokenJSON(raw)
		for _, secret := range []string{
			"agh_claim_top-secret",
			"agh_claim_nested-secret",
			"agh_claim_array-secret",
		} {
			if strings.Contains(string(redacted), secret) {
				t.Fatalf("RedactClaimTokenJSON() = %s, want raw token %q removed", redacted, secret)
			}
		}
		if strings.Contains(string(redacted), `"claim_token"`) {
			t.Fatalf("RedactClaimTokenJSON() = %s, want no raw claim token material", redacted)
		}
		if !strings.Contains(string(redacted), `"keep":"safe"`) {
			t.Fatalf("RedactClaimTokenJSON() = %s, want safe metadata preserved", redacted)
		}
		raw[2] = 'X'
		if !strings.Contains(string(redacted), `"keep":"safe"`) {
			t.Fatalf("RedactClaimTokenJSON() aliased input: %s", redacted)
		}
	})

	t.Run("Should redact tokens even when persisted JSON is malformed", func(t *testing.T) {
		t.Parallel()

		redacted := RedactClaimTokenJSON(json.RawMessage(`{"note":"agh_claim_malformed-secret"`))
		if strings.Contains(string(redacted), "agh_claim_malformed-secret") {
			t.Fatalf("RedactClaimTokenJSON(malformed) = %s, want raw token removed", redacted)
		}
	})
}

func TestClaimResultSanitizesRawClaimTokenMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should redact raw claim token fields", func(t *testing.T) {
		t.Parallel()

		result := ClaimResult{
			Task: &Task{
				Metadata: json.RawMessage(
					`{"claim_token":"task-raw","nested":{"claim_token":"nested-raw","keep":true}}`,
				),
			},
			Run: Run{
				Metadata: json.RawMessage(`{"claim_token":"run-raw","items":[{"claim_token":"item-raw","ok":true}]}`),
				Result:   json.RawMessage(`{"claim_token":"result-raw","ok":true}`),
			},
			CoordinationChannel: &CoordinationChannelMetadata{
				ID:                  " coord.core ",
				AllowedMessageKinds: []string{"status", "status", " reply "},
			},
		}

		claimResultWithoutRawTokenInMetadata(&result)
		for label, raw := range map[string]json.RawMessage{
			"task metadata": result.Task.Metadata,
			"run metadata":  result.Run.Metadata,
			"run result":    result.Run.Result,
		} {
			if strings.Contains(strings.ToLower(string(raw)), "claim_token") {
				t.Fatalf("%s still contains raw claim_token field: %s", label, raw)
			}
		}
		if result.CoordinationChannel == nil {
			t.Fatal("CoordinationChannel = nil, want sanitized metadata")
		}
		if got, want := result.CoordinationChannel.ID, "coord.core"; got != want {
			t.Fatalf("CoordinationChannel.ID = %q, want %q", got, want)
		}
		if got, want := result.CoordinationChannel.DisplayName, "coord.core"; got != want {
			t.Fatalf("CoordinationChannel.DisplayName = %q, want %q", got, want)
		}
		if got, want := result.CoordinationChannel.AllowedMessageKinds, []string{
			"status",
			"reply",
		}; len(
			got,
		) != len(
			want,
		) ||
			got[0] != want[0] ||
			got[1] != want[1] {
			t.Fatalf("AllowedMessageKinds = %#v, want %#v", got, want)
		}
	})

	t.Run("Should drop over-depth metadata before leaking raw claim tokens", func(t *testing.T) {
		t.Parallel()

		nested := any(map[string]any{"claim_token": "too-deep"})
		for range maxClaimTokenMetadataRedactionDepth + 2 {
			nested = map[string]any{"nested": nested}
		}
		raw, err := json.Marshal(nested)
		if err != nil {
			t.Fatalf("json.Marshal(over-depth metadata) error = %v", err)
		}
		result := ClaimResult{Task: &Task{Metadata: raw}}

		claimResultWithoutRawTokenInMetadata(&result)
		if result.Task.Metadata != nil {
			t.Fatalf("Task.Metadata = %s, want nil over-depth redaction", result.Task.Metadata)
		}
	})
}

func TestManagerLookupActiveRunForSessionRejectsUnsafeLeases(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		sessionID string
		runID     string
		seed      func(t *testing.T, store *inMemoryManagerStore)
		want      AutonomyReasonCode
		cause     error
		rawToken  string
	}{
		{
			name:      "Should reject missing session identity",
			sessionID: " ",
			runID:     "run-1",
			want:      AutonomySessionRequired,
			cause:     ErrPermissionDenied,
		},
		{
			name:      "Should reject missing active lease",
			sessionID: "sess-a",
			runID:     "run-1",
			want:      AutonomyNoActiveLease,
			cause:     ErrInvalidClaimToken,
		},
		{
			name:      "Should reject foreign run while session holds another lease",
			sessionID: "sess-a",
			runID:     "run-2",
			rawToken:  "agh_claim_FOREIGN123",
			seed: func(t *testing.T, store *inMemoryManagerStore) {
				t.Helper()
				seedAutonomyLeaseRun(t, store, "run-1", "sess-a", "agh_claim_FOREIGN123", now.Add(time.Minute))
			},
			want:  AutonomyForeignRun,
			cause: ErrPermissionDenied,
		},
		{
			name:      "Should reject stale lease",
			sessionID: "sess-a",
			runID:     "run-1",
			rawToken:  "agh_claim_EXPIRED123",
			seed: func(t *testing.T, store *inMemoryManagerStore) {
				t.Helper()
				seedAutonomyLeaseRun(t, store, "run-1", "sess-a", "agh_claim_EXPIRED123", now.Add(-time.Minute))
			},
			want:  AutonomyLeaseExpired,
			cause: ErrLeaseExpired,
		},
		{
			name:      "Should reject double active lease",
			sessionID: "sess-a",
			runID:     "run-1",
			rawToken:  "agh_claim_DOUBLE_A123",
			seed: func(t *testing.T, store *inMemoryManagerStore) {
				t.Helper()
				seedAutonomyLeaseRun(t, store, "run-1", "sess-a", "agh_claim_DOUBLE_A123", now.Add(time.Minute))
				seedAutonomyLeaseRun(t, store, "run-2", "sess-a", "agh_claim_DOUBLE_B123", now.Add(time.Minute))
			},
			want:  AutonomyLeaseAlreadyHeld,
			cause: ErrActiveRunLease,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			if tt.seed != nil {
				tt.seed(t, store)
			}
			manager := newTaskManagerForTestWithOptions(t, store, WithManagerNow(func() time.Time {
				return now
			}))
			_, err := manager.LookupActiveRunForSession(context.Background(), tt.sessionID, tt.runID)
			if !errors.Is(err, tt.cause) {
				t.Fatalf("LookupActiveRunForSession() error = %v, want cause %v", err, tt.cause)
			}
			reason, ok := AutonomyReasonOf(err)
			if !ok || reason != tt.want {
				t.Fatalf("AutonomyReasonOf() = %q/%v, want %q", reason, ok, tt.want)
			}
			if tt.rawToken != "" && strings.Contains(err.Error(), tt.rawToken) {
				t.Fatalf("LookupActiveRunForSession() leaked raw token in error: %v", err)
			}
		})
	}
}

func TestManagerLookupActiveRunForSessionReturnsInternalHandle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rawToken := "agh_claim_ACTIVE123"
	store := newInMemoryManagerStore()
	seedAutonomyLeaseRun(t, store, "run-1", "sess-a", rawToken, now.Add(time.Minute))
	manager := newTaskManagerForTestWithOptions(t, store, WithManagerNow(func() time.Time {
		return now
	}))

	handle, err := manager.LookupActiveRunForSession(context.Background(), " sess-a ", " run-1 ")
	if err != nil {
		t.Fatalf("LookupActiveRunForSession() error = %v", err)
	}
	if handle.RunID != "run-1" ||
		handle.SessionID != "sess-a" ||
		handle.ClaimToken != rawToken ||
		!VerifyClaimToken(rawToken, handle.ClaimTokenHash) {
		t.Fatalf("LookupActiveRunForSession() = %#v, want active internal lease handle", handle)
	}
}

func seedAutonomyLeaseRun(
	t *testing.T,
	store *inMemoryManagerStore,
	runID string,
	sessionID string,
	rawToken string,
	leaseUntil time.Time,
) {
	t.Helper()

	tokenHash, err := ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash() error = %v", err)
	}
	normalizedRunID := strings.TrimSpace(runID)
	normalizedSessionID := strings.TrimSpace(sessionID)
	store.runs[normalizedRunID] = Run{
		ID:             normalizedRunID,
		TaskID:         "task-" + strings.TrimPrefix(normalizedRunID, "run-"),
		Status:         TaskRunStatusClaimed,
		SessionID:      normalizedSessionID,
		ClaimedBy:      &ActorIdentity{Kind: ActorKindAgentSession, Ref: normalizedSessionID},
		ClaimTokenHash: tokenHash,
		ClaimedAt:      leaseUntil.Add(-time.Minute),
		HeartbeatAt:    leaseUntil.Add(-time.Minute),
		LeaseUntil:     leaseUntil,
		QueuedAt:       leaseUntil.Add(-2 * time.Minute),
	}
	store.claimTokens[normalizedRunID] = rawToken
}

func TestManagerClaimNextRunAndLeaseFencing(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	operator := validActorContext()
	agent := agentActorContextForTest("sess-agent", "ws-agent")

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Claim lease task",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	firstRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator)
	if err != nil {
		t.Fatalf("EnqueueRun(first) error = %v", err)
	}
	secondTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Second claim lease task",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask(second) error = %v", err)
	}
	if _, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: secondTask.ID}, operator); err != nil {
		t.Fatalf("EnqueueRun(second) error = %v", err)
	}

	claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		RunID:            firstRun.ID,
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-agent",
		LeaseDuration:    time.Minute,
		Now:              claimNow,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	if got, want := claim.Run.ID, firstRun.ID; got != want {
		t.Fatalf("ClaimNextRun() run id = %q, want %q", got, want)
	}
	if claim.ClaimToken == "" {
		t.Fatal("ClaimToken is empty")
	}
	if !VerifyClaimToken(claim.ClaimToken, claim.Run.ClaimTokenHash) {
		t.Fatal("ClaimToken does not verify against persisted hash")
	}

	if _, err := manager.CompleteRun(context.Background(), firstRun.ID, RunResult{
		Value: json.RawMessage(`{"legacy":true}`),
	}, agent); !errors.Is(err, ErrInvalidClaimToken) {
		t.Fatalf("CompleteRun(unfenced active lease) error = %v, want %v", err, ErrInvalidClaimToken)
	}
	if _, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
		RunID:         firstRun.ID,
		ClaimToken:    "wrong-token",
		LeaseDuration: time.Minute,
		Now:           claimNow.Add(10 * time.Second),
	}, agent); !errors.Is(err, ErrInvalidClaimToken) {
		t.Fatalf("HeartbeatRunLease(stale token) error = %v, want %v", err, ErrInvalidClaimToken)
	}
	if _, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
		RunID:         firstRun.ID,
		ClaimToken:    claim.ClaimToken,
		LeaseDuration: time.Minute,
		Now:           claim.LeaseUntil,
	}, agent); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("HeartbeatRunLease(expired lease) error = %v, want %v", err, ErrLeaseExpired)
	}
	heartbeat, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
		RunID:         firstRun.ID,
		ClaimToken:    claim.ClaimToken,
		LeaseDuration: 2 * time.Minute,
		Now:           claimNow.Add(30 * time.Second),
	}, agent)
	if err != nil {
		t.Fatalf("HeartbeatRunLease(current token) error = %v", err)
	}
	if got, want := heartbeat.LeaseUntil, claimNow.Add(150*time.Second); !got.Equal(want) {
		t.Fatalf("HeartbeatRunLease().LeaseUntil = %v, want %v", got, want)
	}

	if _, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-agent",
		LeaseDuration:    time.Minute,
		Now:              claimNow.Add(40 * time.Second),
	}, agent); !errors.Is(err, ErrActiveRunLease) {
		t.Fatalf("ClaimNextRun(second active lease) error = %v, want %v", err, ErrActiveRunLease)
	}

	completed, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
		RunID:      firstRun.ID,
		ClaimToken: claim.ClaimToken,
		Result:     RunResult{Value: json.RawMessage(`{"ok":true}`)},
		Now:        claimNow.Add(45 * time.Second),
	}, agent)
	if err != nil {
		t.Fatalf("CompleteRunLease() error = %v", err)
	}
	if got, want := completed.Status, TaskRunStatusCompleted; got != want {
		t.Fatalf("completed.Status = %q, want %q", got, want)
	}
	if completed.LeaseUntil.IsZero() == false || completed.HeartbeatAt.IsZero() == false {
		t.Fatalf(
			"completed lease fields = lease_until %v heartbeat_at %v, want zero",
			completed.LeaseUntil,
			completed.HeartbeatAt,
		)
	}
	if completed.ClaimTokenHash == "" {
		t.Fatal("completed.ClaimTokenHash = empty, want retained fencing history")
	}

	secondClaim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-agent",
		LeaseDuration:    time.Minute,
		Now:              claimNow.Add(time.Minute),
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun(after completion) error = %v", err)
	}
	released, err := manager.ReleaseRunLease(context.Background(), LeaseRelease{
		RunID:      secondClaim.Run.ID,
		ClaimToken: secondClaim.ClaimToken,
		Reason:     "handoff",
		Now:        claimNow.Add(70 * time.Second),
	}, agent)
	if err != nil {
		t.Fatalf("ReleaseRunLease() error = %v", err)
	}
	if got, want := released.Status, TaskRunStatusQueued; got != want {
		t.Fatalf("released.Status = %q, want %q", got, want)
	}
	if released.ClaimTokenHash != "" || released.SessionID != "" || released.ClaimedBy != nil {
		t.Fatalf("released ownership fields = hash %q session %q claimed_by %#v, want cleared",
			released.ClaimTokenHash,
			released.SessionID,
			released.ClaimedBy,
		)
	}
	if _, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
		RunID:         released.ID,
		ClaimToken:    secondClaim.ClaimToken,
		LeaseDuration: time.Minute,
		Now:           claimNow.Add(80 * time.Second),
	}, agent); !errors.Is(err, ErrInvalidClaimToken) {
		t.Fatalf("HeartbeatRunLease(after release) error = %v, want %v", err, ErrInvalidClaimToken)
	}

	failClaim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-agent",
		LeaseDuration:    time.Minute,
		Now:              claimNow.Add(90 * time.Second),
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun(for failure) error = %v", err)
	}
	failed, err := manager.FailRunLease(context.Background(), LeaseFailure{
		RunID:      failClaim.Run.ID,
		ClaimToken: failClaim.ClaimToken,
		Failure:    RunFailure{Error: "worker failed"},
		Now:        claimNow.Add(100 * time.Second),
	}, agent)
	if err != nil {
		t.Fatalf("FailRunLease() error = %v", err)
	}
	if got, want := failed.Status, TaskRunStatusFailed; got != want {
		t.Fatalf("failed.Status = %q, want %q", got, want)
	}
	if got, want := failed.Error, "worker failed"; got != want {
		t.Fatalf("failed.Error = %q, want %q", got, want)
	}
}

func TestManagerCompleteRunLeaseHallucinationGate(t *testing.T) {
	t.Parallel()

	claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	t.Run("Should reject phantom created task ids and preserve the active lease", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		taskRecord, claim, agent := setupCompletionLeaseForTest(
			t,
			manager,
			ScopeGlobal,
			"",
			"sess-phantom",
			claimNow,
		)
		markClaimedRunRunningForTest(t, store, claim.Run.ID)
		before := cloneTaskRun(store.runs[claim.Run.ID])

		_, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
			RunID:          claim.Run.ID,
			ClaimToken:     claim.ClaimToken,
			Result:         RunResult{Value: json.RawMessage(`{"summary":"claimed phantom child"}`)},
			CreatedTaskIDs: []string{"task-phantom-0001"},
			Now:            claimNow.Add(30 * time.Second),
		}, agent)
		if !errors.Is(err, ErrHallucinatedTaskRefs) {
			t.Fatalf("CompleteRunLease() error = %v, want %v", err, ErrHallucinatedTaskRefs)
		}

		var typed *HallucinatedTaskRefsError
		if !errors.As(err, &typed) {
			t.Fatalf("CompleteRunLease() error type = %T, want *HallucinatedTaskRefsError", err)
		}
		if got, want := typed.InvalidTaskIDs, []string{"task-phantom-0001"}; len(got) != len(want) ||
			got[0] != want[0] {
			t.Fatalf("InvalidTaskIDs = %#v, want %#v", got, want)
		}

		after := store.runs[claim.Run.ID]
		assertRunLeaseUnchangedAfterBlockedCompletion(t, after, before)

		events, eventErr := store.ListTaskEvents(context.Background(), EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     claim.Run.ID,
			EventType: taskEventCompletionHallucinationBlocked,
		})
		if eventErr != nil {
			t.Fatalf("ListTaskEvents(hallucination_blocked) error = %v", eventErr)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("hallucination_blocked event count = %d, want %d", got, want)
		}
		payload := decodeCompletionHallucinationBlockedPayload(t, events[0].Payload)
		if got, want := payload.InvalidTaskIDs, []string{"task-phantom-0001"}; len(got) != len(want) ||
			got[0] != want[0] {
			t.Fatalf("blocked payload InvalidTaskIDs = %#v, want %#v", got, want)
		}
		if strings.Contains(string(events[0].Payload), claim.ClaimToken) {
			t.Fatalf("hallucination_blocked payload leaked raw claim token: %s", events[0].Payload)
		}
		if countTaskEventsByType(store.events, taskWatchEventRunCompleted) != 0 {
			t.Fatal("task.run.completed event recorded for rejected completion")
		}
	})

	t.Run("Should reject created task ids from another session or workspace", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			workspaceID string
			createClaim func(t *testing.T, manager *Service, agent ActorContext) string
		}{
			{
				name:        "Should reject a task created by another session",
				workspaceID: "ws-same",
				createClaim: func(t *testing.T, manager *Service, _ ActorContext) string {
					t.Helper()
					otherAgent := agentActorContextForTest("sess-other", "ws-same")
					child, err := manager.CreateTask(context.Background(), CreateTask{
						Scope:       ScopeWorkspace,
						WorkspaceID: "ws-same",
						Title:       "Other session child",
					}, otherAgent)
					if err != nil {
						t.Fatalf("CreateTask(other session child) error = %v", err)
					}
					return child.ID
				},
			},
			{
				name:        "Should reject a task created in another workspace",
				workspaceID: "ws-home",
				createClaim: func(t *testing.T, manager *Service, agent ActorContext) string {
					t.Helper()
					foreignWorkspaceAgent := agentActorContextForTest(agent.Actor.Ref, "ws-away")
					child, err := manager.CreateTask(context.Background(), CreateTask{
						Scope:       ScopeWorkspace,
						WorkspaceID: "ws-away",
						Title:       "Other workspace child",
					}, foreignWorkspaceAgent)
					if err != nil {
						t.Fatalf("CreateTask(other workspace child) error = %v", err)
					}
					return child.ID
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				store := newInMemoryManagerStore()
				manager := newTaskManagerForTest(t, store)
				_, claim, agent := setupCompletionLeaseForTest(
					t,
					manager,
					ScopeWorkspace,
					tt.workspaceID,
					"sess-owner",
					claimNow,
				)
				claimedTaskID := tt.createClaim(t, manager, agent)
				before := cloneTaskRun(store.runs[claim.Run.ID])

				_, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
					RunID:          claim.Run.ID,
					ClaimToken:     claim.ClaimToken,
					Result:         RunResult{Value: json.RawMessage(`{"summary":"claimed invalid child"}`)},
					CreatedTaskIDs: []string{claimedTaskID},
					Now:            claimNow.Add(30 * time.Second),
				}, agent)
				if !errors.Is(err, ErrHallucinatedTaskRefs) {
					t.Fatalf("CompleteRunLease() error = %v, want %v", err, ErrHallucinatedTaskRefs)
				}
				assertRunLeaseUnchangedAfterBlockedCompletion(t, store.runs[claim.Run.ID], before)
				gotBlockedEvents := countTaskEventsByType(store.events, taskEventCompletionHallucinationBlocked)
				if got, want := gotBlockedEvents, 1; got != want {
					t.Fatalf("hallucination_blocked event count = %d, want %d", got, want)
				}
			})
		}
	})

	t.Run("Should accept completion when every created task id belongs to the completing session", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		_, claim, agent := setupCompletionLeaseForTest(t, manager, ScopeWorkspace, "ws-valid", "sess-valid", claimNow)
		child, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-valid",
			Title:       "Valid child",
		}, agent)
		if err != nil {
			t.Fatalf("CreateTask(valid child) error = %v", err)
		}

		completed, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
			RunID:          claim.Run.ID,
			ClaimToken:     claim.ClaimToken,
			Result:         RunResult{Value: json.RawMessage(`{"summary":"valid child complete"}`)},
			CreatedTaskIDs: []string{child.ID},
			Now:            claimNow.Add(30 * time.Second),
		}, agent)
		if err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		if got, want := completed.Status, TaskRunStatusCompleted; got != want {
			t.Fatalf("completed.Status = %q, want %q", got, want)
		}
		if got := countTaskEventsByType(store.events, taskEventCompletionHallucinationBlocked); got != 0 {
			t.Fatalf("hallucination_blocked event count = %d, want 0", got)
		}
		if got := countTaskEventsByType(store.events, taskEventCompletionHallucinationSuspected); got != 0 {
			t.Fatalf("hallucination_suspected event count = %d, want 0", got)
		}
	})

	t.Run("Should warn only when result prose references an absent task id", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		_, claim, agent := setupCompletionLeaseForTest(
			t,
			manager,
			ScopeGlobal,
			"",
			"sess-advisory",
			claimNow,
		)

		completed, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result: RunResult{
				Value: json.RawMessage(`{"summary":"I delegated follow-up to task-phantom-7777"}`),
			},
			Now: claimNow.Add(30 * time.Second),
		}, agent)
		if err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		if got, want := completed.Status, TaskRunStatusCompleted; got != want {
			t.Fatalf("completed.Status = %q, want %q", got, want)
		}

		events, eventErr := store.ListTaskEvents(context.Background(), EventQuery{
			TaskID:    completed.TaskID,
			RunID:     completed.ID,
			EventType: taskEventCompletionHallucinationSuspected,
		})
		if eventErr != nil {
			t.Fatalf("ListTaskEvents(hallucination_suspected) error = %v", eventErr)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("hallucination_suspected event count = %d, want %d", got, want)
		}
		payload := decodeCompletionHallucinationSuspectedPayload(t, events[0].Payload)
		if got, want := payload.SuspectedTaskIDs, []string{"task-phantom-7777"}; len(got) != len(want) ||
			got[0] != want[0] {
			t.Fatalf("suspected payload task IDs = %#v, want %#v", got, want)
		}
		if got := countTaskEventsByType(store.events, taskWatchEventRunCompleted); got != 1 {
			t.Fatalf("task.run.completed event count = %d, want 1", got)
		}
	})
}

func setupCompletionLeaseForTest(
	t *testing.T,
	manager *Service,
	scope Scope,
	workspaceID string,
	sessionID string,
	claimNow time.Time,
) (Task, *ClaimResult, ActorContext) {
	t.Helper()

	operator := validActorContext()
	agentWorkspaceID := workspaceID
	if agentWorkspaceID == "" {
		agentWorkspaceID = "ws-agent-home"
	}
	agent := agentActorContextForTest(sessionID, agentWorkspaceID)
	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       scope,
		WorkspaceID: workspaceID,
		Title:       "Completion gate task",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator); err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            scope,
		WorkspaceID:      workspaceID,
		ClaimerSessionID: sessionID,
		LeaseDuration:    2 * time.Minute,
		Now:              claimNow,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	return *taskRecord, claim, agent
}

func agentActorContextForTest(sessionID string, workspaceID string) ActorContext {
	actor := validActorContext()
	actor.Actor = ActorIdentity{Kind: ActorKindAgentSession, Ref: sessionID}
	actor.Origin = Origin{Kind: OriginKindAgentSession, Ref: "codex"}
	actor.Scope = CallerScope{SessionID: sessionID, WorkspaceID: workspaceID}
	return actor
}

func markClaimedRunRunningForTest(t *testing.T, store *inMemoryManagerStore, runID string) {
	t.Helper()

	run, ok := store.runs[strings.TrimSpace(runID)]
	if !ok {
		t.Fatalf("run %q not found", runID)
	}
	run.Status = TaskRunStatusRunning
	run.StartedAt = run.ClaimedAt.Add(time.Second)
	store.runs[run.ID] = run
}

func assertRunLeaseUnchangedAfterBlockedCompletion(t *testing.T, got Run, want Run) {
	t.Helper()

	if got.Status != want.Status ||
		got.Attempt != want.Attempt ||
		got.SessionID != want.SessionID ||
		got.ClaimTokenHash != want.ClaimTokenHash ||
		!got.LeaseUntil.Equal(want.LeaseUntil) ||
		!got.HeartbeatAt.Equal(want.HeartbeatAt) ||
		!got.EndedAt.Equal(want.EndedAt) ||
		string(got.Result) != string(want.Result) {
		t.Fatalf("run after rejection = %#v, want lease/state unchanged from %#v", got, want)
	}
}

func decodeCompletionHallucinationBlockedPayload(
	t *testing.T,
	raw json.RawMessage,
) completionHallucinationBlockedPayload {
	t.Helper()

	var payload completionHallucinationBlockedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(hallucination_blocked payload) error = %v", err)
	}
	return payload
}

func decodeCompletionHallucinationSuspectedPayload(
	t *testing.T,
	raw json.RawMessage,
) completionHallucinationSuspectedPayload {
	t.Helper()

	var payload completionHallucinationSuspectedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(hallucination_suspected payload) error = %v", err)
	}
	return payload
}

func TestManagerBlockTaskAndReleaseRunParksActiveLease(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	operator := validActorContext()
	agent := agentActorContextForTest("sess-blocker", "ws-blocker")

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Block leased task",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-blocker",
		LeaseDuration:    2 * time.Minute,
		Now:              claimNow,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	before := cloneTaskRun(store.runs[run.ID])

	block, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID:     taskRecord.ID,
		Kind:       BlockKindNeedsInput,
		Reason:     "creator clarification required",
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
	}, agent)
	if err != nil {
		t.Fatalf("BlockTask(active run) error = %v", err)
	}
	if block.ID == "" {
		t.Fatal("BlockTask().ID is empty")
	}
	recurrence := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)]
	if got, want := recurrence.Count, 0; got != want {
		t.Fatalf("recurrence.Count = %d, want %d", got, want)
	}

	persisted := store.runs[run.ID]
	if got, want := persisted.Status, TaskRunStatusQueued; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
	if got, want := persisted.Attempt, before.Attempt; got != want {
		t.Fatalf("persisted.Attempt = %d, want unchanged %d", got, want)
	}
	if persisted.SessionID != "" || persisted.ClaimTokenHash != "" || persisted.ClaimedBy != nil {
		t.Fatalf("persisted lease fields = session %q hash %q claimedBy %#v, want cleared",
			persisted.SessionID,
			persisted.ClaimTokenHash,
			persisted.ClaimedBy,
		)
	}
	storedTask := store.tasks[taskRecord.ID]
	if got, want := storedTask.Status, TaskStatusBlocked; got != want {
		t.Fatalf("storedTask.Status = %q, want %q", got, want)
	}

	events, err := store.ListTaskEvents(context.Background(), EventQuery{
		TaskID:    taskRecord.ID,
		RunID:     run.ID,
		EventType: taskEventRunReleased,
	})
	if err != nil {
		t.Fatalf("ListTaskEvents(released) error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("release event count = %d, want %d", got, want)
	}
	if !strings.Contains(string(events[0].Payload), `"reason":"blocked"`) {
		t.Fatalf("release event payload = %s, want blocked reason", string(events[0].Payload))
	}
}

func TestManagerBlockTaskAndReleaseRunRejectsClaimTokenMismatch(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	operator := validActorContext()
	agent := agentActorContextForTest("sess-mismatch", "ws-mismatch")

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Block token mismatch",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-mismatch",
		LeaseDuration:    time.Minute,
		Now:              claimNow,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	before := cloneTaskRun(store.runs[run.ID])

	_, err = manager.BlockTask(context.Background(), BlockRequest{
		TaskID:     taskRecord.ID,
		Kind:       BlockKindCapability,
		Reason:     "missing deploy capability",
		RunID:      claim.Run.ID,
		ClaimToken: "wrong-token",
	}, agent)
	if !errors.Is(err, ErrInvalidClaimToken) {
		t.Fatalf("BlockTask(wrong token) error = %v, want %v", err, ErrInvalidClaimToken)
	}
	after := store.runs[run.ID]
	if after.Status != before.Status ||
		after.SessionID != before.SessionID ||
		after.ClaimTokenHash != before.ClaimTokenHash ||
		!after.LeaseUntil.Equal(before.LeaseUntil) {
		t.Fatalf("run after rejected block = %#v, want unchanged %#v", after, before)
	}
	blocks, err := manager.ListTaskBlocks(context.Background(), taskRecord.ID, true, agent)
	if err != nil {
		t.Fatalf("ListTaskBlocks() error = %v", err)
	}
	if len(blocks) != 0 {
		t.Fatalf("len(blocks) = %d, want 0 after rejected block", len(blocks))
	}
}

func TestManagerBlockTaskAndReleaseRunSerializesDuplicateActiveCallers(t *testing.T) {
	t.Parallel()

	baseStore := newInMemoryManagerStore()
	store := &lockedLeaseTestStore{inMemoryManagerStore: baseStore}
	var idMu sync.Mutex
	nextID := 0
	manager := newTaskManagerForTestWithOptions(t, store, WithIDGenerator(func(prefix string) string {
		idMu.Lock()
		defer idMu.Unlock()
		nextID++
		return prefix + "-race-" + strconv.Itoa(nextID)
	}))
	operator := validActorContext()
	agent := agentActorContextForTest("sess-race", "ws-race")

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Race block leased task",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-race",
		LeaseDuration:    2 * time.Minute,
		Now:              claimNow,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	blocks := make([]TaskBlock, 2)
	for idx := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			blocks[index], errs[index] = manager.BlockTask(context.Background(), BlockRequest{
				TaskID:     taskRecord.ID,
				Kind:       BlockKindNeedsInput,
				Reason:     "duplicate caller " + strconv.Itoa(index),
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
			}, agent)
		}(idx)
	}
	wg.Wait()

	successes := 0
	for idx, err := range errs {
		if err == nil {
			successes++
			if blocks[idx].ID == "" {
				t.Fatalf("blocks[%d].ID is empty for successful block", idx)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidClaimToken) {
			t.Fatalf("errs[%d] = %v, want nil or %v", idx, err, ErrInvalidClaimToken)
		}
	}
	if successes != 1 {
		t.Fatalf("successful BlockTask calls = %d, want 1 (errs=%#v blocks=%#v)", successes, errs, blocks)
	}

	openBlocks, err := manager.ListTaskBlocks(context.Background(), taskRecord.ID, false, operator)
	if err != nil {
		t.Fatalf("ListTaskBlocks(open) error = %v", err)
	}
	if len(openBlocks) != 1 {
		t.Fatalf("len(openBlocks) = %d, want 1: %#v", len(openBlocks), openBlocks)
	}
	persisted, err := store.GetTaskRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if persisted.Status != TaskRunStatusQueued || persisted.ClaimTokenHash != "" || persisted.SessionID != "" {
		t.Fatalf("persisted run after race = %#v, want one queued unleased run", persisted)
	}
	events, err := store.ListTaskEvents(context.Background(), EventQuery{
		TaskID:    taskRecord.ID,
		RunID:     run.ID,
		EventType: taskEventRunReleased,
	})
	if err != nil {
		t.Fatalf("ListTaskEvents(released) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("release event count = %d, want 1", len(events))
	}
}

func TestManagerBlockClearAutoEnqueuesReadyOptedInTaskIdempotently(t *testing.T) {
	t.Parallel()

	t.Run("Should auto enqueue ready opted in task idempotently after block clear", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:              ScopeWorkspace,
			WorkspaceID:        "ws-auto-block-clear",
			Title:              "Auto enqueue on block clear",
			AutoEnqueueOnReady: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		block, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID: taskRecord.ID,
			Kind:   BlockKindCapability,
			Reason: "capability unavailable",
		}, actor)
		if err != nil {
			t.Fatalf("BlockTask() error = %v", err)
		}
		if _, err := manager.ClearTaskBlock(
			context.Background(),
			taskRecord.ID,
			block.ID,
			"capability restored",
			actor,
		); err != nil {
			t.Fatalf("ClearTaskBlock() error = %v", err)
		}
		if got, want := len(store.runs), 1; got != want {
			t.Fatalf("runs after block-clear auto enqueue = %d, want %d", got, want)
		}
		if got, want := countTaskEventsByType(store.events, taskEventAutoEnqueueTriggered), 1; got != want {
			t.Fatalf("auto enqueue triggered events = %d, want %d", got, want)
		}

		manager.autoEnqueueReadyTaskDetached(context.Background(), taskRecord.ID, autoEnqueueTrigger{
			Kind: autoEnqueueTriggerBlockClear,
			Ref:  block.ID,
		}, actor)
		if got, want := len(store.runs), 1; got != want {
			t.Fatalf("runs after repeated block-clear trigger = %d, want %d", got, want)
		}
	})
}

func TestManagerApprovalGrantedAutoEnqueuesReadyOptedInTask(t *testing.T) {
	t.Parallel()

	t.Run("Should auto enqueue ready opted in task on approval and replay idempotently", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:              ScopeWorkspace,
			WorkspaceID:        "ws-auto-approval",
			Title:              "Auto enqueue on approval",
			ApprovalPolicy:     ApprovalPolicyManual,
			AutoEnqueueOnReady: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if got, want := taskRecord.Status, TaskStatusBlocked; got != want {
			t.Fatalf("created Status = %q, want %q", got, want)
		}

		execution, err := manager.ApproveTask(context.Background(), taskRecord.ID, ExecutionRequest{}, actor)
		if err != nil {
			t.Fatalf("ApproveTask() error = %v", err)
		}
		wantKey := (autoEnqueueTrigger{
			Kind: autoEnqueueTriggerApprovalGranted,
			Ref:  taskRecord.ID,
		}).idempotencyKey(taskRecord.ID)
		if got, want := execution.Run.IdempotencyKey, wantKey; got != want {
			t.Fatalf("approval run idempotency key = %q, want %q", got, want)
		}
		if got, want := countTaskEventsByType(store.events, taskEventAutoEnqueueTriggered), 1; got != want {
			t.Fatalf("auto enqueue triggered events = %d, want %d", got, want)
		}

		replayed, err := manager.ApproveTask(context.Background(), taskRecord.ID, ExecutionRequest{}, actor)
		if err != nil {
			t.Fatalf("ApproveTask(replay) error = %v", err)
		}
		if replayed.Run.ID != execution.Run.ID {
			t.Fatalf("replayed run ID = %q, want %q", replayed.Run.ID, execution.Run.ID)
		}
		if got, want := len(store.runs), 1; got != want {
			t.Fatalf("runs after replay = %d, want %d", got, want)
		}
	})

	t.Run("Should fail approval when auto enqueue idempotency alias cannot persist", func(t *testing.T) {
		t.Parallel()

		store := &approvalAliasFailureStore{inMemoryManagerStore: newInMemoryManagerStore()}
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:              ScopeWorkspace,
			WorkspaceID:        "ws-auto-approval-alias-failure",
			Title:              "Auto enqueue alias failure",
			ApprovalPolicy:     ApprovalPolicyManual,
			AutoEnqueueOnReady: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		store.failKey = taskExecutionIdempotencyKey(taskRecord.ID, ExecutionActionApproval, "")

		execution, err := manager.ApproveTask(context.Background(), taskRecord.ID, ExecutionRequest{}, actor)
		if err == nil {
			t.Fatal("ApproveTask(alias failure) error = nil, want persistence failure")
		}
		if execution != nil {
			t.Fatalf("ApproveTask(alias failure) execution = %#v, want nil", execution)
		}
		if !strings.Contains(err.Error(), "save approval idempotency alias") {
			t.Fatalf("ApproveTask(alias failure) error = %v, want alias persistence context", err)
		}
	})
}

func TestManagerCompleteRunLeaseResetsBlockRecurrencesOnlyOnCompletion(t *testing.T) {
	t.Parallel()

	t.Run("Should reset block recurrences only after run completion", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		maxAttempts := 3
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeGlobal,
			Title:       "Recurrence reset target",
			MaxAttempts: &maxAttempts,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		first, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID: taskRecord.ID,
			Kind:   BlockKindNeedsInput,
			Reason: "first loop",
		}, actor)
		if err != nil {
			t.Fatalf("BlockTask(first) error = %v", err)
		}
		if _, err := manager.ClearTaskBlock(
			context.Background(),
			taskRecord.ID,
			first.ID,
			"resolved",
			actor,
		); err != nil {
			t.Fatalf("ClearTaskBlock(first) error = %v", err)
		}
		second, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID: taskRecord.ID,
			Kind:   BlockKindNeedsInput,
			Reason: "second loop",
		}, actor)
		if err != nil {
			t.Fatalf("BlockTask(second) error = %v", err)
		}
		if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)].Count, 1; got != want {
			t.Fatalf("recurrence count after reblock = %d, want %d", got, want)
		}
		if _, err := manager.ClearTaskBlock(
			context.Background(),
			taskRecord.ID,
			second.ID,
			"resolved again",
			actor,
		); err != nil {
			t.Fatalf("ClearTaskBlock(second) error = %v", err)
		}
		if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)].Count, 1; got != want {
			t.Fatalf("recurrence count after clear = %d, want %d", got, want)
		}

		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		agent := agentActorContextForTest("sess-reset", "ws-reset")
		claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
			Scope:            ScopeGlobal,
			ClaimerSessionID: "sess-reset",
			LeaseDuration:    2 * time.Minute,
			Now:              claimNow,
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if claim.Run.ID != run.ID {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", claim.Run.ID, run.ID)
		}
		if _, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result:     RunResult{Value: json.RawMessage(`{"ok":true}`)},
			Now:        claimNow.Add(time.Minute),
		}, agent); err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		if got, want := len(store.blockRecurrences), 0; got != want {
			t.Fatalf("block recurrence rows after completion = %d, want %d", got, want)
		}
	})
}

type approvalAliasFailureStore struct {
	*inMemoryManagerStore
	failKey string
}

func (s *approvalAliasFailureStore) SaveTaskRunIdempotency(
	ctx context.Context,
	record RunIdempotency,
) error {
	if record.IdempotencyKey == s.failKey {
		return errors.New("approval alias persistence failed")
	}
	return s.inMemoryManagerStore.SaveTaskRunIdempotency(ctx, record)
}

func TestManagerTaskBlockLifecycleRejectsSecretMaterial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(context.Context, *Service, *inMemoryManagerStore, *Task, ActorContext) error
	}{
		{
			name: "Should reject raw secret material in block reason",
			run: func(ctx context.Context, manager *Service, _ *inMemoryManagerStore, taskRecord *Task, actor ActorContext) error {
				_, err := manager.BlockTask(ctx, BlockRequest{
					TaskID: taskRecord.ID,
					Kind:   BlockKindNeedsInput,
					Reason: "mcp_auth_token=raw-mcp-secret",
				}, actor)
				return err
			},
		},
		{
			name: "Should reject raw secret material in block details",
			run: func(ctx context.Context, manager *Service, _ *inMemoryManagerStore, taskRecord *Task, actor ActorContext) error {
				_, err := manager.BlockTask(ctx, BlockRequest{
					TaskID:  taskRecord.ID,
					Kind:    BlockKindNeedsInput,
					Reason:  "needs input",
					Details: json.RawMessage(`{"mcp_auth_token":"raw-mcp-secret"}`),
				}, actor)
				return err
			},
		},
		{
			name: "Should reject raw secret material in clear note",
			run: func(ctx context.Context, manager *Service, _ *inMemoryManagerStore, taskRecord *Task, actor ActorContext) error {
				block, err := manager.BlockTask(ctx, BlockRequest{
					TaskID: taskRecord.ID,
					Kind:   BlockKindNeedsInput,
					Reason: "needs input",
				}, actor)
				if err != nil {
					return err
				}
				_, err = manager.ClearTaskBlock(ctx, taskRecord.ID, block.ID, "oauth_code=raw-oauth-secret", actor)
				return err
			},
		},
		{
			name: "Should reject raw secret material in recover note",
			run: func(ctx context.Context, manager *Service, _ *inMemoryManagerStore, taskRecord *Task, actor ActorContext) error {
				_, err := manager.RecoverTask(ctx, taskRecord.ID, "pkce_verifier=raw-pkce-secret", actor)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(ctx, CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-secret-reject-" + strconv.Itoa(len(tt.name)),
				Title:       "Secret rejection target",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			if err := tt.run(ctx, manager, store, taskRecord, actor); !errors.Is(err, ErrValidation) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, ErrValidation)
			}
			for _, block := range store.blocks[taskRecord.ID] {
				if !block.ClearedAt.IsZero() {
					t.Fatalf("block %#v was cleared after rejected secret-bearing mutation", block)
				}
				if strings.Contains(block.Reason, "raw-mcp-secret") ||
					strings.Contains(string(block.Details), "raw-mcp-secret") ||
					strings.Contains(block.ClearNote, "raw-oauth-secret") {
					t.Fatalf("block %#v persisted raw secret material after rejected mutation", block)
				}
			}
		})
	}
}

func TestTaskBlockHookPayloadsRedactStoredSecretMaterial(t *testing.T) {
	t.Parallel()

	var blocked hookspkg.TaskBlockedPayload
	var unblocked hookspkg.TaskUnblockedPayload
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
		blocked: func(
			_ context.Context,
			payload hookspkg.TaskBlockedPayload,
		) (hookspkg.TaskBlockedPayload, error) {
			blocked = payload
			return payload, nil
		},
		unblocked: func(
			_ context.Context,
			payload hookspkg.TaskUnblockedPayload,
		) (hookspkg.TaskUnblockedPayload, error) {
			unblocked = payload
			return payload, nil
		},
	}))
	actor := validActorContext()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	taskRecord := Task{
		ID:          "task-secret-hook-payload",
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-secret-hook-payload",
		Title:       "Secret hook payload",
		Status:      TaskStatusBlocked,
		CreatedBy:   actor.Actor,
		Origin:      actor.Origin,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	block := TaskBlock{
		ID:          "block-secret-hook-payload",
		WorkspaceID: taskRecord.WorkspaceID,
		TaskID:      taskRecord.ID,
		Kind:        BlockKindNeedsInput,
		Reason:      "mcp_auth_token=raw-mcp-secret",
		Details: json.RawMessage(
			`{"mcp_auth_token":"raw-mcp-secret","nested":{"authorization":"Bearer bearer-secret-token","safe":"ok"}}`,
		),
		CreatedBy: actor.Actor,
		CreatedAt: now,
		ClearedAt: now.Add(time.Minute),
		ClearedBy: actor.Actor,
		ClearNote: "oauth_code=raw-oauth-secret",
	}

	manager.dispatchTaskBlocked(context.Background(), block, taskRecord, actor, nil)
	manager.dispatchTaskUnblocked(context.Background(), block, taskRecord, actor)

	assertPayloadRedactsStoredTaskBlockSecrets(t, "task.blocked payload", blocked)
	assertPayloadRedactsStoredTaskBlockSecrets(t, "task.unblocked payload", unblocked)
	if blocked.Reason == block.Reason {
		t.Fatalf("blocked.Reason = %q, want redacted reason", blocked.Reason)
	}
	if string(blocked.Details) == string(block.Details) {
		t.Fatalf("blocked.Details = %s, want redacted details", blocked.Details)
	}
	if unblocked.ClearNote == block.ClearNote {
		t.Fatalf("unblocked.ClearNote = %q, want redacted clear note", unblocked.ClearNote)
	}
}

func assertPayloadRedactsStoredTaskBlockSecrets(t *testing.T, label string, payload any) {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", label, err)
	}
	rendered := string(encoded)
	for _, forbidden := range []string{
		"raw-mcp-secret",
		"bearer-secret-token",
		"raw-oauth-secret",
		"Bearer bearer-secret-token",
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("%s contains forbidden secret %q: %s", label, forbidden, rendered)
		}
	}
	if !strings.Contains(rendered, "[REDACTED]") {
		t.Fatalf("%s = %s, want redaction marker", label, rendered)
	}
}

func TestManagerClaimNextRunRequiresWriteAuthority(t *testing.T) {
	t.Parallel()

	manager := newTaskManagerForTest(t, newInMemoryManagerStore())
	actor := validActorContext()
	actor.Authority.Write = false
	actor.Authority.CreateGlobal = false
	actor.Authority.CreateWorkspace = false

	if _, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-agent",
	}, actor); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("ClaimNextRun(read-only actor) error = %v, want %v", err, ErrPermissionDenied)
	}
}

func TestManagerReleaseSessionRunLeasesRequeuesActiveRunsStructurally(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	operator := validActorContext()
	agent := agentActorContextForTest("sess-child", "ws-child")

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Structurally released task",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-child",
		LeaseDuration:    time.Minute,
		Now:              now,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	if claim.Run.ID != run.ID || claim.Run.ClaimTokenHash == "" {
		t.Fatalf("claim = %#v, want active lease for %q", claim, run.ID)
	}

	results, err := manager.ReleaseSessionRunLeases(context.Background(), SessionLeaseRelease{
		SessionID: "sess-child",
		Reason:    "ttl_expired",
		Now:       now.Add(30 * time.Second),
	}, operator)
	if err != nil {
		t.Fatalf("ReleaseSessionRunLeases() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	result := results[0]
	if result.PreviousRunStatus != TaskRunStatusClaimed ||
		result.PreviousSessionID != "sess-child" ||
		result.PreviousClaimTokenHash == "" ||
		result.Reason != "ttl_expired" {
		t.Fatalf("release result = %#v, want previous active lease metadata", result)
	}
	persisted, err := store.GetTaskRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if persisted.Status != TaskRunStatusQueued ||
		persisted.SessionID != "" ||
		persisted.ClaimedBy != nil ||
		persisted.ClaimTokenHash != "" ||
		!persisted.LeaseUntil.IsZero() ||
		!persisted.HeartbeatAt.IsZero() {
		t.Fatalf("persisted run after structural release = %#v, want queued and unleased", persisted)
	}
	if _, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
		RunID:         run.ID,
		ClaimToken:    claim.ClaimToken,
		LeaseDuration: time.Minute,
		Now:           now.Add(40 * time.Second),
	}, agent); !errors.Is(err, ErrInvalidClaimToken) {
		t.Fatalf("HeartbeatRunLease(after structural release) error = %v, want %v", err, ErrInvalidClaimToken)
	}

	events, err := store.ListTaskEvents(context.Background(), EventQuery{
		TaskID:    taskRecord.ID,
		RunID:     run.ID,
		EventType: taskEventRunReleased,
	})
	if err != nil {
		t.Fatalf("ListTaskEvents(released) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("release events = %#v, want one task.run_released event", events)
	}
}

type lockedLeaseTestStore struct {
	*inMemoryManagerStore
	mu sync.Mutex
}

func (s *lockedLeaseTestStore) GetTask(ctx context.Context, id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.GetTask(ctx, id)
}

func (s *lockedLeaseTestStore) UpdateTask(ctx context.Context, taskRecord Task, actor ActorContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.UpdateTask(ctx, taskRecord, actor)
}

func (s *lockedLeaseTestStore) ListDependencies(ctx context.Context, taskID string) ([]Dependency, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.ListDependencies(ctx, taskID)
}

func (s *lockedLeaseTestStore) ListDependents(ctx context.Context, taskID string) ([]Dependency, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.ListDependents(ctx, taskID)
}

func (s *lockedLeaseTestStore) ListTaskRuns(ctx context.Context, query RunQuery) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.ListTaskRuns(ctx, query)
}

func (s *lockedLeaseTestStore) GetTaskRun(ctx context.Context, id string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.GetTaskRun(ctx, id)
}

func (s *lockedLeaseTestStore) ListTaskBlocks(
	ctx context.Context,
	taskID string,
	includeCleared bool,
) ([]TaskBlock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.ListTaskBlocks(ctx, taskID, includeCleared)
}

func (s *lockedLeaseTestStore) HasOpenTaskBlocks(ctx context.Context, taskID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.HasOpenTaskBlocks(ctx, taskID)
}

func (s *lockedLeaseTestStore) BlockTaskAndReleaseRun(
	ctx context.Context,
	mutation BlockTaskAndReleaseRunMutation,
) (BlockTaskAndReleaseRunResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.BlockTaskAndReleaseRun(ctx, mutation)
}

func (s *lockedLeaseTestStore) ReleaseRunLease(ctx context.Context, release LeaseRelease) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.ReleaseRunLease(ctx, release)
}

func (s *lockedLeaseTestStore) CreateTaskEvent(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.CreateTaskEvent(ctx, event)
}

func (s *lockedLeaseTestStore) GetTaskEventRecord(ctx context.Context, eventID string) (EventRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inMemoryManagerStore.GetTaskEventRecord(ctx, eventID)
}
