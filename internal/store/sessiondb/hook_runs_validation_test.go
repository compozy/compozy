package sessiondb

import (
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestSessionDBHookRunValidationContract(t *testing.T) {
	t.Run("Should reject invalid hook run outcomes before insert", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			outcome hookspkg.HookRunOutcome
			wantErr string
		}{
			{
				name:    "Should reject an empty outcome",
				outcome: hookspkg.HookRunOutcome(""),
				wantErr: "hooks: invalid hook run outcome \"\"",
			},
			{
				name:    "Should reject an unknown outcome",
				outcome: hookspkg.HookRunOutcome("bogus"),
				wantErr: "hooks: invalid hook run outcome \"bogus\"",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				sessionDB := openTestSessionDB(t, "sess-hook-invalid-outcome")
				err := sessionDB.RecordHookRun(
					testutil.Context(t),
					validHookRunRecordForValidation(tc.outcome),
				)
				if err == nil {
					records, queryErr := sessionDB.QueryHookRuns(testutil.Context(t), store.HookRunQuery{})
					if queryErr != nil {
						t.Fatalf("QueryHookRuns() error = %v", queryErr)
					}
					t.Fatalf("RecordHookRun() error = nil, persisted records = %d", len(records))
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("RecordHookRun() error = %q, want %q", err.Error(), tc.wantErr)
				}

				records, err := sessionDB.QueryHookRuns(testutil.Context(t), store.HookRunQuery{})
				if err != nil {
					t.Fatalf("QueryHookRuns() error = %v", err)
				}
				if got, want := len(records), 0; got != want {
					t.Fatalf("len(records) = %d, want %d", got, want)
				}
			})
		}
	})

	t.Run("Should reject malformed persisted hook run modes", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-hook-invalid-mode")
		insertRawHookRunForValidation(t, sessionDB, "sideways", string(hookspkg.HookRunOutcomeApplied))

		_, err := sessionDB.QueryHookRuns(testutil.Context(t), store.HookRunQuery{})
		if err == nil {
			t.Fatal("QueryHookRuns() error = nil, want invalid mode error")
		}
		if got, want := err.Error(), "hooks: invalid hook mode \"sideways\""; got != want {
			t.Fatalf("QueryHookRuns() error = %q, want %q", got, want)
		}
	})

	t.Run("Should reject malformed persisted hook run outcomes", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-hook-invalid-scan-outcome")
		insertRawHookRunForValidation(t, sessionDB, string(hookspkg.HookModeSync), "bogus")

		_, err := sessionDB.QueryHookRuns(testutil.Context(t), store.HookRunQuery{})
		if err == nil {
			t.Fatal("QueryHookRuns() error = nil, want invalid outcome error")
		}
		if got, want := err.Error(), "hooks: invalid hook run outcome \"bogus\""; got != want {
			t.Fatalf("QueryHookRuns() error = %q, want %q", got, want)
		}
	})
}

func validHookRunRecordForValidation(outcome hookspkg.HookRunOutcome) hookspkg.HookRunRecord {
	return hookspkg.HookRunRecord{
		HookName:      "permission-audit",
		Event:         hookspkg.HookPermissionRequest,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Duration:      25 * time.Millisecond,
		Outcome:       outcome,
		DispatchDepth: 1,
		RecordedAt:    time.Date(2026, 5, 17, 15, 30, 0, 0, time.UTC),
	}
}

func insertRawHookRunForValidation(t *testing.T, sessionDB *SessionDB, mode string, outcome string) {
	t.Helper()

	_, err := sessionDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO hook_runs (
			id, hook_name, event, source, mode, duration_ns, outcome, dispatch_depth,
			patch_applied, error, required, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"hook-invalid-persisted-value",
		"permission-audit",
		hookspkg.HookPermissionRequest.String(),
		hookspkg.HookSourceConfig.String(),
		mode,
		(25 * time.Millisecond).Nanoseconds(),
		outcome,
		1,
		nil,
		nil,
		0,
		store.FormatTimestamp(time.Date(2026, 5, 17, 15, 31, 0, 0, time.UTC)),
	)
	if err != nil {
		t.Fatalf("insert malformed hook run: %v", err)
	}
}
