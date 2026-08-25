//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	workspaceAccessIntegrationHome   = "ws_0000000000000000"
	workspaceAccessIntegrationTarget = "ws_0000000000000001"
)

func TestWorkspaceAccessIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should IT-003 persist one queryable audit summary per decision", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(testutil.Context(t)); err != nil {
				t.Fatalf("GlobalDB.Close() error = %v", err)
			}
		})
		emitter, err := newWorkspaceAccessAuditEmitter(db, time.Now)
		if err != nil {
			t.Fatalf("newWorkspaceAccessAuditEmitter() error = %v", err)
		}
		request := workspaceAccessIntegrationRequest(workspaceAccessIntegrationTarget)

		allowPolicy := workspaceAccessIntegrationPolicy(t, workspaceaccess.ModeApproveAll, emitter, slog.Default())
		assertWorkspaceAccessAuditDelta(t, ctx, db, 0, func() {
			decision, authErr := allowPolicy.Authorize(ctx, request)
			if authErr != nil {
				t.Fatalf("Authorize(allow) error = %v", authErr)
			}
			if !decision.Allowed {
				t.Fatalf("Authorize(allow) decision = %#v, want allowed", decision)
			}
		})

		denyPolicy := workspaceAccessIntegrationPolicy(t, workspaceaccess.ModeDenyAll, emitter, slog.Default())
		assertWorkspaceAccessAuditDelta(t, ctx, db, 1, func() {
			decision, authErr := denyPolicy.Authorize(ctx, request)
			if authErr != nil {
				t.Fatalf("Authorize(deny) error = %v", authErr)
			}
			if decision.Allowed {
				t.Fatalf("Authorize(deny) decision = %#v, want denied", decision)
			}
		})

		assertWorkspaceAccessAuditDelta(t, ctx, db, 2, func() {
			invalid := request
			invalid.TargetWorkspaceID = "workspace-name"
			decision, authErr := denyPolicy.Authorize(ctx, invalid)
			if authErr == nil || !strings.Contains(authErr.Error(), "is not canonical") {
				t.Fatalf("Authorize(error) error = %v, want canonical-id validation", authErr)
			}
			if decision.Allowed {
				t.Fatalf("Authorize(error) decision = %#v, want denied", decision)
			}
		})

		summaries := workspaceAccessIntegrationSummaries(t, ctx, db)
		granted := 0
		denied := 0
		for _, summary := range summaries {
			switch summary.Type {
			case "workspace.access_granted":
				granted++
			case "workspace.access_denied":
				denied++
			default:
				t.Fatalf("event type = %q, want workspace access event", summary.Type)
			}
			if summary.WorkspaceID != workspaceAccessIntegrationHome || summary.SessionID != "sess-a" ||
				summary.AgentName != "coder" || summary.ActorKind != string(workspaceaccess.ActorAgentSession) {
				t.Fatalf("event summary scope = %#v, want actor-home correlation", summary)
			}
			var payload workspaceAccessAuditPayload
			if err := json.Unmarshal(summary.Content, &payload); err != nil {
				t.Fatalf("json.Unmarshal(Content) error = %v", err)
			}
			if payload.TargetWorkspaceID == "" || payload.Seam != workspaceaccess.SeamTool || payload.Source == "" {
				t.Fatalf("audit payload = %#v, want target, seam, and source", payload)
			}
		}
		if granted != 1 || denied != 2 {
			t.Fatalf("event counts = granted:%d denied:%d, want 1 and 2", granted, denied)
		}

		var logs bytes.Buffer
		failingStore := &workspaceAccessIntegrationFailingStore{EventSummaryStore: db}
		failingEmitter, err := newWorkspaceAccessAuditEmitter(failingStore, time.Now)
		if err != nil {
			t.Fatalf("newWorkspaceAccessAuditEmitter(failing) error = %v", err)
		}
		failurePolicy := workspaceAccessIntegrationPolicy(
			t,
			workspaceaccess.ModeApproveAll,
			failingEmitter,
			slog.New(slog.NewTextHandler(&logs, nil)),
		)
		decision, err := failurePolicy.Authorize(ctx, request)
		if err != nil {
			t.Fatalf("Authorize(append failure) error = %v", err)
		}
		if !decision.Allowed || failingStore.writes != 1 {
			t.Fatalf(
				"append failure result = decision:%#v writes:%d, want allowed and one write",
				decision,
				failingStore.writes,
			)
		}
		if !bytes.Contains(logs.Bytes(), []byte("workspace access audit emission failed")) {
			t.Fatalf("logs = %q, want audit failure warning", logs.String())
		}

		deadlineStore := &workspaceAccessIntegrationDeadlineStore{EventSummaryStore: db}
		deadlineEmitter, err := newWorkspaceAccessAuditEmitter(deadlineStore, time.Now)
		if err != nil {
			t.Fatalf("newWorkspaceAccessAuditEmitter(deadline) error = %v", err)
		}
		startedAt := time.Now()
		if err := deadlineEmitter.EmitWorkspaceAccess(ctx, workspaceaccess.AccessRecord{
			Actor: workspaceaccess.ActorRef{
				Kind:        workspaceaccess.ActorAgentSession,
				SessionID:   "sess-a",
				WorkspaceID: workspaceAccessIntegrationHome,
				AgentName:   "coder",
			},
			TargetID: workspaceAccessIntegrationTarget,
			Seam:     workspaceaccess.SeamTool,
		}); err != nil {
			t.Fatalf("EmitWorkspaceAccess(deadline) error = %v", err)
		}
		if deadlineStore.deadline.Before(startedAt) || deadlineStore.deadline.After(
			startedAt.Add(workspaceAccessAuditWriteTimeout+time.Second),
		) {
			t.Fatalf(
				"audit deadline = %s, want bounded by %s",
				deadlineStore.deadline,
				workspaceAccessAuditWriteTimeout,
			)
		}
		select {
		case <-deadlineStore.done:
		default:
			t.Fatal("audit write context remained live after EmitWorkspaceAccess returned")
		}
	})

	t.Run("Should IT-024 resolve only live managed session permission modes", func(t *testing.T) {
		t.Parallel()

		manager, managed := newWorkspaceAccessIntegrationSession(
			t,
			nil,
			compozyconfig.PermissionModeApproveAll,
		)
		source, err := newWorkspaceAccessModeSource(manager)
		if err != nil {
			t.Fatalf("newWorkspaceAccessModeSource() error = %v", err)
		}

		mode, err := source.SessionPermissionMode(testutil.Context(t), managed.ID)
		if err != nil {
			t.Fatalf("SessionPermissionMode(live) error = %v", err)
		}
		if mode != workspaceaccess.ModeApproveAll {
			t.Fatalf("SessionPermissionMode(live) = %q, want %q", mode, workspaceaccess.ModeApproveAll)
		}
		if _, err := source.SessionPermissionMode(testutil.Context(t), "missing"); !errors.Is(
			err,
			session.ErrSessionNotFound,
		) {
			t.Fatalf("SessionPermissionMode(missing) error = %v, want ErrSessionNotFound", err)
		}
		if err := manager.Stop(testutil.Context(t), managed.ID); err != nil {
			t.Fatalf("Manager.Stop() error = %v", err)
		}
		if _, err := source.SessionPermissionMode(testutil.Context(t), managed.ID); err == nil ||
			!strings.Contains(err.Error(), "is stopped") {
			t.Fatalf("SessionPermissionMode(stopped) error = %v, want stopped-session contract", err)
		}
	})

	t.Run("Should IT-025 apply session consent until the real session lifecycle stops", func(t *testing.T) {
		t.Parallel()

		cache := newWorkspaceAccessConsentCache()
		notifier := workspaceAccessIntegrationNotifier{fanout: newSessionLifecycleFanout(cache)}
		manager, managed := newWorkspaceAccessIntegrationSession(
			t,
			notifier,
			compozyconfig.PermissionModeApproveReads,
		)
		source, err := newWorkspaceAccessModeSource(manager)
		if err != nil {
			t.Fatalf("newWorkspaceAccessModeSource() error = %v", err)
		}
		policy, err := workspaceaccess.New(workspaceaccess.Deps{
			Modes:   source,
			Consent: cache,
			Audit:   workspaceAccessIntegrationAuditStub{},
			Log:     slog.Default(),
		})
		if err != nil {
			t.Fatalf("workspaceaccess.New() error = %v", err)
		}
		cache.PutConsent(testutil.Context(t), managed.ID, workspaceaccess.ConsentAllow)
		req := workspaceAccessIntegrationRequest(workspaceAccessIntegrationTarget)
		req.Actor.SessionID = managed.ID

		decision, err := policy.Authorize(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("Authorize(cached consent) error = %v", err)
		}
		if !decision.Allowed || decision.Source != workspaceaccess.SourceSessionConsent {
			t.Fatalf("Authorize(cached consent) = %#v, want session-consent allow", decision)
		}
		if err := manager.Stop(testutil.Context(t), managed.ID); err != nil {
			t.Fatalf("Manager.Stop() error = %v", err)
		}
		if consent, ok := cache.ConsentFor(testutil.Context(t), managed.ID); ok || consent != "" {
			t.Fatalf("ConsentFor(stopped session) = (%q, %t), want miss", consent, ok)
		}
	})
}

