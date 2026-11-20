package observe

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
)

type dailyBudgetReceiptRecovery interface {
	ListDailyBudgetReconcileSessions(context.Context, string) ([]store.DailyBudgetReconcileSession, error)
	DailyBudgetSessionOwner(context.Context, string) (store.DailyBudgetSessionOwnership, error)
	DailyBudgetSessionHasEnforcement(context.Context, string) (bool, error)
	CheckUnresolvedSessionBudgetPolicy(context.Context, string) error
	ListUnresolvedSessionBudgetCandidates(context.Context, string) ([]store.DailyBudgetPotentialOwner, error)
	DailyBudgetInstanceHasLimits(context.Context, string) (bool, error)
	MarkUncertainDailyBudgetDay(context.Context, string, string, time.Time) error
	DailyBudgetRecoveryCursor(context.Context, string) (int64, error)
	AdvanceDailyBudgetRecoveryCursor(context.Context, string, int64) error
	DailyBudgetReceiptState(context.Context, string, string, string) (store.DailyBudgetReceiptState, error)
	BackfillDailyBudgetReceipt(context.Context, store.TokenStatsUpdate, store.TokenUsageDailyUpdate, bool) error
	QuarantineDailyBudgetInstance(context.Context, string) error
	DailyBudgetRecoveryGeneration(context.Context, string) (int64, error)
	ResolveDailyBudgetRecovery(context.Context, string, int64) error
}

var errDailyBudgetLedgerUnreadable = errors.New("local budget ledger unreadable")

// CommitDurableTurnUsage lets a completed ledger write and its usage receipt win the same fence.
func (o *Observer) CommitDurableTurnUsage(
	ctx context.Context, target *session.Session, event acp.AgentEvent, commit func(context.Context) error,
) error {
	info := target.Info()
	if info == nil {
		return commit(ctx)
	}
	o.trackLiveSession(ctx, info)
	o.budgetAdmissionMu.Lock()
	defer o.budgetAdmissionMu.Unlock()
	if err := commit(ctx); err != nil {
		return err
	}
	sessionID, snapshot, ok := o.validateObservedEvent(ctx, info.ID, event)
	if !ok {
		o.markDailyBudgetReceiptPending()
		return nil
	}
	if err := o.aggregateObservedUsage(
		ctx,
		sessionID,
		snapshot,
		event,
		observedEventTimestamp(event, o.now),
	); err != nil {
		o.markDailyBudgetReceiptPending()
		o.logger.WarnContext(ctx, "observe: recover pending durable usage receipt",
			"session_id", sessionID, "turn_id", event.TurnID, "error", err)
	}
	return nil
}

// WithDailyBudgetAdmission linearizes the final durable admission transaction
// against completed-turn receipt writes and their dirty-generation marking.
// Expensive runtime setup happens outside this short critical section; pause
// effects must run after it has been released.
func (o *Observer) WithDailyBudgetAdmission(
	ctx context.Context, sessionID string, commit func(context.Context) error,
) error {
	o.budgetAdmissionMu.RLock()
	defer o.budgetAdmissionMu.RUnlock()
	if err := o.EnsureDailyBudgetReady(ctx, sessionID); err != nil {
		return fmt.Errorf("%w: %w", store.ErrDailyBudgetRecoveryPending, err)
	}
	return commit(ctx)
}

// WithDailyBudgetUpdate fences the historical scan and publication of new caps together.
func (o *Observer) WithDailyBudgetUpdate(
	ctx context.Context, instanceID string, commit func(context.Context) error,
) error {
	return o.WithDailyBudgetMutation(ctx, func(ctx context.Context) error {
		if err := o.ReconcileDailyBudgetInstance(ctx, instanceID); err != nil {
			return err
		}
		return commit(ctx)
	})
}

// WithDailyBudgetMutation keeps a budget state change atomic with completed
// turn usage. The callback can decide whether history needs reconciliation.
func (o *Observer) WithDailyBudgetMutation(ctx context.Context, commit func(context.Context) error) error {
	o.budgetAdmissionMu.Lock()
	defer o.budgetAdmissionMu.Unlock()
	return commit(ctx)
}

// ReconcileDailyBudgetReceipts closes the crash window between a durable Done
// event and its global receipt. Every configured owner's history is scanned
// from its durable sequence cursor, including prior days after long downtime.
// A receipt and its cursor advancement are separately replay-safe; a crash
// between them simply rechecks the receipt on restart.
func (o *Observer) ReconcileDailyBudgetReceipts(ctx context.Context) error {
	return o.reconcileDailyBudgetReceipts(ctx, "", true)
}

