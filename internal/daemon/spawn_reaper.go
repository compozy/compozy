package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
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

func (r *spawnReaper) stopChild(ctx context.Context, child *session.Info, reason string) error {
	cause := session.CauseUserRequested
	if reason == spawnReapReasonTTLExpired {
		cause = session.CauseTimeout
	}
	err := r.sessions.StopWithCause(ctx, child.ID, cause, "spawn_reaper:"+reason)
	if errors.Is(err, session.ErrSessionNotFound) {
		return nil
	}
	return err
}

func (r *spawnReaper) dispatchReasonHook(ctx context.Context, candidate spawnReapCandidate) {
	payload := r.spawnLifecyclePayload(candidate, nil)
	var err error
	switch candidate.reason {
	case spawnReapReasonTTLExpired:
		payload.Event = hookspkg.HookSpawnTTLExpired
		_, err = r.hooksOrNoop().DispatchSpawnTTLExpired(ctx, payload)
	case spawnReapReasonParentStopped:
		payload.Event = hookspkg.HookSpawnParentStopped
		_, err = r.hooksOrNoop().DispatchSpawnParentStopped(ctx, payload)
	}
	if err != nil && r.logger != nil {
		r.logger.Warn("daemon: spawn lifecycle hook failed", "event", payload.Event, "error", err)
	}
}

func (r *spawnReaper) dispatchReapedHook(ctx context.Context, candidate spawnReapCandidate, reapErr error) {
	payload := r.spawnLifecyclePayload(candidate, reapErr)
	payload.Event = hookspkg.HookSpawnReaped
	if _, err := r.hooksOrNoop().DispatchSpawnReaped(ctx, payload); err != nil &&
		r.logger != nil {
		r.logger.Warn("daemon: spawn reaped hook failed", "error", err)
	}
}

func (r *spawnReaper) spawnLifecyclePayload(
	candidate spawnReapCandidate,
	reapErr error,
) hookspkg.SpawnLifecyclePayload {
	child := candidate.child
	lineage := store.NormalizeSessionLineage("", nil)
	if child != nil {
		lineage = store.NormalizeSessionLineage(child.ID, child.Lineage)
	}
	payload := hookspkg.SpawnLifecyclePayload{
		PayloadBase: hookspkg.PayloadBase{Timestamp: r.now().UTC()},
		SpawnContext: hookspkg.SpawnContext{
			ParentSessionID:  lineage.ParentSessionID,
			RootSessionID:    lineage.RootSessionID,
			SpawnDepth:       lineage.SpawnDepth,
			SpawnRole:        lineage.SpawnRole,
			TTLSeconds:       lineage.SpawnBudget.TTLSeconds,
			AutoStopOnParent: lineage.AutoStopOnParent,
		},
		ChildPermissions: spawnReaperPermissionSet(lineage.PermissionPolicy),
		StopReason:       candidate.reason,
		ReapReason:       candidate.reason,
	}
	if child != nil {
		payload.ChildSessionID = child.ID
		payload.AgentName = child.AgentName
		payload.WorkspaceID = child.WorkspaceID
		payload.Workspace = child.Workspace
		payload.ResolvedNetworkParticipation = participation.CloneSpec(child.NetworkParticipation)
		payload.SoulSnapshotID = child.SoulSnapshotID
		payload.SoulDigest = child.SoulDigest
		payload.ParentSoulDigest = child.ParentSoulDigest
	}
	if candidate.parent != nil {
		if payload.ResolvedNetworkParticipation == nil {
			payload.ResolvedNetworkParticipation = participation.CloneSpec(
				candidate.parent.NetworkParticipation,
			)
		}
		if strings.TrimSpace(payload.ParentSoulDigest) == "" {
			payload.ParentSoulDigest = candidate.parent.SoulDigest
		}
		if candidate.parent.Lineage != nil {
			payload.ParentPermissions = spawnReaperPermissionSet(candidate.parent.Lineage.PermissionPolicy)
		}
	}
	if reapErr != nil {
		payload.Error = reapErr.Error()
	}
	return payload
}

func (r *spawnReaper) hooksOrNoop() session.SpawnHooks {
	if r == nil || r.hooks == nil {
		return spawnReaperNoopHooks{}
	}
	return r.hooks
}

func spawnReaperPermissionSet(policy store.SessionPermissionPolicy) *hookspkg.PermissionSet {
	normalized := store.NormalizeSessionPermissionPolicy(policy)
	return &hookspkg.PermissionSet{
		Tools:           append([]string(nil), normalized.Tools...),
		Skills:          append([]string(nil), normalized.Skills...),
		MCPServers:      append([]string(nil), normalized.MCPServers...),
		WorkspacePaths:  append([]string(nil), normalized.WorkspacePaths...),
		NetworkChannels: append([]string(nil), normalized.NetworkChannels...),
		SandboxProfiles: append([]string(nil), normalized.SandboxProfiles...),
	}
}

func spawnReaperLiveState(state session.State) bool {
	switch state {
	case session.StateStarting, session.StateActive, session.StateStopping:
		return true
	default:
		return false
	}
}

type spawnReaperNoopHooks struct{}

func (spawnReaperNoopHooks) DispatchSpawnPreCreate(
	_ context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnCreated(
	_ context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnParentStopped(
	_ context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnTTLExpired(
	_ context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnReaped(
	_ context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return payload, nil
}