func newWorkspaceAccessIntegrationSession(
	t *testing.T,
	notifier session.Notifier,
	permissions compozyconfig.PermissionMode,
) (*session.Manager, *session.Session) {
	t.Helper()
	ctx := testutil.Context(t)
	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	root := filepath.Join(homePaths.HomeDir, "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	cfg := compozyconfig.DefaultWithHome(homePaths)
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      workspaceAccessIntegrationHome,
			RootDir: root,
			Name:    "workspace",
		},
		Config: cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are a coding assistant.",
		}},
	}
	driver := newWorkspaceAccessIntegrationDriver()
	opts := []session.Option{
		session.WithHomePaths(homePaths),
		session.WithWorkspaceResolver(workspaceAccessIntegrationResolver{resolved: resolved}),
		session.WithDriver(driver),
		session.WithStore(
			func(ctx context.Context, owner store.SessionDBOwner, path string) (session.EventRecorder, error) {
				return sessiondb.OpenSessionDB(ctx, owner, path)
			},
		),
		session.WithQueryStore(
			func(ctx context.Context, owner store.SessionDBOwner, path string) (session.EventReadCloser, error) {
				return sessiondb.OpenSessionDBReadOnly(ctx, owner, path)
			},
		),
		session.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		session.WithLifecycleContext(ctx),
	}
	if notifier != nil {
		opts = append(opts, session.WithNotifier(notifier))
	}
	manager, err := session.NewManager(opts...)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	managed, err := manager.Create(ctx, session.CreateOpts{
		AgentName:      "coder",
		Workspace:      workspaceAccessIntegrationHome,
		Permissions:    permissions,
		DisableSandbox: true,
	})
	if err != nil {
		t.Fatalf("Manager.Create() error = %v", err)
	}
	t.Cleanup(func() {
		info, statusErr := manager.Status(testutil.Context(t), managed.ID)
		if statusErr == nil && info.State != session.StateStopped {
			if stopErr := manager.Stop(testutil.Context(t), managed.ID); stopErr != nil {
				t.Fatalf("Manager.Stop(cleanup) error = %v", stopErr)
			}
		}
	})
	return manager, managed
}

