package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestDailyBudgetReceiptKeepsOriginalDayAcrossReplay(t *testing.T) {
	t.Run("Should keep a confirmed turn in its original local day after a zone change", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 0, 30, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		stats := store.TokenStatsUpdate{SessionID: sessionID, TurnID: "original-zone",
			AgentName: "coder", TotalTokens: new(int64(6)), CostStatus: "unknown",
			CostSource: "none", Turns: 1, UpdatedAt: at}
		day := store.LocalDay(at)
		daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: day,
			AgentName: stats.AgentName, TotalTokens: stats.TotalTokens, CostStatus: stats.CostStatus,
			CostSource: stats.CostSource, Turns: 1, UpdatedAt: at}
		if accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil || !accepted {
			t.Fatalf("original receipt accepted=%t err=%v", accepted, err)
		}
		_, offset := at.Zone()
		earlierZone := time.FixedZone("earlier", offset-3600)
		// Replay can supply a different day for the same instant after a local
		// zone change. Avoid mutating the process-wide time.Local in a race test.
		daily.Day = at.In(earlierZone).Format("2006-01-02")
		if daily.Day == day {
			t.Fatal("fixture must cross midnight in the changed zone")
		}
		if accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil || accepted {
			t.Fatalf("replayed receipt accepted=%t err=%v, want idempotent replay", accepted, err)
		}
		var count int
		var receiptDay string
		if err := db.DB().QueryRowContext(ctx, `SELECT count(*), min(day) FROM token_usage_receipts
			WHERE session_id=? AND turn_id=?`, sessionID, stats.TurnID).Scan(&count, &receiptDay); err != nil ||
			count != 1 || receiptDay != day {
			t.Fatalf("receipts=%d day=%q err=%v, want one receipt in %q", count, receiptDay, err, day)
		}
	})
}
