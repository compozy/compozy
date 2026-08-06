package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/sandbox"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/toolruntime"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	skillbundled "github.com/compozy/compozy/skills"
)

func testLiveParticipation(workspaceID, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     strings.TrimSpace(workspaceID),
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       strings.TrimSpace(channelID),
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}
}

func testLocalParticipation() participation.Spec {
	return participation.LocalSpec()
}

func testSessionDBOwner(sessionID string, workspaceID string) store.SessionDBOwner {
	return store.SessionDBOwner{SessionID: sessionID, WorkspaceID: workspaceID}
}

func testLocalParticipationPtr() *participation.Spec {
	return participation.CloneSpec(testLocalParticipation())
}

func testLiveParticipationPtr(workspaceID, channelID string) *participation.Spec {
	spec := testLiveParticipation(workspaceID, channelID)
	return &spec
}

type recordingSessionParticipationResolver struct {
	inner        participation.Resolver
	calls        int
	observations []participation.ResolvedObservation
}

func (r *recordingSessionParticipationResolver) ObserveParticipationResolved(
	_ context.Context,
	observation participation.ResolvedObservation,
) error {
	r.observations = append(r.observations, observation)
	return nil
}

func (r *recordingSessionParticipationResolver) Resolve(
	ctx context.Context,
	in participation.ResolveInput,
) (participation.Spec, error) {
	r.calls++
	return r.inner.Resolve(ctx, in)
}

func newTestSessionParticipationResolver(t *testing.T, available bool) participation.Resolver {
	t.Helper()
	defaults := testLiveParticipation("ws-test", "builders").Bounds
	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: defaults,
		Limits: participation.Limits{
			MaxWakes:          16,
			MaxWakeWallTime:   "2m",
			MaxTotalWallTime:  "10m",
			MaxInputTokens:    65536,
			MaxOutputTokens:   65536,
			MaxWakeDepth:      16,
			MinCoalesceWindow: "100ms",
			MaxCoalesceWindow: "5s",
		},
		Availability: func(context.Context) (bool, error) {
			return available, nil
		},
		ChannelExists: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
}

func receivePromptEvent(t *testing.T, events <-chan acp.AgentEvent) acp.AgentEvent {
	t.Helper()

	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("prompt output channel closed before expected event")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for prompt event")
		return acp.AgentEvent{}
	}
}

type harness struct {
	manager       *Manager
	driver        *fakeDriver
	notifier      *fakeNotifier
	resolver      *fakeWorkspaceResolver
	sandbox       *sandbox.Registry
	cfg           compozyconfig.Config
	homePaths     compozyconfig.HomePaths
	workspace     string
	workspaceID   string
	workspaceName string
}

func reportSessionStop(t *testing.T, h *harness, sessionID string) {
	t.Helper()
	if err := h.manager.Stop(testutil.Context(t), sessionID); err != nil {
		t.Errorf("Stop(%q) cleanup error = %v", sessionID, err)
	}
}