func workspaceAccessIntegrationPolicy(
	t *testing.T,
	mode workspaceaccess.Mode,
	audit workspaceaccess.AuditEmitter,
	logger *slog.Logger,
) *workspaceaccess.DefaultPolicy {
	t.Helper()
	policy, err := workspaceaccess.New(workspaceaccess.Deps{
		Modes:   workspaceAccessIntegrationModeSource{mode: mode},
		Consent: workspaceAccessIntegrationConsentStub{},
		Audit:   audit,
		Log:     logger,
	})
	if err != nil {
		t.Fatalf("workspaceaccess.New() error = %v", err)
	}
	return policy
}

func workspaceAccessIntegrationRequest(target string) workspaceaccess.Request {
	return workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorAgentSession,
			SessionID:   "sess-a",
			WorkspaceID: workspaceAccessIntegrationHome,
			AgentName:   "coder",
		},
		TargetWorkspaceID: target,
		Seam:              workspaceaccess.SeamTool,
	}
}

func assertWorkspaceAccessAuditDelta(
	t *testing.T,
	ctx context.Context,
	db store.EventSummaryStore,
	wantBefore int,
	action func(),
) {
	t.Helper()
	if got := len(workspaceAccessIntegrationSummaries(t, ctx, db)); got != wantBefore {
		t.Fatalf("summaries before action = %d, want %d", got, wantBefore)
	}
	action()
	if got := len(workspaceAccessIntegrationSummaries(t, ctx, db)); got != wantBefore+1 {
		t.Fatalf("summaries after action = %d, want %d", got, wantBefore+1)
	}
}

