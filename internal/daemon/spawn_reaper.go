package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

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
	ctx      context.Context
	cancel   context.CancelFunc
	sessions SessionManager
	tasks    spawnLeaseReleaser
	hooks    session.SpawnHooks
	logger   *slog.Logger
	now      func() time.Time
	interval time.Duration
	wg       sync.WaitGroup
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
	return &spawnReaper{
		ctx:      reaperCtx,
		cancel:   cancel,
		sessions: sessions,
		tasks:    tasks,
		hooks:    hooks,
		logger:   logger,
		now:      now,
		interval: interval,
	}, nil
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
	var infos []*session.Info
	if inventory, ok := r.sessions.(spawnReaperLiveInventory); ok {
		var err error
		infos, err = inventory.ListLive(ctx)
		if err != nil {
			return spawnReaperReport{}, fmt.Errorf("daemon: list live sessions for spawn reaper: %w", err)
		}
	} else {
		infos = r.sessions.List()
	}
	parents, err := r.readParents(ctx, infos)
	if err != nil {
		return spawnReaperReport{}, err
	}

	report := spawnReaperReport{}
	var errs []error
	for _, info := range infos {
		candidate, ok := r.reapCandidate(info, parents)
		if !ok {
			continue
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
	if info == nil || !spawnReaperLiveState(info.State) {
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
	info.Lineage = lineage
	return spawnReapCandidate{child: info, reason: spawnReapReasonTTLExpired}, true
}

func (r *spawnReaper) reapSpawnedCandidate(
	info *session.Info,
	parents map[string]*session.Info,
) (spawnReapCandidate, bool) {
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if lineage.ParentSessionID == "" {
		info.Lineage = lineage
		return spawnReapCandidate{child: info, reason: spawnReapReasonOrphaned}, true
	}
	info.Lineage = lineage

	now := r.now().UTC()
	if lineage.SpawnRole != store.SubagentSpawnRole && lineage.TTLExpiresAt != nil && !lineage.TTLExpiresAt.After(now) {
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
		if subagentSurvivesParentStop(lineage, parent) {
			return spawnReapCandidate{}, false
		}
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
	freshness, err := r.candidateFreshness(ctx, child, reason)
	if err != nil {
		return err
	}
	switch freshness {
	case spawnCandidateEnded:
		// The child stopped on its own since the snapshot: it needs no stop, but its
		// task-run leases still go back now rather than at lease expiry.
		released, releaseErr := r.releaseChildLeases(ctx, child, reason)
		if report != nil {
			report.ReleasedLeases += released
		}
		if releaseErr != nil {
			return fmt.Errorf("daemon: release child leases for %q: %w", child.ID, releaseErr)
		}
		return nil
	case spawnCandidateChanged:
		return nil
	case spawnCandidateCurrent:
	}

	r.dispatchReasonHook(ctx, candidate)
	released, releaseErr := r.releaseChildLeases(ctx, child, reason)
	if report != nil {
		report.ReleasedLeases += released
	}

	stopErr := r.stopChild(ctx, child, reason)
	if stopErr == nil && report != nil {
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
	r.dispatchReapedHook(ctx, candidate, errors.Join(releaseErr, stopErr))

	var errs []error
	if releaseErr != nil {
		errs = append(errs, fmt.Errorf("daemon: release child leases for %q: %w", child.ID, releaseErr))
	}
	if stopErr != nil {
		errs = append(errs, fmt.Errorf("daemon: stop spawned child %q: %w", child.ID, stopErr))
	}
	return errors.Join(errs...)
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

type spawnReaperLiveInventory interface {
	ListLive(context.Context) ([]*session.Info, error)
}

var _ spawnReaperLiveInventory = (*session.Manager)(nil)

func (r *spawnReaper) readParents(ctx context.Context, infos []*session.Info) (map[string]*session.Info, error) {
	parents := make(map[string]*session.Info, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		parents[strings.TrimSpace(info.ID)] = info
	}

	for _, info := range infos {
		if info == nil || !spawnReaperLiveState(info.State) || info.Type != session.SessionTypeSpawned {
			continue
		}
		lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
		parentID := lineage.ParentSessionID
		if parentID == "" {
			continue
		}
		if _, found := parents[parentID]; found {
			continue
		}
		parent, err := r.sessions.Status(ctx, parentID)
		if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
			return nil, fmt.Errorf("daemon: read spawn parent %q: %w", parentID, err)
		}
		parents[parentID] = parent
	}

	return parents, nil
}