func newHarness(t *testing.T, extraOpts ...Option) *harness {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	workspace := filepath.Join(homePaths.HomeDir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}

	h := &harness{
		driver:        newFakeDriver(),
		notifier:      newFakeNotifier(),
		sandbox:       newFakeSandboxRegistry(t),
		cfg:           compozyconfig.DefaultWithHome(homePaths),
		homePaths:     homePaths,
		workspace:     workspace,
		workspaceID:   "ws-primary",
		workspaceName: "workspace",
	}
	resolvedSandbox, err := h.cfg.ResolveSandbox(h.cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	h.resolver = newFakeWorkspaceResolver(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{
			{
				Name:     compozyconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
		},
		Sandbox: resolvedSandbox,
	})
	h.manager = newManagerWithHarness(t, h, extraOpts...)
	return h
}

func newHostedMCPHarness(t *testing.T, extraOpts ...Option) *harness {
	t.Helper()

	hosted := &recordingHostedMCPLauncher{
		server: compozyconfig.MCPServer{
			Name:      "compozy-hosted-tools",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   "/bin/compozy",
			Args:      []string{"tool", "mcp", "--session", "test"},
		},
	}
	return newHarness(t, append(extraOpts, WithHostedMCPLauncher(hosted))...)
}

func newManagerWithHarness(t *testing.T, h *harness, extraOpts ...Option) *Manager {
	t.Helper()

	opts := []Option{
		WithHomePaths(h.homePaths),
		WithDriver(h.driver),
		WithNotifier(h.notifier),
		WithPromptAssembler(
			startupPromptAssemblerFunc(
				func(_ context.Context, startup StartupPromptContext, agent compozyconfig.AgentDef, _ *workspacepkg.ResolvedWorkspace) (string, error) {
					prompt := strings.TrimSpace(agent.Prompt)
					if startup.NetworkParticipation.Mode != participation.ModeLive {
						return prompt, nil
					}
					networkSkill, err := skillbundled.LoadResource(
						testBundledCompozySkillName,
						testBundledNetworkReference,
					)
					if err != nil {
						return "", err
					}
					return prompt + "\n\n" + strings.TrimSpace(networkSkill), nil
				},
			),
		),
		WithWorkspaceResolver(h.resolver),
		WithStore(func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
			return sessiondb.OpenSessionDB(ctx, owner, path)
		}),
		WithQueryStore(func(ctx context.Context, owner store.SessionDBOwner, path string) (EventReadCloser, error) {
			return sessiondb.OpenSessionDBReadOnly(ctx, owner, path)
		}),
		func(manager *Manager) {
			manager.queryStoreExplicit = false
		},
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithSessionIDGenerator(sequentialIDGenerator("sess")),
		WithTurnIDGenerator(sequentialIDGenerator("turn")),
		WithSandboxRegistry(h.sandbox),
		WithSandboxIDGenerator(sequentialIDGenerator("env")),
	}
	opts = append(opts, extraOpts...)

	manager, err := NewManager(opts...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func cleanupTestManager(t *testing.T, manager *Manager) {
	t.Helper()
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Manager.Shutdown() error = %v", err)
		}
	})
}

func createSession(t *testing.T, h *harness) *Session {
	t.Helper()

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "session",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return session
}

func createLiveNetworkSession(t *testing.T, h *harness) *Session {
	t.Helper()

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "network-session",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create(live network session) error = %v", err)
	}
	return session
}

func readMeta(t *testing.T, path string) store.SessionMeta {
	t.Helper()

	meta, err := store.ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta(%q) error = %v", path, err)
	}
	return meta
}

func readStoredEvents(t *testing.T, session *Session) []store.SessionEvent {
	t.Helper()

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(session.ID, session.WorkspaceID),
		session.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopened) error = %v", err)
	}
	return events
}

func storedEventByType(t *testing.T, events []store.SessionEvent, want string) store.SessionEvent {
	t.Helper()

	for _, event := range events {
		if event.Type == want {
			return event
		}
	}

	t.Fatalf("stored event type %q not found", want)
	return store.SessionEvent{}
}

func decodeStoredEventPayload(t *testing.T, event store.SessionEvent) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
		t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
	}
	return payload
}

func collectEvents(t *testing.T, eventsCh <-chan acp.AgentEvent) []acp.AgentEvent {
	t.Helper()

	events := make([]acp.AgentEvent, 0, 4)
	for event := range eventsCh {
		events = append(events, event)
	}
	return events
}

func containsEventType(events []store.SessionEvent, want string) bool {
	for _, event := range events {
		if event.Type == want {
			return true
		}
	}
	return false
}

func countEventType(events []store.SessionEvent, want string) int {
	count := 0
	for _, event := range events {
		if event.Type == want {
			count++
		}
	}
	return count
}

func sequentialIDGenerator(prefix string) IDGenerator {
	var counter atomic.Int64
	return func() (string, error) {
		return fmt.Sprintf("%s-%d", prefix, counter.Add(1)), nil
	}
}

type promptAssemblerFunc func(context.Context, compozyconfig.AgentDef, *workspacepkg.ResolvedWorkspace) (string, error)

func (fn promptAssemblerFunc) Assemble(
	ctx context.Context,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, agent, workspace)
}

type resumeContextPromptAssembler struct {
	checkpoint string
}

func (a *resumeContextPromptAssembler) Assemble(
	_ context.Context,
	agent compozyconfig.AgentDef,
	_ *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return agent.Prompt, nil
}

func (a *resumeContextPromptAssembler) ResumeContextSection(
	_ context.Context,
	_ StartupPromptContext,
) (string, error) {
	return a.checkpoint, nil
}

const (
	testBundledCompozySkillName = "compozy"
	testBundledNetworkReference = "references/network.md"
)