func workspaceAccessIntegrationSummaries(
	t *testing.T,
	ctx context.Context,
	db store.EventSummaryStore,
) []store.EventSummary {
	t.Helper()
	summaries, err := db.ListEventSummaries(ctx, store.EventSummaryQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: workspaceAccessIntegrationHome,
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	for _, summary := range summaries {
		if summary.ProfileID != store.DefaultProfileID {
			t.Fatalf("event summary profile = %q, want %q", summary.ProfileID, store.DefaultProfileID)
		}
	}
	return summaries
}

type workspaceAccessIntegrationModeSource struct {
	mode workspaceaccess.Mode
}

func (s workspaceAccessIntegrationModeSource) SessionPermissionMode(
	context.Context,
	string,
) (workspaceaccess.Mode, error) {
	return s.mode, nil
}

type workspaceAccessIntegrationConsentStub struct{}

func (workspaceAccessIntegrationConsentStub) ConsentFor(
	context.Context,
	string,
) (workspaceaccess.Consent, bool) {
	return "", false
}

func (workspaceAccessIntegrationConsentStub) PutConsent(
	context.Context,
	string,
	workspaceaccess.Consent,
) {
}

type workspaceAccessIntegrationAuditStub struct{}

func (workspaceAccessIntegrationAuditStub) EmitWorkspaceAccess(
	context.Context,
	workspaceaccess.AccessRecord,
) error {
	return nil
}

type workspaceAccessIntegrationFailingStore struct {
	store.EventSummaryStore
	writes int
}

type workspaceAccessIntegrationDeadlineStore struct {
	store.EventSummaryStore
	deadline time.Time
	done     <-chan struct{}
}

func (s *workspaceAccessIntegrationDeadlineStore) WriteEventSummary(
	ctx context.Context,
	_ store.EventSummary,
) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("workspace access audit write context has no deadline")
	}
	s.deadline = deadline
	s.done = ctx.Done()
	return nil
}

func (s *workspaceAccessIntegrationFailingStore) WriteEventSummary(
	context.Context,
	store.EventSummary,
) error {
	s.writes++
	return errors.New("forced append failure")
}

type workspaceAccessIntegrationResolver struct {
	resolved workspacepkg.ResolvedWorkspace
}

func (r workspaceAccessIntegrationResolver) Resolve(
	_ context.Context,
	ref string,
) (workspacepkg.ResolvedWorkspace, error) {
	if ref == r.resolved.ID || ref == r.resolved.RootDir {
		return r.resolved, nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r workspaceAccessIntegrationResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.Resolve(ctx, path)
}

type workspaceAccessIntegrationNotifier struct {
	fanout *sessionLifecycleFanout
}

func (n workspaceAccessIntegrationNotifier) OnSessionCreated(ctx context.Context, sess *session.Session) {
	n.fanout.OnSessionCreated(ctx, sess)
}

func (n workspaceAccessIntegrationNotifier) OnSessionStopped(ctx context.Context, sess *session.Session) {
	n.fanout.OnSessionStopped(ctx, sess)
}

func (workspaceAccessIntegrationNotifier) OnAgentEvent(context.Context, string, any) {}

type workspaceAccessIntegrationDriver struct {
	mu        sync.Mutex
	processes map[*session.AgentProcess]*workspaceAccessIntegrationProcess
}

type workspaceAccessIntegrationProcess struct {
	done chan struct{}
	once sync.Once
}

func newWorkspaceAccessIntegrationDriver() *workspaceAccessIntegrationDriver {
	return &workspaceAccessIntegrationDriver{
		processes: make(map[*session.AgentProcess]*workspaceAccessIntegrationProcess),
	}
}

func (d *workspaceAccessIntegrationDriver) Start(
	_ context.Context,
	opts acp.StartOpts,
) (*session.AgentProcess, error) {
	procState := &workspaceAccessIntegrationProcess{done: make(chan struct{})}
	proc := session.NewAgentProcess(session.AgentProcessOptions{
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		SessionID: "acp-workspace-access",
		StartedAt: time.Now().UTC(),
		Done:      procState.done,
	})
	d.mu.Lock()
	d.processes[proc] = procState
	d.mu.Unlock()
	return proc, nil
}

func (*workspaceAccessIntegrationDriver) Prompt(
	context.Context,
	*session.AgentProcess,
	acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	events := make(chan acp.AgentEvent)
	close(events)
	return events, nil
}

func (*workspaceAccessIntegrationDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *workspaceAccessIntegrationDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	procState := d.processes[proc]
	d.mu.Unlock()
	if procState == nil {
		return fmt.Errorf("workspace access integration: unknown process")
	}
	procState.once.Do(func() { close(procState.done) })
	return nil
}
