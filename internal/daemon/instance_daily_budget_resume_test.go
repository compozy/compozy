package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/agentinstance"
)

type resumeBudgetReaderStub struct {
	limits agentinstance.DailyBudgetLimits
}

func (r resumeBudgetReaderStub) DailyBudgetLimits(context.Context, string) (agentinstance.DailyBudgetLimits, error) {
	return r.limits, nil
}

type resumeBudgetReconcilerStub struct {
	calls int
}

func (r *resumeBudgetReconcilerStub) ReconcileDailyBudgetInstance(context.Context, string) error {
	r.calls++
	return errors.New("old session ledger missing")
}

func TestDailyBudgetResumePreflightLeavesUnconfiguredInstancesUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		caps  agentinstance.DailyBudgetLimits
		calls int
	}{
		{name: "Should not open unreadable history when daily limits are disabled"},
		{name: "Should reconcile active token caps before resuming", caps: agentinstance.DailyBudgetLimits{
			Tokens: new(int64(100)),
		}, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reconciler := &resumeBudgetReconcilerStub{}
			err := reconcileInstanceBudgetBeforeResume(
				t.Context(),
				"instance",
				resumeBudgetReaderStub{tc.caps},
				reconciler,
			)
			if (err != nil) != (tc.calls == 1) {
				t.Fatalf("resume preflight error=%v, want error only with configured cap", err)
			}
			if reconciler.calls != tc.calls {
				t.Fatalf("budget ledger scans=%d, want %d", reconciler.calls, tc.calls)
			}
		})
	}
}