type startupPromptAssemblerFunc func(
	context.Context,
	StartupPromptContext,
	compozyconfig.AgentDef,
	*workspacepkg.ResolvedWorkspace,
) (string, error)

func (fn startupPromptAssemblerFunc) Assemble(
	ctx context.Context,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, StartupPromptContext{
		AgentName:   strings.TrimSpace(agent.Name),
		SessionType: SessionTypeUser,
	}, agent, workspace)
}

func (fn startupPromptAssemblerFunc) AssembleStartup(
	ctx context.Context,
	startup StartupPromptContext,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, startup, agent, workspace)
}

type startupPromptOverlayFunc func(context.Context, StartupPromptContext, string) (string, error)

func (fn startupPromptOverlayFunc) Apply(
	ctx context.Context,
	startup StartupPromptContext,
	prompt string,
) (string, error) {
	return fn(ctx, startup, prompt)
}

type fakeNotifier struct {
	mu             sync.Mutex
	created        []*Info
	stopped        []*Info
	stoppedSignal  chan string
	eventSignal    chan notifiedAgentEvent
	events         map[string][]acp.AgentEvent
	order          []string
	finalizingHook func(context.Context, *Session)
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{
		stoppedSignal: make(chan string, 16),
		eventSignal:   make(chan notifiedAgentEvent, 64),
		events:        make(map[string][]acp.AgentEvent),
	}
}

func (n *fakeNotifier) OnSessionCreated(_ context.Context, session *Session) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.created = append(n.created, session.Info())
	n.order = append(n.order, "created:"+session.ID)
}

func (n *fakeNotifier) OnSessionStopped(_ context.Context, session *Session) {
	n.mu.Lock()
	n.stopped = append(n.stopped, session.Info())
	n.order = append(n.order, "stopped:"+session.ID)
	n.mu.Unlock()
	n.stoppedSignal <- session.ID
}

func (n *fakeNotifier) waitForStopped(t *testing.T, sessionID string) {
	t.Helper()
	select {
	case got := <-n.stoppedSignal:
		if got != sessionID {
			t.Fatalf("stopped session = %q, want %q", got, sessionID)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for stopped session %q", sessionID)
	}
}

func (n *fakeNotifier) OnSessionFinalizing(ctx context.Context, session *Session) {
	n.mu.Lock()
	hook := n.finalizingHook
	if hook != nil {
		n.order = append(n.order, "finalizing:"+session.ID)
	}
	n.mu.Unlock()
	if hook != nil {
		hook(ctx, session)
	}
}

func (n *fakeNotifier) OnAgentEvent(_ context.Context, sessionID string, event any) {
	n.mu.Lock()
	agentEvent, ok := event.(acp.AgentEvent)
	if !ok {
		n.mu.Unlock()
		return
	}
	n.events[sessionID] = append(n.events[sessionID], agentEvent)
	n.mu.Unlock()
	n.eventSignal <- notifiedAgentEvent{sessionID: sessionID, eventType: agentEvent.Type}
}

type notifiedAgentEvent struct {
	sessionID string
	eventType string
}

func (n *fakeNotifier) waitForAgentEvent(t *testing.T, sessionID string, eventType string) {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case got := <-n.eventSignal:
			if got.sessionID == sessionID && got.eventType == eventType {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %s event in session %q", eventType, sessionID)
		}
	}
}

func (n *fakeNotifier) createdCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.created)
}

func (n *fakeNotifier) stoppedCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.stopped)
}

func (n *fakeNotifier) eventCount(sessionID string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.events[sessionID])
}

func (n *fakeNotifier) eventsForSession(sessionID string) []acp.AgentEvent {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]acp.AgentEvent(nil), n.events[sessionID]...)
}

func (n *fakeNotifier) notificationOrder() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.order...)
}

type fakeNetworkPeerLifecycle struct {
	mu    sync.Mutex
	joins []fakeNetworkJoinCall
	left  []string
}

type fakeNetworkJoinCall struct {
	sessionID    string
	peerID       string
	channel      string
	capabilities []NetworkPeerCapability
}

func newFakeNetworkPeerLifecycle() *fakeNetworkPeerLifecycle {
	return &fakeNetworkPeerLifecycle{}
}