// ReconcileDailyBudgetInstance also handles a previously unconfigured
// instance. Call it before publishing new caps, so earlier usage today is
// present in the receipt fence before that instance becomes budgeted.
func (o *Observer) ReconcileDailyBudgetInstance(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return errors.New("observe: targeted budget recovery requires an instance id")
	}
	return o.reconcileDailyBudgetReceipts(ctx, instanceID, false)
}

// EnsureDailyBudgetReady is a cheap no-op during normal operation. If any live
// receipt write failed, the next budgeted autonomous turn must first recover
// committed Done events or quarantine the affected instance.
func (o *Observer) EnsureDailyBudgetReady(ctx context.Context, sessionID string) error {
	if sessionID != "" {
		o.mu.RLock()
		clean := o.dailyBudgetDirtyGeneration == o.dailyBudgetReconciledGeneration
		o.mu.RUnlock()
		if clean {
			return nil
		}
		recovery, ok := o.registry.(dailyBudgetReceiptRecovery)
		if !ok {
			return nil
		}
		guarded, err := recovery.DailyBudgetSessionHasEnforcement(ctx, sessionID)
		if err != nil || !guarded {
			return err
		}
		owner, err := recovery.DailyBudgetSessionOwner(ctx, sessionID)
		if err != nil {
			return err
		}
		if owner.Unresolved {
			return recovery.CheckUnresolvedSessionBudgetPolicy(ctx, sessionID)
		}
		if owner.InstanceID == "" {
			return nil
		}
		configured, err := recovery.DailyBudgetInstanceHasLimits(ctx, owner.InstanceID)
		if err != nil || !configured {
			return err
		}
	}
	const maxReconcileAttempts = 3
	for range maxReconcileAttempts {
		o.mu.RLock()
		pending := o.dailyBudgetDirtyGeneration
		reconciled := o.dailyBudgetReconciledGeneration
		o.mu.RUnlock()
		if pending == reconciled {
			return nil
		}
		if err := o.ReconcileDailyBudgetReceipts(ctx); err != nil {
			return fmt.Errorf("observe: pending daily budget receipts could not be reconciled: %w", err)
		}
		o.mu.Lock()
		if o.dailyBudgetReconciledGeneration < pending {
			o.dailyBudgetReconciledGeneration = pending
		}
		stable := o.dailyBudgetDirtyGeneration == o.dailyBudgetReconciledGeneration
		o.mu.Unlock()
		if stable {
			return nil
		}
	}
	return errors.New("observe: daily budget receipts changed during recovery; retry admission")
}

// ReconcileBudgetSessionBeforeDeletion prevents quiet triage cleanup, retention
// and operator deletion from erasing the sole surviving Done event when the
// live global observer failed. A damaged budgeted ledger stays undeleted.
func (o *Observer) ReconcileBudgetSessionBeforeDeletion(ctx context.Context, sessionID string) error {
	recovery, ok := o.registry.(dailyBudgetReceiptRecovery)
	if !ok {
		return nil
	}
	owner, err := recovery.DailyBudgetSessionOwner(ctx, sessionID)
	if err != nil {
		return err
	}
	if owner.Unresolved {
		return o.preserveUnresolvedSessionBudgetBeforeDeletion(ctx, recovery, sessionID)
	}
	if owner.InstanceID == "" {
		return nil
	}
	configured, err := recovery.DailyBudgetInstanceHasLimits(ctx, owner.InstanceID)
	if err != nil {
		return err
	}
	sessions, err := recovery.ListDailyBudgetReconcileSessions(ctx, owner.InstanceID)
	if err != nil {
		return fmt.Errorf("observe: inspect budgeted ledger before delete: %w", err)
	}
	for index := range sessions {
		sess := &sessions[index]
		if sess.ID != sessionID {
			continue
		}
		if err := o.reconcileDailyBudgetSession(ctx, recovery, sess); err != nil {
			if !configured {
				if !errors.Is(err, errDailyBudgetLedgerUnreadable) {
					return err // preserve a readable ledger when the global write needs a retry
				}
				// Preserve the old opt-out deletion behavior. The durable
				// uncertainty marker prevents unsafe retroactive activation
				// until the next local-day boundary.
				if markErr := recovery.MarkUncertainDailyBudgetDay(
					ctx,
					owner.InstanceID,
					sessionID,
					o.now(),
				); markErr != nil {
					return errors.Join(err, markErr)
				}
				return nil
			}
			if pauseErr := recovery.QuarantineDailyBudgetInstance(ctx, owner.InstanceID); pauseErr != nil {
				return errors.Join(err, pauseErr)
			}
			return fmt.Errorf("observe: refusing deletion until budget receipt is durable: %w", err)
		}
		return nil
	}
	return nil
}

