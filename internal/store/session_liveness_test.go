package store

import (
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestSessionLivenessMetaValidate(t *testing.T) {
	t.Parallel()

	t.Run("Should accept nil metadata", func(t *testing.T) {
		t.Parallel()

		var meta *SessionLivenessMeta
		if err := meta.Validate(); err != nil {
			t.Fatalf("Validate(nil) error = %v", err)
		}
	})

	t.Run("Should reject a negative subprocess pid", func(t *testing.T) {
		t.Parallel()

		meta := &SessionLivenessMeta{SubprocessPID: -1}
		if err := meta.Validate(); err == nil {
			t.Fatal("Validate(negative pid) error = nil, want non-nil")
		}
	})

	t.Run("Should reject an invalid stall state", func(t *testing.T) {
		t.Parallel()

		meta := &SessionLivenessMeta{StallState: "blocked"}
		if err := meta.Validate(); err == nil {
			t.Fatal("Validate(invalid stall state) error = nil, want non-nil")
		}
	})

	t.Run("Should require a reason when stall state is set", func(t *testing.T) {
		t.Parallel()

		meta := &SessionLivenessMeta{StallState: SessionStallStateDetected}
		if err := meta.Validate(); err == nil {
			t.Fatal("Validate(stall state without reason) error = nil, want non-nil")
		}
	})

	t.Run("Should require a stall state when a reason is set", func(t *testing.T) {
		t.Parallel()

		meta := &SessionLivenessMeta{StallReason: SessionStallReasonActivityTimeout}
		if err := meta.Validate(); err == nil {
			t.Fatal("Validate(stall reason without state) error = nil, want non-nil")
		}
	})

	t.Run("Should accept valid stalled-session metadata", func(t *testing.T) {
		t.Parallel()

		meta := &SessionLivenessMeta{
			SubprocessPID: 42,
			StallState:    SessionStallStateDetected,
			StallReason:   SessionStallReasonActivityTimeout,
		}
		if err := meta.Validate(); err != nil {
			t.Fatalf("Validate(valid stalled metadata) error = %v", err)
		}
	})
}

func TestCloneSessionLivenessMeta(t *testing.T) {
	t.Parallel()

	t.Run("Should deep-copy and normalize session liveness metadata", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2026, 4, 21, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
		lastUpdateAt := startedAt.Add(30 * time.Second)
		meta := &SessionLivenessMeta{
			SubprocessPID:       101,
			SubprocessStartedAt: &startedAt,
			LastUpdateAt:        &lastUpdateAt,
			StallState:          "  " + SessionStallStateDetected + "  ",
			StallReason:         "  " + SessionStallReasonActivityTimeout + "  ",
		}

		cloned := CloneSessionLivenessMeta(meta)
		if cloned == nil {
			t.Fatal("CloneSessionLivenessMeta() = nil, want metadata")
		}
		if cloned == meta {
			t.Fatal("CloneSessionLivenessMeta() returned the original pointer")
		}
		if got := cloned.SubprocessPID; got != meta.SubprocessPID {
			t.Fatalf("cloned.SubprocessPID = %d, want %d", got, meta.SubprocessPID)
		}
		if cloned.SubprocessStartedAt == meta.SubprocessStartedAt {
			t.Fatal("SubprocessStartedAt pointer reused, want deep copy")
		}
		if cloned.LastUpdateAt == meta.LastUpdateAt {
			t.Fatal("LastUpdateAt pointer reused, want deep copy")
		}
		if got := cloned.SubprocessStartedAt.Location(); got != time.UTC {
			t.Fatalf("cloned.SubprocessStartedAt location = %v, want UTC", got)
		}
		if got := cloned.LastUpdateAt.Location(); got != time.UTC {
			t.Fatalf("cloned.LastUpdateAt location = %v, want UTC", got)
		}
		if got := cloned.StallState; got != SessionStallStateDetected {
			t.Fatalf("cloned.StallState = %q, want %q", got, SessionStallStateDetected)
		}
		if got := cloned.StallReason; got != SessionStallReasonActivityTimeout {
			t.Fatalf("cloned.StallReason = %q, want %q", got, SessionStallReasonActivityTimeout)
		}
		if CloneSessionLivenessMeta(nil) != nil {
			t.Fatal("CloneSessionLivenessMeta(nil) != nil, want nil")
		}
	})
}

