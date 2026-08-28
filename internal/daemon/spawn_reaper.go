package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	defaultSpawnReaperInterval = 5 * time.Second

	spawnReapReasonTTLExpired    = "ttl_expired"
	spawnReapReasonParentStopped = "parent_stopped"
	spawnReapReasonOrphaned      = "orphaned"
)

type spawnLeaseReleaser interface {
	ReleaseSessionRunLeases(
		context.Context,
		taskpkg.SessionLeaseRelease,
		taskpkg.ActorContext,
	) ([]taskpkg.SessionLeaseReleaseResult, error)
}

// spawnReaperTTLStopper lets the concrete session manager classify prompt
// state under its lifecycle lock instead of relying on a stale sweep snapshot.
type spawnReaperTTLStopper interface {
	StopWithSpawnTTL(context.Context, string, string) error
}

var _ spawnReaperTTLStopper = (*session.Manager)(nil)

type spawnReaper struct {
	ctx           context.Context
	cancel        context.CancelFunc
	sessions      SessionManager
	tasks         spawnLeaseReleaser
	hooks         session.SpawnHooks
	logger        *slog.Logger
	now           func() time.Time
	interval      time.Duration
	callLifecycle spawnReaperCallLifecycle
	wg            sync.WaitGroup
}

type spawnReaperCallLifecycle interface {
	FenceReapSession(context.Context, callspkg.ReapedSession) (bool, error)
	FailRecipientDeliveries(context.Context, string, string) error
	FinalizeReapedSession(context.Context, callspkg.ReapedSession) error
}

type spawnReaperReport struct {
	Checked        int
	Reaped         int
	ReleasedLeases int
	TTLExpired     int
	ParentStopped  int
	Orphaned       int
}

type spawnReapCandidate struct {
	child  *session.Info
	parent *session.Info
	reason string
}

func newSpawnReaper(
	ctx context.Context,
	sessions SessionManager,
	tasks spawnLeaseReleaser,
	hooks session.SpawnHooks,
	logger *slog.Logger,
	now func() time.Time,
	interval time.Duration,
	callLifecycle spawnReaperCallLifecycle,
) (*spawnReaper, error) {
	if ctx == nil {
		return nil, errors.New("daemon: spawn reaper context is required")
	}
	if sessions == nil {
		return nil, errors.New("daemon: spawn reaper requires session manager")
	}
	if tasks == nil {
		return nil, errors.New("daemon: spawn reaper requires task lease releaser")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if interval <= 0 {
		interval = defaultSpawnReaperInterval
	}
	reaperCtx, cancel := context.WithCancel(ctx)
	reaper := &spawnReaper{
		ctx:      reaperCtx,
		cancel:   cancel,
		sessions: sessions,
		tasks:    tasks,
		hooks:    hooks,
		logger:   logger,
		now:      now,
		interval: interval,
	}
	reaper.callLifecycle = callLifecycle
	return reaper, nil
}

func (r *spawnReaper) start() {
	if r == nil {
		return
	}
	r.wg.Go(func() {
		r.loop()
	})
}

func (r *spawnReaper) loop() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			report, err := r.Sweep(r.ctx)
			if err != nil && r.logger != nil && !errors.Is(err, context.Canceled) {
				r.logger.Warn("daemon: spawn reaper sweep failed", "error", err)
			}
			if report.Reaped > 0 && r.logger != nil {
				r.logger.Info(
					"daemon: spawn reaper sweep complete",
					"reaped", report.Reaped,
					"released_leases", report.ReleasedLeases,
					"ttl_expired", report.TTLExpired,
					"parent_stopped", report.ParentStopped,
					"orphaned", report.Orphaned,
				)
			}
		}
	}
}

func (r *spawnReaper) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: spawn reaper shutdown context is required")
	}
	if r.cancel != nil {
		r.cancel()
	}
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown spawn reaper: %w", ctx.Err())
	}
}