func (o *Observer) preserveUnresolvedSessionBudgetBeforeDeletion(
	ctx context.Context, recovery dailyBudgetReceiptRecovery, sessionID string,
) error {
	candidates, err := recovery.ListUnresolvedSessionBudgetCandidates(ctx, sessionID)
	if err != nil {
		return err
	}
	sessions, err := o.registry.ListSessions(ctx, store.SessionListQuery{
		ID: sessionID, ReadScope: store.ReadScope{AllProfiles: true}, Archive: store.SessionArchiveInclude, Limit: 1,
	})
	if err != nil || len(sessions) == 0 {
		return err
	}
	var earliest time.Time
	for _, candidate := range candidates {
		if earliest.IsZero() || candidate.CreatedAt.Before(earliest) {
			earliest = candidate.CreatedAt
		}
	}
	// Ordinary usage projection belongs to the session, including when the
	// intact root proves no current instance could own it.
	sess := &store.DailyBudgetReconcileSession{
		SessionInfo: sessions[0], UnresolvedOwner: true, InstanceCreatedAt: earliest,
	}
	err = o.reconcileDailyBudgetSession(ctx, recovery, sess)
	if err == nil {
		return nil
	}
	if errors.Is(err, agentinstance.ErrBudgetHistoryUncertain) {
		return recovery.CheckUnresolvedSessionBudgetPolicy(ctx, sessionID)
	}
	if !errors.Is(err, errDailyBudgetLedgerUnreadable) {
		return err
	}
	for _, candidate := range candidates {
		// An unreadable opted-out ledger was already deletable. Keep today
		// conservative for each plausible owner if the last copy is removed.
		if markErr := recovery.MarkUncertainDailyBudgetDay(
			ctx,
			candidate.InstanceID,
			sessionID,
			o.now(),
		); markErr != nil {
			return errors.Join(err, markErr)
		}
	}
	return nil
}

func (o *Observer) reconcileDailyBudgetReceipts(ctx context.Context, instanceID string, quarantine bool) error {
	recovery, ok := o.registry.(dailyBudgetReceiptRecovery)
	if !ok {
		return nil // test registries may not implement instance budgets
	}
	sessions, err := recovery.ListDailyBudgetReconcileSessions(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("observe: enumerate daily budget recovery sessions: %w", err)
	}
	owners := make(map[string]int64)
	failedOwners := make(map[string]struct{})
	for index := range sessions {
		sess := &sessions[index]
		if _, seen := owners[sess.InstanceID]; !seen {
			generation, err := recovery.DailyBudgetRecoveryGeneration(ctx, sess.InstanceID)
			if err != nil {
				return err
			}
			owners[sess.InstanceID] = generation
		}
		if err := o.reconcileDailyBudgetSession(ctx, recovery, sess); err != nil {
			if !quarantine || sess.InstanceID == "" || ctx.Err() != nil {
				return err
			}
			failedOwners[sess.InstanceID] = struct{}{}
			if pauseErr := recovery.QuarantineDailyBudgetInstance(ctx, sess.InstanceID); pauseErr != nil {
				return errors.Join(err, fmt.Errorf("observe: quarantine instance %q: %w", sess.InstanceID, pauseErr))
			}
			o.logger.ErrorContext(ctx, "observe: quarantined instance after unreadable budget ledger",
				"instance_id", sess.InstanceID, "session_id", sess.ID, "error", err)
		}
	}
	for owner, generation := range owners {
		if owner == "" {
			continue
		}
		if _, failed := failedOwners[owner]; failed {
			continue
		}
		if err := recovery.ResolveDailyBudgetRecovery(ctx, owner, generation); err != nil {
			return fmt.Errorf("observe: resolve recovered instance %q: %w", owner, err)
		}
	}
	o.logger.InfoContext(ctx, "observe: reconciled daily budget session ledgers", "sessions", len(sessions))
	return nil
}

