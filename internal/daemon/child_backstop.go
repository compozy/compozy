package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

// childSequenceReadTimeout bounds one durable high-water sequence read. The read
// is a single indexed lookup, so a slower answer means the child's run store is
// wedged; the backstop treats that as "no progress observed" instead of letting
// the parent poll loop block, which is the deadlock this component exists to
// prevent.
const childSequenceReadTimeout = 2 * time.Second

// childLiveness tracks a child run's durable journal high-water sequence for the
// daemon backstop. advanced reports whether the sequence moved, resetting the
// stall clock; the caller acts when now-lastSeen exceeds the backstop budget.
type childLiveness struct {
	lastSeq  uint64
	lastSeen time.Time
}

func (c *childLiveness) advanced(seq uint64, now time.Time) bool {
	if seq > c.lastSeq {
		c.lastSeq, c.lastSeen = seq, now
		return true
	}
	return false
}

// wedged reports whether the child has failed to advance its durable sequence for
// the whole budget. An unreadable sequence never resets the clock, so a child
// whose journal never becomes observable is reaped like a silent one.
func (c *childLiveness) wedged(now time.Time, budget time.Duration) bool {
	return budget > 0 && now.Sub(c.lastSeen) >= budget
}

// childBackstopBudget resolves the durable per-child liveness budget, returning 0
// when the backstop must stay disarmed. The budget is held strictly greater than
// the fast watchdog's idle window so the in-attempt layer always gets the first
// chance to self-heal (ADR-003 nested budgets); a policy that violates the
// invariant is corrected here rather than trusted.
func childBackstopBudget(policy model.StallPolicy) time.Duration {
	if !policy.Enabled {
		return 0
	}
	budget := policy.ChildTimeout
	if budget <= policy.IdleTimeout {
		budget = policy.IdleTimeout * 2
	}
	if budget <= 0 {
		return 0
	}
	return budget
}

// childBackstop is the durable per-child liveness guard described by ADR-003.
// One instance guards exactly one child run and is owned by the single
// waitForTaskMultiChild call that awaits that child, so reaping a wedged child
// never touches a sibling. A nil backstop is the disarmed policy and every method
// is a no-op on it.
type childBackstop struct {
	manager  *RunManager
	runID    string
	budget   time.Duration
	liveness childLiveness
	lease    *runDBLease
}

// newChildBackstop arms the backstop for one child, or returns nil when the
// resolved stall policy disarms it.
func (m *RunManager) newChildBackstop(runID string, policy model.StallPolicy) *childBackstop {
	budget := childBackstopBudget(policy)
	if budget <= 0 {
		return nil
	}
	return &childBackstop{
		manager:  m,
		runID:    runID,
		budget:   budget,
		liveness: childLiveness{lastSeen: m.now()},
	}
}

// check reaps the child when its durable journal sequence has not advanced for
// the whole budget. A child that is still advancing is never touched.
func (b *childBackstop) check(ctx context.Context) {
	if b == nil {
		return
	}
	now := b.manager.now()
	if seq, ok := b.sequence(ctx); ok && b.liveness.advanced(seq, now) {
		return
	}
	if !b.liveness.wedged(now, b.budget) {
		return
	}
	b.reap(ctx, now)
}

// reap cancels the wedged child through the manager's normal cancel path so it
// settles on a terminal status and the parent join proceeds. The stall clock
// restarts afterwards: a child that somehow survives the cancel is reaped again
// on the next budget instead of being abandoned after one attempt.
func (b *childBackstop) reap(ctx context.Context, now time.Time) {
	slog.Default().Warn(
		"daemon: reaping wedged multi-run child",
		"run_id", b.runID,
		"budget", b.budget.String(),
		"idle_for", now.Sub(b.liveness.lastSeen).String(),
		"last_sequence", b.liveness.lastSeq,
	)
	if err := b.manager.Cancel(detachContext(ctx), b.runID); err != nil {
		slog.Default().Warn("daemon: cancel wedged multi-run child", "run_id", b.runID, "error", err)
	}
	b.liveness.lastSeen = now
}

// sequence reads the child's durable journal high-water sequence. A failed read
// reports no reading rather than an error: the stall clock keeps running, so a
// child whose journal never becomes observable is treated as making no progress.
func (b *childBackstop) sequence(ctx context.Context) (uint64, bool) {
	readCtx, cancel := context.WithTimeout(detachContext(ctx), childSequenceReadTimeout)
	defer cancel()
	if b.lease == nil {
		lease, err := b.manager.acquireRunDB(readCtx, b.runID)
		if err != nil {
			slog.Default().Debug("daemon: child liveness store unavailable", "run_id", b.runID, "error", err)
			return 0, false
		}
		b.lease = lease
	}
	seq, err := b.lease.DB().CurrentMaxSequence(readCtx)
	if err != nil {
		slog.Default().Debug("daemon: child liveness sequence unreadable", "run_id", b.runID, "error", err)
		return 0, false
	}
	return seq, true
}

// close releases the child's run store lease held for the lifetime of the wait.
func (b *childBackstop) close() {
	if b == nil || b.lease == nil {
		return
	}
	if err := b.lease.Close(); err != nil {
		slog.Default().Warn("daemon: release child liveness store", "run_id", b.runID, "error", err)
	}
	b.lease = nil
}

// childStallPolicy resolves the stall policy that governs one child run. A child
// without a resolved runtime config falls back to the on-by-default policy rather
// than silently running without a backstop.
func childStallPolicy(cfg *model.RuntimeConfig) model.StallPolicy {
	if cfg != nil {
		return cfg.StallPolicy()
	}
	defaults := &model.RuntimeConfig{}
	defaults.ApplyDefaults()
	return defaults.StallPolicy()
}
