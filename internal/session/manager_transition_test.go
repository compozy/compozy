package session

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

var errRecordingCatalogMissingSession = errors.New("recording catalog missing session")

func TestManagerLifecycleCatalogTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should keep memory meta and catalog consistent on stopping transition", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)

		if err := h.manager.RequestStopWithCause(testutil.Context(t), session.ID, CauseUserRequested, ""); err != nil {
			t.Fatalf("RequestStopWithCause() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		if got, want := session.Info().State, StateStopping; got != want {
			t.Fatalf("memory state after request stop = %q, want %q", got, want)
		}
		stoppingMeta := readMeta(t, session.MetaPath())
		if stoppingMeta.State != string(StateStopping) {
			t.Fatalf("meta state after request stop = %q, want %q", stoppingMeta.State, StateStopping)
		}
		stoppingCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after request stop", session.ID)
		}
		if stoppingCatalog.State != string(StateStopping) {
			t.Fatalf("catalog state after request stop = %q, want %q", stoppingCatalog.State, StateStopping)
		}
	})

	t.Run("Should leave metadata as recovery source when stopping catalog update fails", func(t *testing.T) {
		t.Parallel()

		updateErr := errors.New("catalog update failed")
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		catalog.setUpdateErr(updateErr)

		err := h.manager.RequestStopWithCause(testutil.Context(t), session.ID, CauseUserRequested, "")
		if !errors.Is(err, updateErr) {
			t.Fatalf("RequestStopWithCause() error = %v, want wrapped catalog error", err)
		}
		t.Cleanup(func() {
			catalog.setUpdateErr(nil)
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		stoppingMeta := readMeta(t, session.MetaPath())
		if stoppingMeta.State != string(StateStopping) {
			t.Fatalf("meta state after catalog failure = %q, want %q", stoppingMeta.State, StateStopping)
		}
		activeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after failed update", session.ID)
		}
		if activeCatalog.State != string(StateActive) {
			t.Fatalf("catalog state after failed update = %q, want %q", activeCatalog.State, StateActive)
		}

		catalog.setUpdateErr(nil)
		_, reconcileErr := catalog.ReconcileSessions(testutil.Context(t), []store.SessionInfo{{
			ID:          session.ID,
			WorkspaceID: stoppingMeta.WorkspaceID,
			State:       stoppingMeta.State,
			UpdatedAt:   stoppingMeta.UpdatedAt,
		}})
		if reconcileErr != nil {
			t.Fatalf("ReconcileSessions() error = %v", reconcileErr)
		}
		reconciledCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after reconcile", session.ID)
		}
		if reconciledCatalog.State != string(StateStopping) {
			t.Fatalf("catalog state after reconcile = %q, want %q", reconciledCatalog.State, StateStopping)
		}
	})

	t.Run("Should keep memory meta and catalog consistent on active and stopped transitions", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)

		activeInfo := session.Info()
		if activeInfo.State != StateActive {
			t.Fatalf("memory state after create = %q, want %q", activeInfo.State, StateActive)
		}
		activeMeta := readMeta(t, session.MetaPath())
		if activeMeta.State != string(StateActive) {
			t.Fatalf("meta state after create = %q, want %q", activeMeta.State, StateActive)
		}
		activeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after create", session.ID)
		}
		if activeCatalog.State != string(StateActive) {
			t.Fatalf("catalog state after create = %q, want %q", activeCatalog.State, StateActive)
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stoppedMeta := readMeta(t, session.MetaPath())
		if stoppedMeta.State != string(StateStopped) {
			t.Fatalf("meta state after stop = %q, want %q", stoppedMeta.State, StateStopped)
		}
		stoppedCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after stop", session.ID)
		}
		if stoppedCatalog.State != string(StateStopped) {
			t.Fatalf("catalog state after stop = %q, want %q", stoppedCatalog.State, StateStopped)
		}
	})

	t.Run("Should register stopped startup failure when create fails before active registration", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		catalog.requireExistingUpdates()
		h := newHarness(t, WithSessionCatalog(catalog))
		startErr := errors.New("driver start failed")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, startErr
		}

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "failed",
			Workspace: h.workspaceID,
		})
		if err == nil {
			t.Fatal("Create() error = nil, want startup failure")
		}
		if errors.Is(err, errRecordingCatalogMissingSession) {
			t.Fatalf("Create() error = %v, want failed start to register catalog row", err)
		}

		failedMeta := readMeta(t, store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "sess-1")))
		assertStartupFailureMeta(t, failedMeta)
		failedCatalog, ok := catalog.get("sess-1")
		if !ok {
			t.Fatal("catalog missing failed create session sess-1")
		}
		assertStartupFailureCatalog(t, failedCatalog)
	})

	t.Run("Should restore catalog projection from metadata after resume start failure", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		beforeMeta := readMeta(t, session.MetaPath())
		beforeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing stopped session %q before resume", session.ID)
		}
		resumeErr := errors.New("resume failed")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, resumeErr
		}

		_, err := h.manager.Resume(testutil.Context(t), session.ID)
		if !errors.Is(err, resumeErr) {
			t.Fatalf("Resume() error = %v, want wrapped resume failure", err)
		}
		afterMeta := readMeta(t, session.MetaPath())
		afterCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after failed resume", session.ID)
		}
		assertRestoredResumeMeta(t, afterMeta, beforeMeta)
		assertRestoredResumeCatalog(t, afterCatalog, beforeCatalog)
	})

	t.Run("Should synchronize a successful attach CAS into the active session", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		attachedAt := active.Info().UpdatedAt.Add(time.Minute)
		attach, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        attachedAt,
			TTL:        time.Minute,
		})
		if err != nil {
			t.Fatalf("AttachSession() error = %v", err)
		}
		info := active.Info()
		if info.AttachedTo != attach.AttachedTo || info.AttachExpiresAt == nil ||
			!info.AttachExpiresAt.Equal(attach.AttachExpiresAt) || !info.UpdatedAt.Equal(attachedAt) {
			t.Fatalf("active info after attach = %#v, want synchronized lock %#v", info, attach)
		}
		if AttachableForInfo(info, attachedAt) {
			t.Fatal("AttachableForInfo(attached) = true, want locked")
		}
		healthByID, err := h.manager.SessionHealthForPage(testutil.Context(t), []*Info{info})
		if err != nil {
			t.Fatalf("SessionHealthForPage() error = %v", err)
		}
		if healthByID[active.ID].Attachable {
			t.Fatalf("session health after attach = %#v, want not attachable", healthByID[active.ID])
		}
	})

	t.Run("Should leave active memory unchanged when attach CAS fails", func(t *testing.T) {
		t.Parallel()

		attachErr := errors.New("attach CAS failed")
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		before := active.Info()
		catalog.setAttachErr(attachErr)
		_, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        before.UpdatedAt.Add(time.Minute),
			TTL:        time.Minute,
		})
		if !errors.Is(err, attachErr) {
			t.Fatalf("AttachSession() error = %v, want wrapped CAS failure", err)
		}
		after := active.Info()
		if after.AttachedTo != before.AttachedTo || after.AttachExpiresAt != nil ||
			!after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("active info after failed CAS = %#v, want unchanged %#v", after, before)
		}
	})
}