func (f *fakeNetworkPeerLifecycle) JoinChannel(
	_ context.Context,
	join NetworkPeerJoin,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.joins = append(f.joins, fakeNetworkJoinCall{
		sessionID:    join.SessionID,
		peerID:       join.PeerID,
		channel:      join.Channel,
		capabilities: cloneNetworkPeerCapabilities(join.Capabilities),
	})
	return nil
}

func (f *fakeNetworkPeerLifecycle) LeaveChannel(_ context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.left = append(f.left, sessionID)
	return nil
}

func (f *fakeNetworkPeerLifecycle) joinCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.joins)
}

func (f *fakeNetworkPeerLifecycle) joinCall(index int) fakeNetworkJoinCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.joins[index]
}

func (f *fakeNetworkPeerLifecycle) leaveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.left)
}

func (f *fakeNetworkPeerLifecycle) leaveCall(index int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.left[index]
}

type fakeEventRecorder struct {
	closeCalls int
}

func (r *fakeEventRecorder) ListTokenUsage(context.Context) ([]store.TokenUsage, error) {
	return nil, nil
}

type markerFailingRecorder struct {
	EventRecorder
	failErr    error
	closeCalls int
}

func (r *markerFailingRecorder) Record(ctx context.Context, event store.SessionEvent) error {
	if event.Type == events.TranscriptMarkerCreated {
		return r.failErr
	}
	return r.EventRecorder.Record(ctx, event)
}

func (r *markerFailingRecorder) Close(ctx context.Context) error {
	r.closeCalls++
	return r.EventRecorder.Close(ctx)
}

type failingSinglePromptRecorder struct {
	mu      sync.Mutex
	failErr error
	failed  bool
	events  []store.SessionEvent
}

func (r *failingSinglePromptRecorder) ListTokenUsage(context.Context) ([]store.TokenUsage, error) {
	return nil, nil
}

func (r *failingSinglePromptRecorder) Record(ctx context.Context, event store.SessionEvent) error {
	_, err := r.RecordPersisted(ctx, event)
	return err
}

func (r *failingSinglePromptRecorder) RecordPersisted(
	_ context.Context,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failed || event.Type == acp.EventTypeToolCall {
		r.failed = true
		return store.SessionEvent{}, r.failErr
	}
	event.Sequence = int64(len(r.events) + 1)
	r.events = append(r.events, event)
	return event, nil
}

func (r *failingSinglePromptRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *failingSinglePromptRecorder) Query(
	context.Context,
	store.EventQuery,
) ([]store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]store.SessionEvent(nil), r.events...), nil
}

func (r *failingSinglePromptRecorder) History(
	context.Context,
	store.EventQuery,
) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *failingSinglePromptRecorder) Close(context.Context) error {
	return nil
}

func (r *fakeEventRecorder) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (r *fakeEventRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *fakeEventRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return nil, nil
}

func (r *fakeEventRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *fakeEventRecorder) Close(context.Context) error {
	r.closeCalls++
	return nil
}

type fakeDriver struct {
	mu                    sync.Mutex
	startCalls            []acp.StartOpts
	promptCalls           []acp.PromptRequest
	stopCalls             int
	cancelCalls           int
	processes             map[*AgentProcess]*fakeProcess
	lastProc              *fakeProcess
	promptHook            func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error)
	cancelHook            func(proc *fakeProcess) error
	cancelWithContextHook func(context.Context, *fakeProcess) error
	approveHook           func(proc *fakeProcess, req acp.ApproveRequest) error
	stopHook              func(proc *fakeProcess) error
	startHook             func(opts acp.StartOpts, sequence int) (*fakeProcess, error)
	interruptScopes       []toolruntime.InterruptScope
	interruptErr          error
	fallbackOnResume      bool
}

type fakeWorkspaceResolver struct {
	mu                     sync.Mutex
	byRef                  map[string]workspacepkg.ResolvedWorkspace
	byPath                 map[string]workspacepkg.ResolvedWorkspace
	resolveCalls           []string
	resolveOrRegisterCalls []string
	resolveErr             error
	resolveOrRegisterErr   error
	resolveHook            func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	resolveOrRegisterHook  func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	autoRegisterConfig     compozyconfig.Config
	autoRegisterAgents     []compozyconfig.AgentDef
	nextID                 int
}

type fakeSkillRegistry struct {
	mu                sync.Mutex
	skillsByWorkspace map[string][]*skillspkg.Skill
	calls             []workspacepkg.ResolvedWorkspace
	err               error
}