func TestSessionActivityMetaValidateCloneAndIdleSeconds(t *testing.T) {
	t.Parallel()

	t.Run("Should validate activity counters", func(t *testing.T) {
		t.Parallel()

		var nilActivity *SessionActivityMeta
		if err := nilActivity.Validate(); err != nil {
			t.Fatalf("Validate(nil activity) error = %v", err)
		}
		for _, invalid := range []*SessionActivityMeta{
			{IterationCurrent: -1},
			{IterationMax: -1},
			{IdleSeconds: -1},
		} {
			if err := invalid.Validate(); err == nil {
				t.Fatalf("Validate(%#v) error = nil, want non-nil", invalid)
			}
		}
	})

	t.Run("Should deep-copy and normalize activity metadata", func(t *testing.T) {
		t.Parallel()

		turnStartedAt := time.Date(2026, 4, 21, 13, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
		lastActivityAt := turnStartedAt.Add(2 * time.Minute)
		lastProgressAt := turnStartedAt.Add(3 * time.Minute)
		meta := &SessionActivityMeta{
			TurnID:             "  turn-1  ",
			TurnSource:         "  user  ",
			TurnStartedAt:      &turnStartedAt,
			LastActivityAt:     &lastActivityAt,
			LastActivityKind:   "  tool_call  ",
			LastActivityDetail: "  running  ",
			CurrentTool:        "  shell  ",
			ToolCallID:         "  call-1  ",
			LastProgressAt:     &lastProgressAt,
			IterationCurrent:   2,
			IterationMax:       4,
			IdleSeconds:        5,
		}

		cloned := CloneSessionActivityMeta(meta)
		if cloned == nil {
			t.Fatal("CloneSessionActivityMeta() = nil, want metadata")
		}
		if cloned.TurnStartedAt == meta.TurnStartedAt {
			t.Fatal("TurnStartedAt pointer reused, want deep copy")
		}
		if cloned.LastActivityAt == meta.LastActivityAt {
			t.Fatal("LastActivityAt pointer reused, want deep copy")
		}
		if cloned.LastProgressAt == meta.LastProgressAt {
			t.Fatal("LastProgressAt pointer reused, want deep copy")
		}
		if got := cloned.TurnID; got != "turn-1" {
			t.Fatalf("cloned.TurnID = %q, want trimmed turn id", got)
		}
		if got := cloned.CurrentTool; got != "shell" {
			t.Fatalf("cloned.CurrentTool = %q, want trimmed tool", got)
		}
		if got := cloned.LastActivityAt.Location(); got != time.UTC {
			t.Fatalf("cloned.LastActivityAt location = %v, want UTC", got)
		}
		if CloneSessionActivityMeta(nil) != nil {
			t.Fatal("CloneSessionActivityMeta(nil) != nil, want nil")
		}
	})

	t.Run("Should report idle seconds defensively", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 21, 14, 0, 0, 0, time.UTC)
		lastActivityAt := now.Add(-45 * time.Second)
		if got := SessionActivityIdleSeconds(&SessionActivityMeta{LastActivityAt: &lastActivityAt}, now); got != 45 {
			t.Fatalf("SessionActivityIdleSeconds() = %d, want 45", got)
		}
		futureActivityAt := now.Add(time.Second)
		if got := SessionActivityIdleSeconds(&SessionActivityMeta{LastActivityAt: &futureActivityAt}, now); got != 0 {
			t.Fatalf("SessionActivityIdleSeconds(future) = %d, want 0", got)
		}
		if got := SessionActivityIdleSeconds(nil, now); got != 0 {
			t.Fatalf("SessionActivityIdleSeconds(nil) = %d, want 0", got)
		}
	})
}

func TestHookRunQueryValidate(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a valid hook-run query", func(t *testing.T) {
		t.Parallel()

		valid := HookRunQuery{
			SessionID: "sess-1",
			Event:     "session.post_resume",
			Outcome:   hookspkg.HookRunOutcomeApplied,
			Limit:     10,
		}
		if err := valid.Validate(); err != nil {
			t.Fatalf("Validate(valid query) error = %v", err)
		}
	})

	t.Run("Should reject an invalid hook-run outcome", func(t *testing.T) {
		t.Parallel()

		invalidOutcome := HookRunQuery{
			SessionID: "sess-1",
			Event:     "session.post_resume",
			Outcome:   hookspkg.HookRunOutcome("broken"),
			Limit:     10,
		}
		if err := invalidOutcome.Validate(); err == nil {
			t.Fatal("Validate(invalid outcome) error = nil, want non-nil")
		}
	})

	t.Run("Should reject a negative query limit", func(t *testing.T) {
		t.Parallel()

		invalidLimit := HookRunQuery{Limit: -1}
		if err := invalidLimit.Validate(); err == nil {
			t.Fatal("Validate(invalid limit) error = nil, want non-nil")
		}
	})
}