type recordingSessionCatalog struct {
	mu            sync.Mutex
	sessions      map[string]store.SessionInfo
	updateErr     error
	updateHook    func(store.SessionStateUpdate) error
	deleteErr     error
	attachErr     error
	strictUpdates bool
}

func newRecordingSessionCatalog() *recordingSessionCatalog {
	return &recordingSessionCatalog{sessions: make(map[string]store.SessionInfo)}
}

func (c *recordingSessionCatalog) RegisterSession(_ context.Context, session store.SessionInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[session.ID] = session
	return nil
}

func (c *recordingSessionCatalog) UpdateSessionState(_ context.Context, update store.SessionStateUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.updateHook != nil {
		if err := c.updateHook(update); err != nil {
			return err
		}
	}
	if c.updateErr != nil {
		return c.updateErr
	}
	current, ok := c.sessions[update.ID]
	if c.strictUpdates && !ok {
		return errRecordingCatalogMissingSession
	}
	current.State = update.State
	current.ACPSessionID = update.ACPSessionID
	if update.StopReasonSet {
		current.StopReason = store.StopReason("")
		if update.StopReason != nil {
			current.StopReason = store.StopReason(*update.StopReason)
		}
		current.StopDetail = update.StopDetail
	}
	if update.FailureSet {
		current.Failure = store.CloneSessionFailure(update.Failure)
	}
	current.Liveness = store.CloneSessionLivenessMeta(update.Liveness)
	current.Sandbox = cloneSessionSandboxMeta(update.Sandbox)
	current.UpdatedAt = update.UpdatedAt
	c.sessions[update.ID] = current
	return nil
}

func (c *recordingSessionCatalog) DeleteSession(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deleteErr != nil {
		return c.deleteErr
	}
	if _, ok := c.sessions[id]; !ok {
		return store.ErrSessionNotFound
	}
	delete(c.sessions, id)
	return nil
}
func (c *recordingSessionCatalog) ListSessions(
	_ context.Context,
	query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]store.SessionInfo, 0, len(c.sessions))
	for _, session := range c.sessions {
		if query.WorkspaceID != "" && session.WorkspaceID != query.WorkspaceID {
			continue
		}
		out = append(out, session)
	}
	return out, nil
}

