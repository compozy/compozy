package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestSpawnReaperSweepClassifiesReasonsReleasesLeasesAndStopsChildren(t *testing.T) {
	t.Parallel()
	t.Run("Should classify reasons, release leases, and stop children", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		expired := now.Add(-time.Minute)
		future := now.Add(time.Hour)
		sequence := make([]string, 0)
		sessions := &fakeSessionManager{
			infos: []*session.Info{
				rootReaperInfo("parent-live", session.StateActive),
				rootReaperInfo("parent-stopped", session.StateStopped),
				spawnedReaperInfo("child-ttl", "parent-live", expired, true),
				spawnedReaperInfo("child-parent", "parent-stopped", future, true),
				spawnedReaperInfo("child-orphan", "missing-parent", future, true),
				rootReaperInfo("manual", session.StateActive),
				provenanceReaperInfo("provenance-child", "parent-stopped"),
				provenanceReaperInfo("provenance-orphan", "missing-parent"),
			},
			stopWithCauseErr: func(id string, _ session.StopCause, _ string) error {
				sequence = append(sequence, "stop:"+id)
				return nil
			},
		}
		leases := &fakeSpawnLeaseReleaser{
			resultCountBySession: map[string]int{
				"child-ttl":    2,
				"child-parent": 1,
			},
			sequence: &sequence,
		}
		hooks := &recordingSpawnHooks{}

		reaper, err := newSpawnReaper(
			context.Background(),
			sessions,
			leases,
			hooks,
			discardLogger(),
			func() time.Time { return now },
			time.Hour,
			nil,
		)
		if err != nil {
			t.Fatalf("newSpawnReaper() error = %v", err)
		}

		report, err := reaper.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep() error = %v", err)
		}
		if report.Checked != 3 ||
			report.Reaped != 3 ||
			report.ReleasedLeases != 3 ||
			report.TTLExpired != 1 ||
			report.ParentStopped != 1 ||
			report.Orphaned != 1 {
			t.Fatalf("report = %#v, want three classified reaps and three released leases", report)
		}
		if len(sessions.stopWithCauseCalls) != 3 {
			t.Fatalf(
				"stop calls = %#v, want only spawned children reaped and provenance user sessions left alone",
				sessions.stopWithCauseCalls,
			)
		}
		assertStopWithCause(
			t,
			sessions.stopWithCauseCalls,
			"child-ttl",
			session.CauseTimeout,
			"spawn_reaper:ttl_expired",
		)
		assertStopWithCause(
			t,
			sessions.stopWithCauseCalls,
			"child-parent",
			session.CauseUserRequested,
			"spawn_reaper:parent_stopped",
		)
		assertStopWithCause(
			t,
			sessions.stopWithCauseCalls,
			"child-orphan",
			session.CauseUserRequested,
			"spawn_reaper:orphaned",
		)
		if got, want := sequence, []string{
			"release:child-ttl",
			"stop:child-ttl",
			"release:child-parent",
			"stop:child-parent",
			"release:child-orphan",
			"stop:child-orphan",
		}; !equalStrings(got, want) {
			t.Fatalf("sequence = %#v, want release before stop for each child %#v", got, want)
		}
		assertReleaseReason(t, leases.releases, "child-ttl", spawnReapReasonTTLExpired)
		assertReleaseReason(t, leases.releases, "child-parent", spawnReapReasonParentStopped)
		assertReleaseReason(t, leases.releases, "child-orphan", spawnReapReasonOrphaned)
		if len(hooks.ttlExpired) != 1 || hooks.ttlExpired[0].ChildSessionID != "child-ttl" {
			t.Fatalf("ttl hooks = %#v, want child-ttl", hooks.ttlExpired)
		}
		if len(hooks.parentStopped) != 1 || hooks.parentStopped[0].ChildSessionID != "child-parent" {
			t.Fatalf("parent stopped hooks = %#v, want child-parent", hooks.parentStopped)
		}
		if len(hooks.reaped) != 3 {
			t.Fatalf("reaped hooks = %#v, want three", hooks.reaped)
		}
	})
}

