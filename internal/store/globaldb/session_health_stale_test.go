package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBSessionHealthStaleDetection(t *testing.T) {
	t.Parallel()

	t.Run("Should not mark active prompt rows stale from idle presence cutoff", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(
			t,
			globalDB,
			"heartbeat-active-prompt",
			"sess-active-prompt",
		)
		baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		health := heartbeatSessionHealthForTest(sessionID, workspaceID, "coder", baseAt)
		health.State = heartbeat.SessionHealthStatePrompting
		health.Health = heartbeat.SessionHealthHealthy
		health.ActivePrompt = true
		health.Attachable = true
		health.EligibleForWake = false
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonPromptActive)
		health.LastActivityAt = baseAt.Add(time.Hour)
		health.LastPresenceAt = baseAt
		health.UpdatedAt = baseAt.Add(time.Hour)
		if _, err := globalDB.UpsertSessionHealth(ctx, health); err != nil {
			t.Fatalf("UpsertSessionHealth(active prompt) error = %v", err)
		}

		marked, err := globalDB.MarkSessionHealthStale(
			ctx,
			baseAt.Add(30*time.Minute),
			baseAt.Add(2*time.Hour),
		)
		if err != nil {
			t.Fatalf("MarkSessionHealthStale(active prompt) error = %v", err)
		}
		if marked != 0 {
			t.Fatalf("MarkSessionHealthStale(active prompt) = %d, want 0", marked)
		}

		stored, err := globalDB.GetSessionHealth(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetSessionHealth(active prompt) error = %v", err)
		}
		if stored.Health != heartbeat.SessionHealthHealthy ||
			stored.State != heartbeat.SessionHealthStatePrompting ||
			!stored.ActivePrompt ||
			stored.IneligibilityReason != string(heartbeat.SessionHealthReasonPromptActive) {
			t.Fatalf("active prompt health = %#v, want prompt-active non-stale row", stored)
		}
	})
}