func (c *recordingSessionCatalog) AttachSession(
	_ context.Context,
	req store.SessionAttachRequest,
) (store.SessionAttach, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attachErr != nil {
		return store.SessionAttach{}, c.attachErr
	}
	normalized := req.Normalize()
	current, ok := c.sessions[normalized.SessionID]
	if !ok {
		return store.SessionAttach{}, store.ErrSessionNotFound
	}
	if current.State != string(StateActive) {
		return store.SessionAttach{}, store.ErrSessionNotAttachable
	}
	if current.AttachedTo != "" && current.AttachExpiresAt != nil && current.AttachExpiresAt.After(normalized.Now) {
		return store.SessionAttach{}, store.ErrSessionAttachLocked
	}
	expiresAt := normalized.Now.Add(normalized.TTL).UTC()
	current.AttachedTo = normalized.AttachedTo
	current.AttachExpiresAt = &expiresAt
	current.UpdatedAt = normalized.Now
	c.sessions[normalized.SessionID] = current
	return store.SessionAttach{
		SessionID:       normalized.SessionID,
		AttachedTo:      normalized.AttachedTo,
		AttachedAt:      normalized.Now,
		AttachExpiresAt: expiresAt,
	}, nil
}

func (c *recordingSessionCatalog) ReconcileSessions(
	_ context.Context,
	sessions []store.SessionInfo,
) (store.ReconcileResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, session := range sessions {
		c.sessions[session.ID] = session
	}
	indexed := make([]string, 0, len(sessions))
	for _, session := range sessions {
		indexed = append(indexed, session.ID)
	}
	return store.ReconcileResult{Indexed: indexed}, nil
}

func (c *recordingSessionCatalog) get(id string) (store.SessionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[id]
	return session, ok
}

func (c *recordingSessionCatalog) setUpdateErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateErr = err
}

func (c *recordingSessionCatalog) setAttachErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attachErr = err
}

func (c *recordingSessionCatalog) requireExistingUpdates() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.strictUpdates = true
}

func assertStartupFailureMeta(t *testing.T, meta store.SessionMeta) {
	t.Helper()

	if meta.State != string(StateStopped) {
		t.Fatalf("failed meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.Failure == nil {
		t.Fatal("failed meta failure = nil, want startup failure")
	}
	if got, want := meta.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed meta failure kind = %q, want %q", got, want)
	}
}

func assertStartupFailureCatalog(t *testing.T, info store.SessionInfo) {
	t.Helper()

	if info.State != string(StateStopped) {
		t.Fatalf("failed catalog state = %q, want %q", info.State, StateStopped)
	}
	if info.Failure == nil {
		t.Fatal("failed catalog failure = nil, want startup failure")
	}
	if got, want := info.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed catalog failure kind = %q, want %q", got, want)
	}
}

func assertRestoredResumeMeta(t *testing.T, after store.SessionMeta, before store.SessionMeta) {
	t.Helper()

	if after.State != before.State {
		t.Fatalf("restored meta state = %q, want %q", after.State, before.State)
	}
	if derefString(after.ACPSessionID) != derefString(before.ACPSessionID) {
		t.Fatalf(
			"restored meta acp session id = %q, want %q",
			derefString(after.ACPSessionID),
			derefString(before.ACPSessionID),
		)
	}
	if sessionMetaStopReason(after) != sessionMetaStopReason(before) {
		t.Fatalf(
			"restored meta stop reason = %q, want %q",
			sessionMetaStopReason(after),
			sessionMetaStopReason(before),
		)
	}
	assertSameSessionFailure(t, after.Failure, before.Failure, "restored meta")
}

func assertRestoredResumeCatalog(t *testing.T, after store.SessionInfo, before store.SessionInfo) {
	t.Helper()

	if after.State != before.State {
		t.Fatalf("restored catalog state = %q, want %q", after.State, before.State)
	}
	if derefString(after.ACPSessionID) != derefString(before.ACPSessionID) {
		t.Fatalf(
			"restored catalog acp session id = %q, want %q",
			derefString(after.ACPSessionID),
			derefString(before.ACPSessionID),
		)
	}
	if after.StopReason != before.StopReason {
		t.Fatalf("restored catalog stop reason = %q, want %q", after.StopReason, before.StopReason)
	}
	if after.StopDetail != before.StopDetail {
		t.Fatalf("restored catalog stop detail = %q, want %q", after.StopDetail, before.StopDetail)
	}
	assertSameSessionFailure(t, after.Failure, before.Failure, "restored catalog")
}

func assertSameSessionFailure(
	t *testing.T,
	after *store.SessionFailure,
	before *store.SessionFailure,
	label string,
) {
	t.Helper()

	if before == nil || after == nil {
		if before != after {
			t.Fatalf("%s failure = %#v, want %#v", label, after, before)
		}
		return
	}
	if after.Kind != before.Kind {
		t.Fatalf("%s failure kind = %q, want %q", label, after.Kind, before.Kind)
	}
	if after.Summary != before.Summary {
		t.Fatalf("%s failure summary = %q, want %q", label, after.Summary, before.Summary)
	}
}