func TestSpawnReaperReapsTTLExpiredStarvationWorkers(t *testing.T) {
	t.Run("Should reap only expired starvation workers", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
		sessions := &fakeSessionManager{
			infos: []*session.Info{
				starvationReaperInfo("worker-expired", now.Add(-time.Minute)),
				starvationReaperInfo("worker-live", now.Add(time.Hour)),
				roleReaperInfo("role-session"),
			},
			stopWithCauseErr: func(string, session.StopCause, string) error { return nil },
		}
		leases := &fakeSpawnLeaseReleaser{resultCountBySession: map[string]int{"worker-expired": 1}}
		hooks := &recordingSpawnHooks{}

		reaper, err := newSpawnReaper(
			context.Background(),
			sessions,
			leases,
			hooks,
			discardLogger(),
			func() time.Time { return now },
			time.Hour,
			nil,
		)
		if err != nil {
			t.Fatalf("newSpawnReaper() error = %v", err)
		}

		report, err := reaper.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep() error = %v", err)
		}
		if report.Reaped != 1 || report.TTLExpired != 1 {
			t.Fatalf("report = %#v, want exactly the expired starvation worker reaped", report)
		}
		if len(sessions.stopWithCauseCalls) != 1 {
			t.Fatalf("stop calls = %#v, want only worker-expired (live worker + lineage-less role session untouched)",
				sessions.stopWithCauseCalls)
		}
		assertStopWithCause(
			t,
			sessions.stopWithCauseCalls,
			"worker-expired",
			session.CauseTimeout,
			"spawn_reaper:ttl_expired",
		)
	})
}

func TestSpawnReaperUsesParkedIdleClockAndCallProtection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Minute), now.Add(time.Hour)
	parent := rootReaperInfo("parent", session.StateActive)
	expired := spawnedReaperInfo("parked-expired", "parent", now.Add(-time.Hour), true)
	expired.State, expired.ParkedAt, expired.IdleExpiresAt = session.StateStopped, &past, &past

	t.Run("Should reap only an expired parked child after protecting open calls", func(t *testing.T) {
		t.Parallel()

		inWindow := spawnedReaperInfo("parked-in-window", "parent", now.Add(-time.Hour), true)
		inWindow.State, inWindow.ParkedAt, inWindow.IdleExpiresAt = session.StateStopped, &past, &future
		openCall := spawnedReaperInfo("open-call", "parent", now.Add(-time.Minute), true)
		lifecycle := &fakeSpawnReaperCallLifecycle{protected: map[string]bool{"open-call": true}}
		reaper, err := newSpawnReaper(
			context.Background(),
			&fakeSessionManager{infos: []*session.Info{parent, expired, inWindow, openCall}},
			&fakeSpawnLeaseReleaser{}, &recordingSpawnHooks{}, discardLogger(),
			func() time.Time { return now }, time.Hour, lifecycle,
		)
		if err != nil {
			t.Fatalf("newSpawnReaper() error = %v", err)
		}
		report, err := reaper.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep() error = %v", err)
		}
		if report.Reaped != 1 || report.TTLExpired != 1 {
			t.Fatalf("Sweep() report = %#v, want only parked-expired reaped", report)
		}
		if !equalStrings(lifecycle.finalized, []string{"parked-expired:ttl_expired"}) ||
			!equalStrings(lifecycle.failedDeliveries, []string{"parked-expired:ttl_expired"}) {
			t.Fatalf("call lifecycle = finalized %#v failed %#v", lifecycle.finalized, lifecycle.failedDeliveries)
		}
		if !equalStrings(lifecycle.sequence, []string{
			"fail-deliveries:parked-expired:ttl_expired", "finalize:parked-expired:ttl_expired",
		}) {
			t.Fatalf("call lifecycle sequence = %#v, want deliveries failed before finalization", lifecycle.sequence)
		}
	})

	t.Run("Should preserve a delivery failure and leave the child unreaped", func(t *testing.T) {
		t.Parallel()

		lifecycle := &fakeSpawnReaperCallLifecycle{failDeliveryErr: context.DeadlineExceeded}
		reaper, err := newSpawnReaper(
			context.Background(), &fakeSessionManager{infos: []*session.Info{parent, expired}},
			&fakeSpawnLeaseReleaser{}, &recordingSpawnHooks{}, discardLogger(),
			func() time.Time { return now }, time.Hour, lifecycle,
		)
		if err != nil {
			t.Fatalf("newSpawnReaper() error = %v", err)
		}
		report, err := reaper.Sweep(context.Background())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Sweep() error = %v, want context.DeadlineExceeded", err)
		}
		if report.Reaped != 0 || len(lifecycle.finalized) != 0 {
			t.Fatalf("Sweep() = report %#v finalized %#v, want no completed reap", report, lifecycle.finalized)
		}
	})
}