func (r *spawnReaper) Sweep(ctx context.Context) (spawnReaperReport, error) {
	if r == nil {
		return spawnReaperReport{}, nil
	}
	if ctx == nil {
		return spawnReaperReport{}, errors.New("daemon: spawn reaper sweep context is required")
	}
	infos, err := r.sessions.ListAll(ctx)
	if err != nil {
		return spawnReaperReport{}, fmt.Errorf("daemon: list sessions for spawn reaper: %w", err)
	}
	parents := make(map[string]*session.Info, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		parents[strings.TrimSpace(info.ID)] = info
	}

	report := spawnReaperReport{}
	var errs []error
	for _, info := range infos {
		candidate, ok := r.reapCandidate(info, parents)
		if !ok {
			continue
		}
		if r.callLifecycle != nil {
			allowed, protectErr := r.callLifecycle.FenceReapSession(ctx, reapedSessionFor(candidate))
			if protectErr != nil {
				errs = append(
					errs,
					fmt.Errorf("daemon: inspect call reaper protection for %q: %w", info.ID, protectErr),
				)
				continue
			}
			if !allowed {
				continue
			}
		}
		report.Checked++
		if err := r.reap(ctx, candidate, &report); err != nil {
			errs = append(errs, err)
		}
	}
	return report, errors.Join(errs...)
}

func (r *spawnReaper) reapCandidate(
	info *session.Info,
	parents map[string]*session.Info,
) (spawnReapCandidate, bool) {
	if info == nil || (!spawnReaperLiveState(info.State) && info.ParkedAt == nil) {
		return spawnReapCandidate{}, false
	}
	switch info.Type {
	case session.SessionTypeSpawned:
		return r.reapSpawnedCandidate(info, parents)
	case session.SessionTypeSystem:
		return r.reapTTLSystemCandidate(info)
	default:
		return spawnReapCandidate{}, false
	}
}

