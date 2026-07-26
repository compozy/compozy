package store

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestValidStopReason(t *testing.T) {
	t.Parallel()

	validReasons := []StopReason{
		StopCompleted,
		StopUserCanceled,
		StopMaxIterations,
		StopLoopDetected,
		StopTimeout,
		StopBudgetExceeded,
		StopError,
		StopAgentCrashed,
		StopHookStopped,
		StopShutdown,
	}

	for _, reason := range validReasons {
		t.Run("Should validate "+string(reason), func(t *testing.T) {
			t.Parallel()

			if !ValidStopReason(reason) {
				t.Fatalf("ValidStopReason(%q) = false, want true", reason)
			}
		})
	}

	invalidReasons := []StopReason{"", "unknown", " completed "}
	for _, reason := range invalidReasons {
		name := strings.TrimSpace(string(reason))
		if name == "" {
			name = "empty"
		}
		name = strings.ReplaceAll(name, " ", "_")
		t.Run("Should reject invalid "+name, func(t *testing.T) {
			t.Parallel()

			if ValidStopReason(reason) {
				t.Fatalf("ValidStopReason(%q) = true, want false", reason)
			}
		})
	}
}

func TestSessionMetaValidateStopReason(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 9, 11, 0, 0, 0, time.UTC)
	validReason := StopAgentCrashed
	invalidReason := StopReason("invalid")

	tests := []struct {
		name      string
		reason    *StopReason
		wantError bool
	}{
		{name: "Should validate nil stop reason", reason: nil},
		{name: "Should validate supported stop reason", reason: &validReason},
		{name: "Should reject invalid stop reason", reason: &invalidReason, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			meta := SessionMeta{
				ID:                   "sess-meta",
				AgentName:            "coder",
				WorkspaceID:          "ws-meta",
				NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
				State:                "stopped",
				StopReason:           tt.reason,
				CreatedAt:            now,
				UpdatedAt:            now,
			}

			err := meta.Validate()
			if tt.wantError && err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if tt.wantError && !strings.Contains(err.Error(), "invalid session stop reason") {
				t.Fatalf("Validate() error = %v, want to contain %q", err, "invalid session stop reason")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSessionStopReasonValidationContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 17, 23, 0, 0, 0, time.UTC)

	t.Run("Should reject invalid stop reason on session info", func(t *testing.T) {
		t.Parallel()

		info := SessionInfo{
			ID:          "sess-info",
			AgentName:   "coder",
			WorkspaceID: "ws-info",
			State:       "stopped",
			StopReason:  StopReason("invalid"),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		err := info.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid session stop reason") {
			t.Fatalf("Validate() error = %v, want invalid session stop reason", err)
		}
	})

	t.Run("Should accept omitted and supported stop reasons on session info", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name   string
			reason StopReason
		}{
			{name: "omitted"},
			{name: "supported", reason: StopCompleted},
		} {
			t.Run("Should accept "+tc.name+" reason", func(t *testing.T) {
				t.Parallel()

				info := SessionInfo{
					ID:          "sess-info-" + tc.name,
					AgentName:   "coder",
					WorkspaceID: "ws-info",
					State:       "stopped",
					StopReason:  tc.reason,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				if err := info.Validate(); err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
			})
		}
	})

	t.Run("Should reject invalid stop reason on session state update", func(t *testing.T) {
		t.Parallel()

		invalidReason := "invalid"
		update := SessionStateUpdate{
			ID:            "sess-update",
			State:         "stopped",
			StopReasonSet: true,
			StopReason:    &invalidReason,
			UpdatedAt:     now,
		}

		err := update.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid session stop reason") {
			t.Fatalf("Validate() error = %v, want invalid session stop reason", err)
		}
	})

	t.Run("Should accept clear and supported stop reasons on session state update", func(t *testing.T) {
		t.Parallel()

		blankReason := "   "
		validReason := string(StopUserCanceled)
		for _, tc := range []struct {
			name      string
			setReason bool
			reason    *string
		}{
			{name: "unset"},
			{name: "nil clear", setReason: true},
			{name: "blank clear", setReason: true, reason: &blankReason},
			{name: "supported", setReason: true, reason: &validReason},
		} {
			t.Run("Should accept "+tc.name+" reason", func(t *testing.T) {
				t.Parallel()

				update := SessionStateUpdate{
					ID:            "sess-update-" + strings.ReplaceAll(tc.name, " ", "-"),
					State:         "stopped",
					StopReasonSet: tc.setReason,
					StopReason:    tc.reason,
					UpdatedAt:     now,
				}
				if err := update.Validate(); err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
			})
		}
	})
}