func newFakeWorkspaceResolver(resolved *workspacepkg.ResolvedWorkspace) *fakeWorkspaceResolver {
	r := &fakeWorkspaceResolver{
		byRef:              make(map[string]workspacepkg.ResolvedWorkspace),
		byPath:             make(map[string]workspacepkg.ResolvedWorkspace),
		autoRegisterConfig: resolved.Config,
		autoRegisterAgents: append([]compozyconfig.AgentDef(nil), resolved.Agents...),
	}
	r.upsert(resolved)
	return r
}

func newFakeSkillRegistry() *fakeSkillRegistry {
	return &fakeSkillRegistry{
		skillsByWorkspace: make(map[string][]*skillspkg.Skill),
	}
}

func newFakeSandboxRegistry(t *testing.T) *sandbox.Registry {
	t.Helper()

	registry, err := sandbox.NewRegistry(fakeSandboxProvider{})
	if err != nil {
		t.Fatalf("NewRegistry(fake sandbox) error = %v", err)
	}
	return registry
}

type fakeSandboxProvider struct{}

func (fakeSandboxProvider) Backend() sandbox.Backend {
	return sandbox.BackendLocal
}

func (fakeSandboxProvider) Prepare(
	_ context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	state := sandbox.SessionState{
		SandboxID:             req.SandboxID,
		Backend:               sandbox.BackendLocal,
		Profile:               req.Sandbox.Profile,
		InstanceID:            strings.TrimSpace(req.InstanceID),
		State:                 "prepared",
		RuntimeRootDir:        req.LocalRootDir,
		RuntimeAdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
		ProviderState:         append(json.RawMessage(nil), req.ProviderState...),
		PreparedAt:            time.Now().UTC(),
	}
	return sandbox.Prepared{
		State:                 state,
		RuntimeRootDir:        req.LocalRootDir,
		RuntimeAdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
		Launch: sandbox.LaunchSpec{
			Command:        req.AgentCommand,
			Cwd:            req.LocalRootDir,
			AdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
			Env:            append([]string(nil), req.AgentEnv...),
		},
	}, nil
}

func (fakeSandboxProvider) SyncToRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	return sandbox.SyncResult{}, nil
}

func (fakeSandboxProvider) SyncFromRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	return sandbox.SyncResult{}, nil
}

func (fakeSandboxProvider) Destroy(context.Context, sandbox.SessionState) error {
	return nil
}

func (r *fakeSkillRegistry) ForWorkspace(
	_ context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]*skillspkg.Skill, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if resolved != nil {
		r.calls = append(r.calls, cloneResolvedWorkspaceForTests(resolved))
	}
	if r.err != nil {
		return nil, r.err
	}
	if resolved == nil {
		return nil, nil
	}

	skills := r.skillsByWorkspace[resolved.ID]
	return append([]*skillspkg.Skill(nil), skills...), nil
}

func (r *fakeSkillRegistry) ForAgentDef(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	_ compozyconfig.AgentDef,
) ([]*skillspkg.Skill, error) {
	return r.ForWorkspace(ctx, resolved)
}

func (r *fakeSkillRegistry) setSkills(workspaceID string, skills []*skillspkg.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skillsByWorkspace[strings.TrimSpace(workspaceID)] = append([]*skillspkg.Skill(nil), skills...)
}

func (r *fakeSkillRegistry) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *fakeSkillRegistry) call(index int) workspacepkg.ResolvedWorkspace {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneResolvedWorkspaceForTests(&r.calls[index])
}

