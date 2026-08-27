package observe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestNewWithEmptyHomePathsReturnsError(t *testing.T) {
	t.Parallel()

	if _, err := New(testutil.Context(t), WithHomePaths(compozyconfig.HomePaths{})); err == nil {
		t.Fatal("New(empty home paths) error = nil, want non-nil")
	}
}

func TestValidObservedSessionID(t *testing.T) {
	t.Run("Should reject a parent directory path component", func(t *testing.T) {
		t.Parallel()

		if validObservedSessionID("..") {
			t.Fatal("validObservedSessionID(\"..\") = true, want false")
		}
	})
}

func TestNewOpensRegistryAndCloseSucceeds(t *testing.T) {
	home, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	observer, err := New(observeTestContext(t),
		WithHomePaths(home),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if observer.registry == nil {
		t.Fatal("observer.registry = nil, want opened registry")
	}
	if observer.registry.Path() != home.DatabaseFile {
		t.Fatalf("observer.registry.Path() = %q, want %q", observer.registry.Path(), home.DatabaseFile)
	}
	if err := observer.Close(observeTestContext(t)); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWithTaskDashboardConfigOverridesDefaults(t *testing.T) {
	t.Parallel()

	home, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	observer, err := New(
		observeTestContext(t),
		WithHomePaths(home),
		WithTaskDashboardConfig(TaskDashboardConfig{
			ActiveRunLimit:   7,
			BacklogWarnAfter: 45 * time.Second,
			StaleAfter:       15 * time.Second,
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := observer.Close(observeTestContext(t)); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	if got, want := observer.taskDashboardConfig.activeRunLimit, 7; got != want {
		t.Fatalf("taskDashboardConfig.activeRunLimit = %d, want %d", got, want)
	}
	if got, want := observer.taskDashboardConfig.backlogWarnAfter, 45*time.Second; got != want {
		t.Fatalf("taskDashboardConfig.backlogWarnAfter = %v, want %v", got, want)
	}
	if got, want := observer.taskDashboardConfig.staleAfter, 15*time.Second; got != want {
		t.Fatalf("taskDashboardConfig.staleAfter = %v, want %v", got, want)
	}
}

func TestOnSessionCreatedResolverFailureStillTracksSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.resolveProviderAuth = func(context.Context, string, string, string, string) (
		compozyconfig.ProviderAuthMode,
		error,
	) {
		return "", errors.New("boom")
	}

	sess := newSession("sess-resolver-failure", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	snapshot, ok := h.observer.sessionSnapshot(sess.ID)
	if !ok {
		t.Fatal("sessionSnapshot() cached = false, want true")
	}
	if snapshot.permissionMode != sess.EffectivePermissions {
		t.Fatalf(
			"sessionSnapshot().permissionMode = %q, want live session value %q",
			snapshot.permissionMode,
			sess.EffectivePermissions,
		)
	}
}

func TestHealthFallsBackToRegistryWithoutSessionSource(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.sessionSource = nil

	now := h.now
	lastActivityAt := now.Add(-40 * time.Minute)
	for _, info := range []store.SessionInfo{
		{
			ID:            "sess-active",
			AgentName:     "coder",
			WorkspaceID:   h.workspaceID,
			State:         "active",
			RuntimeStatus: store.SessionRuntimeUnbound,
			Liveness: &store.SessionLivenessMeta{
				StallState:  store.SessionStallStateDetected,
				StallReason: store.SessionStallReasonActivityTimeout,
				Activity: &store.SessionActivityMeta{
					TurnID:           "turn-stalled",
					LastActivityAt:   &lastActivityAt,
					LastActivityKind: "timeout",
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{ID: "sess-stopped", AgentName: "coder", WorkspaceID: h.workspaceID, State: "stopped", RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now},
		{ID: "sess-orphaned", AgentName: "coder", WorkspaceID: h.workspaceID, State: "orphaned", RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now},
	} {
		info.ProfileID = store.DefaultProfileID
		if err := h.observer.registry.RegisterSession(testutil.Context(t), info); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
		}
	}

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health(nil) error = %v", err)
	}
	if health.ActiveSessions != 1 || health.ActiveAgents != 1 {
		t.Fatalf("Health() = %#v, want 1 active session/agent", health)
	}
	if len(health.Activities) != 1 || health.Activities[0].Status != "stalled" {
		t.Fatalf("Health().Activities = %#v, want one stalled activity", health.Activities)
	}
}

func TestSessionDBSizeHelpers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "sess-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	dbPath := store.SessionDBFile(sessionDir)
	if err := os.WriteFile(dbPath, []byte("db"), 0o644); err != nil {
		t.Fatalf("WriteFile(db) error = %v", err)
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatalf("WriteFile(wal) error = %v", err)
	}
	if err := os.WriteFile(dbPath+"-shm", []byte("shm"), 0o644); err != nil {
		t.Fatalf("WriteFile(shm) error = %v", err)
	}

	gotDB, err := databaseSize(dbPath)
	if err != nil {
		t.Fatalf("databaseSize() error = %v", err)
	}
	if gotDB != int64(len("db")+len("wal")+len("shm")) {
		t.Fatalf("databaseSize() = %d", gotDB)
	}

	gotTotal, err := totalSessionDBSize(dir)
	if err != nil {
		t.Fatalf("totalSessionDBSize() error = %v", err)
	}
	if gotTotal != gotDB {
		t.Fatalf("totalSessionDBSize() = %d, want %d", gotTotal, gotDB)
	}

	empty, err := databaseSize("")
	if err != nil {
		t.Fatalf("databaseSize(empty) error = %v", err)
	}
	if empty != 0 {
		t.Fatalf("databaseSize(empty) = %d, want 0", empty)
	}
}

func TestLoadSessionMetadataSkipsMissingMetaAndKeepsStoppedState(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if err := os.MkdirAll(filepath.Join(h.home.SessionsDir, "sess-empty"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	sessionDir := filepath.Join(h.home.SessionsDir, "sess-stopped")
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), store.SessionMeta{
		ID:                   "sess-stopped",
		ProfileID:            store.DefaultProfileID,
		Name:                 "Stopped",
		AgentName:            "coder",
		Provider:             "claude",
		WorkspaceID:          h.workspaceID,
		NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
		State:                "stopped",
		RuntimeStatus:        store.SessionRuntimeUnbound,
		CreatedAt:            h.now,
		UpdatedAt:            h.now,
	}); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	sessions, err := h.observer.loadSessionMetadata(testutil.Context(t))
	if err != nil {
		t.Fatalf("loadSessionMetadata() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}
	if sessions[0].State != "stopped" {
		t.Fatalf("sessions[0].State = %q, want stopped", sessions[0].State)
	}
}

func TestLoadSessionMetadataLogsInvalidProviderSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	var logs bytes.Buffer
	h.observer.logger = slog.New(slog.NewTextHandler(&logs, nil))

	sessionDir := filepath.Join(h.home.SessionsDir, "sess-without-provider")
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), store.SessionMeta{
		ID:                   "sess-without-provider",
		ProfileID:            store.DefaultProfileID,
		Name:                 "Missing Provider",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
		State:                "stopped",
		RuntimeStatus:        store.SessionRuntimeUnbound,
		CreatedAt:            h.now,
		UpdatedAt:            h.now,
	}); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	sessions, err := h.observer.loadSessionMetadata(testutil.Context(t))
	if err != nil {
		t.Fatalf("loadSessionMetadata() error = %v", err)
	}
	if got := len(sessions); got != 0 {
		t.Fatalf("len(sessions) = %d, want 0 for invalid provider metadata", got)
	}
	if !strings.Contains(logs.String(), "session_id=sess-without-provider") ||
		!strings.Contains(logs.String(), "skipping invalid session metadata") {
		t.Fatalf("logs = %q, want original session_id", logs.String())
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the profile owner when projecting a session", func(t *testing.T) {
		t.Parallel()
		info := &session.Info{ID: "sess-profile", ProfileID: "profile-observe"}
		projected := sessionInfoFromSession(info)
		if projected.ProfileID != info.ProfileID {
			t.Fatalf("sessionInfoFromSession().ProfileID = %q, want %q", projected.ProfileID, info.ProfileID)
		}
	})

	if got := sessionInfoFromSession(nil); !reflect.DeepEqual(got, store.SessionInfo{}) {
		t.Fatalf("sessionInfoFromSession(nil) = %#v, want zero value", got)
	}
	if got := stringPointer(""); got != nil {
		t.Fatalf("stringPointer(\"\") = %#v, want nil", got)
	}
	if got := summarizeEvent(acp.AgentEvent{Raw: []byte(`{"hello":"world"}`)}); got != `{"hello":"world"}` {
		t.Fatalf("summarizeEvent(raw) = %q", got)
	}
	long := truncateSummary(strings.Repeat("a", 300))
	if len([]rune(long)) != 240 {
		t.Fatalf("truncateSummary(len=300) rune count = %d, want 240", len([]rune(long)))
	}
}

func TestSummarizeEventPrefersPermissionSpecificFields(t *testing.T) {
	t.Parallel()

	got := summarizeEvent(acp.AgentEvent{
		Type:     acp.EventTypePermission,
		Title:    "permission.request",
		Resource: "/tmp/secret.txt",
		Decision: "deny",
		Text:     "fallback text",
	})
	if got != "permission.request" {
		t.Fatalf("summarizeEvent(permission) = %q, want %q", got, "permission.request")
	}
}

func TestObserverVersionSourceUsedByHealth(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.startedAt = h.now
	h.observer.now = func() time.Time { return h.now.Add(time.Second) }

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.Version != "1.2.3" {
		t.Fatalf("Health().Version = %q, want 1.2.3", health.Version)
	}
}

func TestMissingPathHelpers(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "missing")
	size, err := totalSessionDBSize(missingDir)
	if err != nil {
		t.Fatalf("totalSessionDBSize(missing) error = %v", err)
	}
	if size != 0 {
		t.Fatalf("totalSessionDBSize(missing) = %d, want 0", size)
	}

	h := newHarness(t)
	h.observer.homePaths.SessionsDir = missingDir
	sessions, err := h.observer.loadSessionMetadata(testutil.Context(t))
	if err != nil {
		t.Fatalf("loadSessionMetadata(missing) error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("len(loadSessionMetadata(missing)) = %d, want 0", len(sessions))
	}
}

type fakeObserveWorkspaceResolver struct {
	expectedRef string
	resolved    workspacepkg.ResolvedWorkspace
	err         error
	calls       atomic.Int64
}

func (r *fakeObserveWorkspaceResolver) Resolve(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
	r.calls.Add(1)
	if r.err != nil {
		return workspacepkg.ResolvedWorkspace{}, r.err
	}
	if want := strings.TrimSpace(r.expectedRef); want != "" && strings.TrimSpace(ref) != want {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf("unexpected workspace ref %q, want %q", ref, want)
	}
	return r.resolved, nil
}

func (r *fakeObserveWorkspaceResolver) ResolveOrRegister(
	_ context.Context,
	ref string,
) (workspacepkg.ResolvedWorkspace, error) {
	r.calls.Add(1)
	if r.err != nil {
		return workspacepkg.ResolvedWorkspace{}, r.err
	}
	if want := strings.TrimSpace(r.expectedRef); want != "" && strings.TrimSpace(ref) != want {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf("unexpected workspace ref %q, want %q", ref, want)
	}
	return r.resolved, nil
}

func seedObserveProfile(t testing.TB, registry *globaldb.GlobalDB, profileID string, createdAt time.Time) {
	t.Helper()
	if _, err := registry.DB().ExecContext(
		testutil.Context(t),
		`INSERT INTO profiles (id, name, color, icon, state, created_at)
		 VALUES (?, ?, '#8E8EB5', 'circle', 'active', ?)`,
		profileID,
		profileID,
		store.FormatTimestamp(createdAt),
	); err != nil {
		t.Fatalf("insert observe profile %q: %v", profileID, err)
	}
}