// reapTTLSystemCandidate reaps a parent-less, TTL-bearing system session (a capability-matched
// starvation worker) once its deadline passes. System sessions without a TTL lineage — ordinary
// task-role and task sessions — carry no lineage and are never reaped here.
func (r *spawnReaper) reapTTLSystemCandidate(info *session.Info) (spawnReapCandidate, bool) {
	if info.Lineage == nil {
		return spawnReapCandidate{}, false
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if strings.TrimSpace(lineage.ParentSessionID) != "" || lineage.TTLExpiresAt == nil {
		return spawnReapCandidate{}, false
	}
	if lineage.TTLExpiresAt.After(r.now().UTC()) {
		return spawnReapCandidate{}, false
	}
	normalized := *info
	normalized.Lineage = lineage
	return spawnReapCandidate{child: &normalized, reason: spawnReapReasonTTLExpired}, true
}

func (r *spawnReaper) reapSpawnedCandidate(
	info *session.Info,
	parents map[string]*session.Info,
) (spawnReapCandidate, bool) {
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if lineage.ParentSessionID == "" {
		normalized := *info
		normalized.Lineage = lineage
		return spawnReapCandidate{child: &normalized, reason: spawnReapReasonOrphaned}, true
	}
	normalized := *info
	normalized.Lineage = lineage
	info = &normalized

	now := r.now().UTC()
	if info.ParkedAt != nil {
		if info.IdleExpiresAt == nil || info.IdleExpiresAt.After(now) {
			return spawnReapCandidate{}, false
		}
		return spawnReapCandidate{
			child: info, parent: parents[lineage.ParentSessionID], reason: spawnReapReasonTTLExpired,
		}, true
	}
	if lineage.TTLExpiresAt != nil && !lineage.TTLExpiresAt.After(now) {
		return spawnReapCandidate{
			child:  info,
			parent: parents[lineage.ParentSessionID],
			reason: spawnReapReasonTTLExpired,
		}, true
	}
	parent := parents[lineage.ParentSessionID]
	if parent == nil {
		return spawnReapCandidate{child: info, reason: spawnReapReasonOrphaned}, true
	}
	if lineage.AutoStopOnParent && !spawnReaperLiveState(parent.State) {
		return spawnReapCandidate{
			child:  info,
			parent: parent,
			reason: spawnReapReasonParentStopped,
		}, true
	}
	return spawnReapCandidate{}, false
}

func (r *spawnReaper) reap(
	ctx context.Context,
	candidate spawnReapCandidate,
	report *spawnReaperReport,
) error {
	child := candidate.child
	if child == nil {
		return nil
	}
	reason := strings.TrimSpace(candidate.reason)
	if reason == "" {
		reason = spawnReapReasonOrphaned
	}

	r.dispatchReasonHook(ctx, candidate)
	released, releaseErr := r.releaseChildLeases(ctx, child, reason)
	if report != nil {
		report.ReleasedLeases += released
	}

	stopErr := r.stopChild(ctx, child, reason)
	var deliveryErr error
	var finalizeErr error
	if stopErr == nil && r.callLifecycle != nil {
		deliveryErr = r.callLifecycle.FailRecipientDeliveries(ctx, child.ID, reason)
		if deliveryErr == nil {
			finalizeErr = r.callLifecycle.FinalizeReapedSession(ctx, reapedSessionFor(candidate))
		}
	}
	if stopErr == nil && deliveryErr == nil && finalizeErr == nil && report != nil {
		report.Reaped++
		switch reason {
		case spawnReapReasonTTLExpired:
			report.TTLExpired++
		case spawnReapReasonParentStopped:
			report.ParentStopped++
		case spawnReapReasonOrphaned:
			report.Orphaned++
		}
	}
	r.dispatchReapedHook(ctx, candidate, errors.Join(releaseErr, stopErr, finalizeErr, deliveryErr))

	var errs []error
	if releaseErr != nil {
		errs = append(errs, fmt.Errorf("daemon: release child leases for %q: %w", child.ID, releaseErr))
	}
	if stopErr != nil {
		errs = append(errs, fmt.Errorf("daemon: stop spawned child %q: %w", child.ID, stopErr))
	}
	if deliveryErr != nil {
		errs = append(errs, fmt.Errorf("daemon: fail deliveries for reaped child %q: %w", child.ID, deliveryErr))
	}
	if finalizeErr != nil {
		errs = append(errs, fmt.Errorf("daemon: finalize reaped child %q: %w", child.ID, finalizeErr))
	}
	return errors.Join(errs...)
}

func reapedSessionFor(candidate spawnReapCandidate) callspkg.ReapedSession {
	child := candidate.child
	if child == nil {
		return callspkg.ReapedSession{}
	}
	lineage := store.NormalizeSessionLineage(child.ID, child.Lineage)
	return callspkg.ReapedSession{
		ProfileID:       child.ProfileID,
		Scope:           callspkg.Scope(child.Scope),
		WorkspaceID:     child.WorkspaceID,
		SessionID:       child.ID,
		ParentSessionID: lineage.ParentSessionID,
		RootSessionID:   lineage.RootSessionID,
		AgentName:       child.AgentName,
		Reason:          strings.TrimSpace(candidate.reason),
	}
}

func (r *spawnReaper) releaseChildLeases(ctx context.Context, child *session.Info, reason string) (int, error) {
	actor, err := taskpkg.DeriveDaemonActorContext("spawn-reaper", "daemon.spawn_reaper")
	if err != nil {
		return 0, err
	}
	results, err := r.tasks.ReleaseSessionRunLeases(ctx, taskpkg.SessionLeaseRelease{
		SessionID: child.ID,
		Reason:    reason,
		Now:       r.now().UTC(),
	}, actor)
	if err != nil {
		return 0, err
	}
	return len(results), nil
}

func (r *spawnReaper) stopChild(
	ctx context.Context,
	child *session.Info,
	reason string,
) error {
	if child.ParkedAt != nil && child.State == session.StateStopped {
		return nil
	}
	if reason == spawnReapReasonTTLExpired {
		if stopper, ok := r.sessions.(spawnReaperTTLStopper); ok {
			err := stopper.StopWithSpawnTTL(ctx, child.ID, "spawn_reaper:"+reason)
			if errors.Is(err, session.ErrSessionNotFound) {
				return nil
			}
			return err
		}
	}
	cause := session.CauseUserRequested
	if reason == spawnReapReasonTTLExpired {
		// Adapters that do not provide the atomic operation cannot prove that a
		// prompt stayed settled through the stop transition. Preserve the
		// historical timeout classification rather than risk a false clean stop.
		cause = session.CauseTimeout
	}
	err := r.sessions.StopWithCause(ctx, child.ID, cause, "spawn_reaper:"+reason)
	if errors.Is(err, session.ErrSessionNotFound) {
		return nil
	}
	return err
}