func (r *fakeWorkspaceResolver) Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ref := strings.TrimSpace(idOrPath)
	r.resolveCalls = append(r.resolveCalls, ref)
	if r.resolveHook != nil {
		return r.resolveHook(ctx, ref)
	}
	if r.resolveErr != nil {
		return workspacepkg.ResolvedWorkspace{}, r.resolveErr
	}
	if resolved, ok := r.byRef[ref]; ok {
		return cloneResolvedWorkspaceForTests(&resolved), nil
	}
	if resolved, ok := r.byPath[normalizeResolverPath(ref)]; ok {
		return cloneResolvedWorkspaceForTests(&resolved), nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *fakeWorkspaceResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	target := normalizeResolverPath(path)
	r.resolveOrRegisterCalls = append(r.resolveOrRegisterCalls, target)
	if r.resolveOrRegisterHook != nil {
		return r.resolveOrRegisterHook(ctx, target)
	}
	if r.resolveOrRegisterErr != nil {
		return workspacepkg.ResolvedWorkspace{}, r.resolveOrRegisterErr
	}
	if resolved, ok := r.byPath[target]; ok {
		return cloneResolvedWorkspaceForTests(&resolved), nil
	}

	r.nextID++
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      fmt.Sprintf("ws-auto-%d", r.nextID),
			RootDir: target,
			Name:    filepath.Base(target),
		},
		Config: r.autoRegisterConfig,
		Agents: append([]compozyconfig.AgentDef(nil), r.autoRegisterAgents...),
	}
	r.upsert(&resolved)
	return cloneResolvedWorkspaceForTests(&resolved), nil
}

func (r *fakeWorkspaceResolver) upsert(resolved *workspacepkg.ResolvedWorkspace) {
	cloned := cloneResolvedWorkspaceForTests(resolved)
	if strings.TrimSpace(cloned.WorkspaceID) == "" {
		cloned.WorkspaceID = strings.TrimSpace(cloned.ID)
	}
	r.byRef[cloned.ID] = cloned
	if name := strings.TrimSpace(cloned.Name); name != "" {
		r.byRef[name] = cloned
	}
	if path := normalizeResolverPath(cloned.RootDir); path != "" {
		cloned.RootDir = path
		r.byPath[path] = cloned
	}
}

func normalizeResolverPath(path string) string {
	target := strings.TrimSpace(path)
	if target == "" {
		return ""
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return filepath.Clean(target)
	}
	return filepath.Clean(absPath)
}

func cloneResolvedWorkspaceForTests(src *workspacepkg.ResolvedWorkspace) workspacepkg.ResolvedWorkspace {
	if src == nil {
		return workspacepkg.ResolvedWorkspace{}
	}
	dst := *src
	dst.AdditionalDirs = append([]string(nil), src.AdditionalDirs...)
	dst.Agents = append([]compozyconfig.AgentDef(nil), src.Agents...)
	dst.Skills = append([]workspacepkg.SkillPath(nil), src.Skills...)
	return dst
}

type recordingHostedMCPLauncher struct {
	mu       sync.Mutex
	server   compozyconfig.MCPServer
	requests []HostedMCPLaunchRequest
	arms     []string
	cancels  []string
	releases []string
}

func (l *recordingHostedMCPLauncher) ArmLaunch(_ context.Context, sessionID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.arms = append(l.arms, sessionID)
	return nil
}

var _ HostedMCPLauncher = (*recordingHostedMCPLauncher)(nil)

func (l *recordingHostedMCPLauncher) Launch(
	_ context.Context,
	req HostedMCPLaunchRequest,
) (compozyconfig.MCPServer, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, req)
	return l.server, nil
}

func (l *recordingHostedMCPLauncher) CancelLaunch(sessionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancels = append(l.cancels, sessionID)
}

func (l *recordingHostedMCPLauncher) ReleaseSession(sessionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.releases = append(l.releases, sessionID)
}

func (l *recordingHostedMCPLauncher) launchRequests() []HostedMCPLaunchRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]HostedMCPLaunchRequest(nil), l.requests...)
}

func (l *recordingHostedMCPLauncher) armedSessionIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.arms...)
}

func (l *recordingHostedMCPLauncher) releaseSessionIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.releases...)
}

func newFakeDriver() *fakeDriver {
	return &fakeDriver{
		processes: make(map[*AgentProcess]*fakeProcess),
	}
}