func (o *Observer) reconcileDailyBudgetSession(
	ctx context.Context,
	recovery dailyBudgetReceiptRecovery,
	sess *store.DailyBudgetReconcileSession,
) (retErr error) {
	if !validObservedSessionID(sess.ID) {
		return fmt.Errorf("observe: invalid budget recovery session id %q", sess.ID)
	}
	ledgerPath := store.SessionDBFile(filepath.Join(o.homePaths.SessionsDir, sess.ID))
	owner := store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: sess.WorkspaceID}
	ledger, err := sessiondb.OpenSessionDBReadOnly(ctx, owner, ledgerPath)
	if err != nil {
		return fmt.Errorf("%w: open budget recovery ledger %q: %w", errDailyBudgetLedgerUnreadable, sess.ID, err)
	}
	defer func() { retErr = errors.Join(retErr, ledger.Close(context.WithoutCancel(ctx))) }()
	snapshot := o.observedSessionSnapshot(ctx, sess.ID, sess.ProfileID, sess.AgentName,
		sess.Provider, sess.Model, sess.WorkspaceID, "", 0, nil)
	lastSequence, err := recovery.DailyBudgetRecoveryCursor(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("observe: read budget ledger cursor %q: %w", sess.ID, err)
	}
	query := store.EventQuery{Type: acp.EventTypeDone, AfterSequence: lastSequence,
		Forward: true, Limit: 128}
	var policyErr error
	for {
		rows, err := ledger.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("%w: query budget recovery ledger %q: %w", errDailyBudgetLedgerUnreadable, sess.ID, err)
		}
		for _, row := range rows {
			query.AfterSequence = row.Sequence
			if err := o.recoverBudgetDone(ctx, recovery, sess, snapshot, row); err != nil {
				if errors.Is(err, agentinstance.ErrBudgetHistoryUncertain) {
					policyErr = err
					continue // keep projecting ordinary usage from later completed turns
				}
				return err
			}
		}
		if !sess.UnresolvedOwner && query.AfterSequence > lastSequence {
			if err := recovery.AdvanceDailyBudgetRecoveryCursor(ctx, sess.ID, query.AfterSequence); err != nil {
				return fmt.Errorf("observe: advance budget ledger cursor %q: %w", sess.ID, err)
			}
			lastSequence = query.AfterSequence
		}
		if len(rows) < query.Limit {
			break
		}
	}
	return policyErr
}

func (o *Observer) recoverBudgetDone(
	ctx context.Context, recovery dailyBudgetReceiptRecovery, sess *store.DailyBudgetReconcileSession,
	snapshot observedSession, row store.SessionEvent,
) error {
	event, err := transcript.UnmarshalAgentEvent(row.Content)
	if err != nil {
		return fmt.Errorf("%w: decode budget recovery event %q/%q: %w",
			errDailyBudgetLedgerUnreadable, sess.ID, row.TurnID, err)
	}
	if !hasBudgetUsageEvidence(event) {
		return nil
	}
	if event.TurnID != row.TurnID || event.TurnID == "" {
		return fmt.Errorf("observe: budget recovery turn mismatch for %q", sess.ID)
	}
	receipt, err := recovery.DailyBudgetReceiptState(ctx, sess.ID, event.TurnID, sess.ProfileID)
	if err != nil {
		return fmt.Errorf("observe: check recovered receipt %q/%q: %w", sess.ID, event.TurnID, err)
	}
	if receipt.Present && !receipt.OwnershipUnresolved {
		return nil
	}
	usageAt := row.Timestamp
	if event.Usage != nil && !event.Usage.Timestamp.IsZero() {
		usageAt = event.Usage.Timestamp
	}
	day := receipt.Day
	if !receipt.Present {
		day = store.LocalDay(usageAt)
	}
	uncertainToday := sess.UnresolvedOwner && !usageAt.Before(sess.InstanceCreatedAt) && day == store.LocalDay(o.now())
	if !receipt.Present && (!sess.UnresolvedOwner || event.HasCanonicalUsageProjection() || uncertainToday) {
		if err := o.backfillBudgetDone(ctx, recovery, sess, snapshot, row, event); err != nil {
			return err
		}
	}
	if uncertainToday {
		return fmt.Errorf(
			"%w: unresolved usage ownership for session %q",
			agentinstance.ErrBudgetHistoryUncertain,
			sess.ID,
		)
	}
	return nil
}

func (o *Observer) backfillBudgetDone(
	ctx context.Context, recovery dailyBudgetReceiptRecovery, sess *store.DailyBudgetReconcileSession,
	snapshot observedSession, row store.SessionEvent, event acp.AgentEvent,
) error {
	eventSnapshot := usageSnapshotForDone(snapshot, event)
	if event.PromptRuntimeSnapshot() == nil {
		// The provider's current auth mode cannot establish how an earlier turn was metered.
		eventSnapshot.suppressCostEstimate = true
	}
	if row.AgentName != "" {
		eventSnapshot.agentName = row.AgentName
	}
	stats, daily := o.observedUsageUpdates(ctx, sess.ID, eventSnapshot, event, row.Timestamp)
	if err := recovery.BackfillDailyBudgetReceipt(ctx, stats, daily, event.HasCanonicalUsageProjection()); err != nil {
		return fmt.Errorf("observe: recover daily budget receipt %q/%q: %w", sess.ID, row.TurnID, err)
	}
	return nil
}