func TestSpawnReaperTTLClassification(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		prompting bool
		wantCause session.StopCause
	}{
		{
			name:      "Should reap a settled child as completed",
			wantCause: session.CauseCompleted,
		},
		{
			name:      "Should keep an in-flight prompt as a timeout",
			prompting: true,
			wantCause: session.CauseTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hooks := &recordingSpawnHooks{}
			leases := &fakeSpawnLeaseReleaser{resultCountBySession: map[string]int{"child": 1}}
			sessions := &spawnReaperAtomicSessionManager{
				fakeSessionManager: &fakeSessionManager{
					infos: []*session.Info{
						rootReaperInfo("parent", session.StateActive),
						spawnedReaperInfo("child", "parent", now.Add(-time.Minute), true),
					},
				},
				prompting: map[string]bool{"child": tt.prompting},
			}
			reaper, err := newSpawnReaper(
				context.Background(),
				sessions,
				leases,
				hooks,
				discardLogger(),
				func() time.Time { return now },
				time.Hour,
				nil,
			)
			if err != nil {
				t.Fatalf("newSpawnReaper() error = %v", err)
			}

			if _, err := reaper.Sweep(context.Background()); err != nil {
				t.Fatalf("Sweep() error = %v", err)
			}
			assertStopWithCause(
				t,
				sessions.ttlStopCalls,
				"child",
				tt.wantCause,
				"spawn_reaper:ttl_expired",
			)
			if got := len(sessions.ttlStopCalls); got != 1 {
				t.Fatalf("atomic TTL stop calls = %d, want exactly one", got)
			}
			if got := len(sessions.stopWithCauseCalls); got != 0 {
				t.Fatalf("fallback stop calls = %d, want zero when atomic TTL stop is available", got)
			}
			if got := len(leases.releases); got != 1 {
				t.Fatalf("lease releases = %d, want exactly one", got)
			}
			assertReleaseReason(t, leases.releases, "child", spawnReapReasonTTLExpired)
			if len(hooks.ttlExpired) != 1 || len(hooks.reaped) != 1 {
				t.Fatalf("lifecycle hooks = ttl=%d reaped=%d, want one each", len(hooks.ttlExpired), len(hooks.reaped))
			}
		})
	}
}

func TestSpawnReaperTTLWithoutAtomicStopperUsesTimeoutFallback(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve timeout classification without atomic stop", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
		sessions := &fakeSessionManager{infos: []*session.Info{
			rootReaperInfo("parent", session.StateActive),
			spawnedReaperInfo("child", "parent", now.Add(-time.Minute), true),
		}}
		reaper, err := newSpawnReaper(
			context.Background(),
			sessions,
			&fakeSpawnLeaseReleaser{},
			&recordingSpawnHooks{},
			discardLogger(),
			func() time.Time { return now },
			time.Hour,
			nil,
		)
		if err != nil {
			t.Fatalf("newSpawnReaper() error = %v", err)
		}
		if _, err := reaper.Sweep(context.Background()); err != nil {
			t.Fatalf("Sweep() error = %v", err)
		}
		assertStopWithCause(
			t,
			sessions.stopWithCauseCalls,
			"child",
			session.CauseTimeout,
			"spawn_reaper:ttl_expired",
		)
	})
}

type spawnReaperAtomicSessionManager struct {
	*fakeSessionManager
	prompting    map[string]bool
	ttlStopCalls []fakeStopWithCauseCall
}

type fakeSpawnReaperCallLifecycle struct {
	protected        map[string]bool
	finalized        []string
	failedDeliveries []string
	sequence         []string
	failDeliveryErr  error
}

func (f *fakeSpawnReaperCallLifecycle) FenceReapSession(_ context.Context, sessionID string) (bool, error) {
	return !f.protected[sessionID], nil
}

func (f *fakeSpawnReaperCallLifecycle) FailRecipientDeliveries(
	_ context.Context,
	sessionID string,
	reason string,
) error {
	f.failedDeliveries = append(f.failedDeliveries, sessionID+":"+reason)
	f.sequence = append(f.sequence, "fail-deliveries:"+sessionID+":"+reason)
	return f.failDeliveryErr
}

func (f *fakeSpawnReaperCallLifecycle) FinalizeReapedSession(
	_ context.Context,
	sessionID string,
	reason string,
) error {
	f.finalized = append(f.finalized, sessionID+":"+reason)
	f.sequence = append(f.sequence, "finalize:"+sessionID+":"+reason)
	return nil
}

func (m *spawnReaperAtomicSessionManager) StopWithSpawnTTL(
	_ context.Context,
	id string,
	detail string,
) error {
	cause := session.CauseCompleted
	if m.prompting[id] {
		cause = session.CauseTimeout
	}
	m.ttlStopCalls = append(m.ttlStopCalls, fakeStopWithCauseCall{
		id: id, cause: cause, detail: detail,
	})
	return nil
}

func starvationReaperInfo(id string, ttl time.Time) *session.Info {
	return &session.Info{
		ID:          id,
		AgentName:   "coder",
		WorkspaceID: "ws-1",
		Workspace:   "/repo",
		Type:        session.SessionTypeSystem,
		State:       session.StateActive,
		Lineage: &store.SessionLineage{
			SpawnRole:    session.DefaultSpawnRole,
			TTLExpiresAt: &ttl,
			SpawnBudget:  store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 1, TTLSeconds: 900},
		},
	}
}