func (d *fakeDriver) Start(ctx context.Context, opts acp.StartOpts) (*AgentProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	copied := opts
	copied.AdditionalDirs = append([]string(nil), opts.AdditionalDirs...)
	copied.Env = append([]string(nil), opts.Env...)
	copied.MCPServers = append([]compozyconfig.MCPServer(nil), opts.MCPServers...)
	d.startCalls = append(d.startCalls, copied)
	if copied.ActivateMCPServers != nil {
		if err := copied.ActivateMCPServers(ctx); err != nil {
			return nil, err
		}
	}

	sequence := len(d.startCalls)
	var proc *fakeProcess
	var err error
	if d.startHook != nil {
		proc, err = d.startHook(copied, sequence)
	} else {
		sessionID := fmt.Sprintf("acp-%d", sequence)
		if copied.ResumeSessionID != "" {
			if d.fallbackOnResume {
				sessionID = fmt.Sprintf("acp-new-%d", sequence)
			} else {
				sessionID = copied.ResumeSessionID
			}
		}
		proc = newFakeProcess(copied.AgentName, copied.Command, copied.Cwd, sessionID)
	}
	if err != nil {
		return nil, err
	}

	proc.handle.toolHost = copied.ToolHost
	proc.handle.approvePermissionFn = func(ctx context.Context, req acp.ApproveRequest) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		d.mu.Lock()
		hook := d.approveHook
		d.mu.Unlock()

		if hook != nil {
			return hook(proc, req)
		}
		return nil
	}

	d.processes[proc.handle] = proc
	d.lastProc = proc
	return proc.handle, nil
}

func (d *fakeDriver) Prompt(
	_ context.Context,
	proc *AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	fakeProc := d.processes[proc]
	d.promptCalls = append(d.promptCalls, req)
	hook := d.promptHook
	d.mu.Unlock()

	if fakeProc == nil {
		return nil, errors.New("test: unknown fake process")
	}
	if hook != nil {
		return hook(fakeProc, req)
	}

	totalTokens := int64(9)
	events := make(chan acp.AgentEvent, 2)
	go func() {
		defer close(events)
		ts := time.Now().UTC()
		events <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: fakeProc.handle.SessionID,
			TurnID:    req.TurnID,
			Timestamp: ts,
			Text:      "reply",
		}
		events <- acp.AgentEvent{
			Type:             acp.EventTypeDone,
			SessionID:        fakeProc.handle.SessionID,
			TurnID:           req.TurnID,
			Timestamp:        ts,
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
			Usage: &acp.TokenUsage{
				TurnID:      req.TurnID,
				TotalTokens: &totalTokens,
				Timestamp:   ts,
			},
		}
	}()
	return events, nil
}

func (d *fakeDriver) Cancel(ctx context.Context, proc *AgentProcess) error {
	d.mu.Lock()
	fakeProc := d.processes[proc]
	d.cancelCalls++
	hook := d.cancelHook
	contextHook := d.cancelWithContextHook
	d.mu.Unlock()

	if fakeProc == nil {
		return errors.New("test: unknown fake process")
	}
	if contextHook != nil {
		return contextHook(ctx, fakeProc)
	}
	if hook != nil {
		return hook(fakeProc)
	}
	return nil
}

func (d *fakeDriver) Interrupt(
	_ context.Context,
	sessionID string,
	turnID string,
) (toolruntime.InterruptReport, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.interruptScopes = append(d.interruptScopes, toolruntime.InterruptScope{
		SessionID: sessionID,
		TurnID:    turnID,
	})
	if d.interruptErr != nil {
		return toolruntime.InterruptReport{}, d.interruptErr
	}
	return toolruntime.InterruptReport{Matched: 1, Signaled: 1}, nil
}

func (d *fakeDriver) Stop(_ context.Context, proc *AgentProcess) error {
	d.mu.Lock()
	fakeProc := d.processes[proc]
	d.stopCalls++
	hook := d.stopHook
	d.mu.Unlock()

	if fakeProc == nil {
		return errors.New("test: unknown fake process")
	}
	if hook != nil {
		return hook(fakeProc)
	}
	fakeProc.exit()
	return nil
}

func (d *fakeDriver) lastProcess() *fakeProcess {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastProc
}

func lookupEnvValue(env []string, key string) (string, bool) {
	for _, entry := range env {
		existingKey, value, ok := strings.Cut(entry, "=")
		if ok && existingKey == key {
			return value, true
		}
	}
	return "", false
}

type fakeProcess struct {
	mu      sync.Mutex
	done    chan struct{}
	closed  bool
	waitErr error
	stderr  string
	health  subprocess.HealthState
	handle  *AgentProcess
}

type sessionApproveCapture struct {
	SessionID string
	RequestID string
	TurnID    string
	Decision  string
}

func newFakeProcess(agentName string, command string, cwd string, sessionID string) *fakeProcess {
	proc := &fakeProcess{
		done:   make(chan struct{}),
		health: subprocess.HealthState{Healthy: true},
	}
	proc.handle = &AgentProcess{
		PID:       1,
		AgentName: agentName,
		Command:   command,
		Cwd:       cwd,
		SessionID: sessionID,
		caps: acp.Caps{
			SupportsLoadSession: true,
			SupportedModes:      []string{"chat"},
		},
		StartedAt: time.Now().UTC(),
		done:      proc.done,
		waitFn:    proc.wait,
		stderrFn:  proc.stderrOutput,
		healthStateFn: func() subprocess.HealthState {
			proc.mu.Lock()
			defer proc.mu.Unlock()
			return proc.health
		},
	}
	return proc
}

func (p *fakeProcess) wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

func (p *fakeProcess) stderrOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stderr
}

func (p *fakeProcess) setHealth(state subprocess.HealthState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health = state
}

func (p *fakeProcess) exit() {
	p.finish(nil, "")
}

func (p *fakeProcess) crash(err error, stderr string) {
	p.finish(err, stderr)
}

func (p *fakeProcess) finish(err error, stderr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.waitErr = err
	p.stderr = stderr
	if !p.closed {
		p.closed = true
		close(p.done)
	}
}

func recordResumeReplayFixture(t *testing.T, manager *Manager, session *Session, contextText string) {
	t.Helper()

	turnID := "turn-resume-replay"
	now := time.Now().UTC()
	toolInput, err := json.Marshal(map[string]string{
		"path": strings.Repeat("nested/path/", 80) + "fixture.txt",
	})
	if err != nil {
		t.Fatalf("json.Marshal(tool input) error = %v", err)
	}
	events := []acp.AgentEvent{
		{
			Type:      acp.EventTypeUserMessage,
			TurnID:    turnID,
			Timestamp: now,
			Text:      contextText,
		},
		{
			Type:      acp.EventTypeAgentMessage,
			TurnID:    turnID,
			Timestamp: now.Add(time.Millisecond),
			Text:      "Acknowledged " + contextText,
		},
		acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     turnID,
			ToolCallID: "tool-resume-replay",
			Timestamp:  now.Add(2 * time.Millisecond),
		}.WithTool("read", toolInput, false),
		acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			TurnID:     turnID,
			ToolCallID: "tool-resume-replay",
			Timestamp:  now.Add(3 * time.Millisecond),
			Text:       strings.Repeat("tool-noise-line\n", 80),
		}.WithTool("read", nil, false),
	}
	for _, event := range events {
		if err := manager.recordEvent(testutil.Context(t), session, event); err != nil {
			t.Fatalf("recordEvent(%q) error = %v", event.Type, err)
		}
	}
}

func assertResumeReplayEqualsPrunedEvents(
	t *testing.T,
	systemPrompt string,
	events []store.SessionEvent,
) {
	t.Helper()

	want, err := transcript.Assemble(events)
	if err != nil {
		t.Fatalf("transcript.Assemble(events) error = %v", err)
	}
	want = transcript.Prune(want, transcript.PruneOptions{Dedup: true})
	got := resumeReplayMessagesFromPrompt(t, systemPrompt)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resume replay = %#v, want pruned transcript %#v", got, want)
	}
}

func resumeReplayMessagesFromPrompt(t *testing.T, systemPrompt string) []transcript.Message {
	t.Helper()

	_, replayAndSuffix, ok := strings.Cut(systemPrompt, "<compozy_context_replay>\n")
	if !ok {
		t.Fatalf("system prompt is missing replay start marker: %q", systemPrompt)
	}
	replayJSON, _, ok := strings.Cut(replayAndSuffix, "\n</compozy_context_replay>")
	if !ok {
		t.Fatalf("system prompt is missing replay end marker: %q", systemPrompt)
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(replayJSON), &messages); err != nil {
		t.Fatalf("json.Unmarshal(resume replay) error = %v", err)
	}
	return messages
}

func assertContextRebuiltMarkerCount(t *testing.T, events []store.SessionEvent, want int) {
	t.Helper()

	messages, err := transcript.Assemble(events)
	if err != nil {
		t.Fatalf("transcript.Assemble(events) error = %v", err)
	}
	count := 0
	for _, message := range messages {
		if message.Role == transcript.RoleSystem && message.Content == "Context rebuilt from log." {
			count++
		}
	}
	if count != want {
		t.Fatalf("context rebuilt marker count = %d, want %d", count, want)
	}
}

func waitForCondition(t *testing.T, label string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}