func roleReaperInfo(id string) *session.Info {
	return &session.Info{
		ID:          id,
		AgentName:   "coder",
		WorkspaceID: "ws-1",
		Workspace:   "/repo",
		Type:        session.SessionTypeSystem,
		State:       session.StateActive,
	}
}

type fakeSpawnLeaseReleaser struct {
	resultCountBySession map[string]int
	releases             []taskpkg.SessionLeaseRelease
	sequence             *[]string
}

func (f *fakeSpawnLeaseReleaser) ReleaseSessionRunLeases(
	_ context.Context,
	release taskpkg.SessionLeaseRelease,
	_ taskpkg.ActorContext,
) ([]taskpkg.SessionLeaseReleaseResult, error) {
	f.releases = append(f.releases, release)
	if f.sequence != nil {
		*f.sequence = append(*f.sequence, "release:"+release.SessionID)
	}
	count := f.resultCountBySession[release.SessionID]
	results := make([]taskpkg.SessionLeaseReleaseResult, 0, count)
	for range count {
		results = append(results, taskpkg.SessionLeaseReleaseResult{
			Run: taskpkg.Run{
				ID:        release.SessionID + "-run",
				SessionID: release.SessionID,
			},
			PreviousRunStatus: taskpkg.TaskRunStatusRunning,
			PreviousSessionID: release.SessionID,
			Reason:            release.Reason,
		})
	}
	return results, nil
}

type recordingSpawnHooks struct {
	ttlExpired    []hookspkg.SpawnTTLExpiredPayload
	parentStopped []hookspkg.SpawnParentStoppedPayload
	reaped        []hookspkg.SpawnReapedPayload
}

func (h *recordingSpawnHooks) DispatchSpawnPreCreate(
	_ context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	return payload, nil
}

func (h *recordingSpawnHooks) DispatchSpawnCreated(
	_ context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return payload, nil
}

func (h *recordingSpawnHooks) DispatchSpawnParentStopped(
	_ context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	h.parentStopped = append(h.parentStopped, payload)
	return payload, nil
}

func (h *recordingSpawnHooks) DispatchSpawnTTLExpired(
	_ context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	h.ttlExpired = append(h.ttlExpired, payload)
	return payload, nil
}

func (h *recordingSpawnHooks) DispatchSpawnReaped(
	_ context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	h.reaped = append(h.reaped, payload)
	return payload, nil
}

func rootReaperInfo(id string, state session.State) *session.Info {
	return &session.Info{
		ID:          id,
		AgentName:   "coder",
		WorkspaceID: "ws-1",
		Workspace:   "/repo",
		Type:        session.SessionTypeUser,
		State:       state,
		Lineage: &store.SessionLineage{
			RootSessionID: id,
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools: []string{"read"},
			},
		},
	}
}

func provenanceReaperInfo(id string, parentID string) *session.Info {
	return &session.Info{
		ID:          id,
		AgentName:   "coder",
		WorkspaceID: "ws-1",
		Workspace:   "/repo",
		Type:        session.SessionTypeSystem,
		State:       session.StateActive,
		Lineage: &store.SessionLineage{
			ParentSessionID: parentID,
			RootSessionID:   parentID,
			SpawnDepth:      1,
		},
	}
}

func spawnedReaperInfo(id string, parentID string, ttl time.Time, autoStop bool) *session.Info {
	return &session.Info{
		ID:          id,
		AgentName:   "coder",
		WorkspaceID: "ws-1",
		Workspace:   "/repo",
		Type:        session.SessionTypeSpawned,
		State:       session.StateActive,
		Lineage: &store.SessionLineage{
			ParentSessionID:  parentID,
			RootSessionID:    "parent-live",
			SpawnDepth:       1,
			SpawnRole:        session.DefaultSpawnRole,
			TTLExpiresAt:     &ttl,
			AutoStopOnParent: autoStop,
			SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 1, TTLSeconds: 3600},
			PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{"read"}},
		},
	}
}

func assertStopWithCause(
	t *testing.T,
	calls []fakeStopWithCauseCall,
	id string,
	cause session.StopCause,
	detail string,
) {
	t.Helper()

	for _, call := range calls {
		if call.id == id {
			if call.cause != cause || call.detail != detail {
				t.Fatalf("stop call for %s = %#v, want cause %v detail %q", id, call, cause, detail)
			}
			return
		}
	}
	t.Fatalf("stop calls = %#v, missing %s", calls, id)
}

func assertReleaseReason(
	t *testing.T,
	releases []taskpkg.SessionLeaseRelease,
	sessionID string,
	reason string,
) {
	t.Helper()

	for _, release := range releases {
		if release.SessionID == sessionID {
			if release.Reason != reason {
				t.Fatalf("release for %s = %#v, want reason %q", sessionID, release, reason)
			}
			return
		}
	}
	t.Fatalf("releases = %#v, missing %s", releases, sessionID)
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
