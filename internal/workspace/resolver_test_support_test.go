package workspace

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

func existingCaseAlias(path string) (string, bool) {
	for index := len(path) - 1; index >= 0; index-- {
		character := path[index]
		var replacement byte
		switch {
		case character >= 'a' && character <= 'z':
			replacement = character - ('a' - 'A')
		case character >= 'A' && character <= 'Z':
			replacement = character + ('a' - 'A')
		default:
			continue
		}
		alias := path[:index] + string([]byte{replacement}) + path[index+1:]
		if _, err := os.Stat(alias); err == nil {
			return alias, true
		}
	}
	return "", false
}

func symlinkWorkspaceRootForTest(t *testing.T, target string) string {
	t.Helper()

	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create symlink workspace root: %v", err)
	}
	return link
}

func runResolveCacheHitInvalidateAndEviction(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	workspaceConfig := filepath.Join(root, compozyconfig.DirName, compozyconfig.ConfigName)
	agentFile := filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "coder", agentDefinitionFile)
	skillsDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName)
	skillOne := filepath.Join(skillsDir, "marketing", "alpha")
	skillTwo := filepath.Join(skillsDir, "marketing", "beta")

	writeFile(t, workspaceConfig, "[http]\nport = 4242\n")
	writeAgentDef(t, agentFile, "coder", "v1")
	writeSkill(t, skillOne)

	ws := Workspace{ID: "ws_cache", RootDir: root, Name: "repo"}
	store := newMockWorkspaceStore(ws)
	loadedConfig := validConfig(homePaths)
	loadedConfig.WindowManager.HistoryLimit = 77
	loadedConfig.WindowManager.Snap.RepeatRatios = []float64{0.5, 0.75, 0.25}
	loadedConfig.WindowManager.Shortcuts = map[string]windowmanager.ShortcutBinding{
		"window.focus.left": {"alt+KeyH"},
	}
	loader := &countingConfigLoader{cfg: loadedConfig}
	currentTime := time.Unix(1_700_000_000, 0).UTC()

	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
		withNow(func() time.Time { return currentTime }),
		WithCacheTTL(10*time.Minute),
	)

	first, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(first) error = %v", err)
	}
	if got := loader.Calls(); got != 1 {
		t.Fatalf("config loader calls after first resolve = %d, want 1", got)
	}
	if first.Config.WindowManager.HistoryLimit != 77 ||
		!slices.Equal(
			first.Config.WindowManager.Shortcuts["window.focus.left"],
			windowmanager.ShortcutBinding{"alt+KeyH"},
		) {
		t.Fatalf("Resolve(first) WindowManager = %#v, want loaded config", first.Config.WindowManager)
	}
	first.Config.WindowManager.Snap.RepeatRatios[0] = 0.4
	first.Config.WindowManager.Shortcuts["window.focus.left"] = windowmanager.ShortcutBinding{"meta+KeyH"}

	currentTime = currentTime.Add(1 * time.Minute)
	second, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(second) error = %v", err)
	}
	if got := loader.Calls(); got != 1 {
		t.Fatalf("config loader calls after cache hit = %d, want 1", got)
	}
	if !second.ResolvedAt.Equal(first.ResolvedAt) {
		t.Fatalf("ResolvedAt on cache hit = %v, want %v", second.ResolvedAt, first.ResolvedAt)
	}
	if got := second.Config.WindowManager.Snap.RepeatRatios; len(got) != 3 || got[0] != 0.5 {
		t.Fatalf("Resolve(cache hit) repeat ratios = %#v, want isolated loaded ratios", got)
	}
	if got := second.Config.WindowManager.Shortcuts["window.focus.left"]; !slices.Equal(
		got,
		windowmanager.ShortcutBinding{"alt+KeyH"},
	) {
		t.Fatalf("Resolve(cache hit) shortcut = %q, want isolated alt+KeyH", got)
	}

	modTime := time.Unix(1_700_000_100, 0).UTC()
	writeFile(t, workspaceConfig, "[http]\nport = 4343\n")
	touchPath(t, workspaceConfig, modTime)
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after config change) error = %v", err)
	}
	if got := loader.Calls(); got != 2 {
		t.Fatalf("config loader calls after config invalidation = %d, want 2", got)
	}

	modTime = modTime.Add(1 * time.Minute)
	writeAgentDef(t, agentFile, "coder", "v2")
	touchPath(t, agentFile, modTime)
	currentTime = currentTime.Add(1 * time.Minute)
	afterAgent, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after agent change) error = %v", err)
	}
	if got := loader.Calls(); got != 3 {
		t.Fatalf("config loader calls after agent invalidation = %d, want 3", got)
	}
	if got := agentModel(afterAgent.Agents, "coder"); got != "v2" {
		t.Fatalf("agent model after agent invalidation = %q, want %q", got, "v2")
	}

	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(before nested skill creation) error = %v", err)
	}
	if got := loader.Calls(); got != 3 {
		t.Fatalf("config loader calls before nested skill creation = %d, want 3", got)
	}

	writeSkill(t, skillTwo)
	touchPath(t, skillsDir, modTime.Add(1*time.Minute))
	currentTime = currentTime.Add(1 * time.Minute)
	afterSkill, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after skill change) error = %v", err)
	}
	if got := loader.Calls(); got != 4 {
		t.Fatalf("config loader calls after skill invalidation = %d, want 4", got)
	}
	if got := skillNames(afterSkill.Skills); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("skill names after skill invalidation = %#v, want %#v", got, []string{"alpha", "beta"})
	}

	modTime = modTime.Add(1 * time.Minute)
	writeFile(t, filepath.Join(skillOne, skillDefinitionFile), strings.Join([]string{
		"---",
		"name: alpha",
		"description: updated test skill",
		"---",
		"",
		"Updated skill body.",
		"",
	}, "\n"))
	touchPath(t, filepath.Join(skillOne, skillDefinitionFile), modTime)
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after nested skill definition change) error = %v", err)
	}
	if got := loader.Calls(); got != 5 {
		t.Fatalf("config loader calls after nested skill definition invalidation = %d, want 5", got)
	}

	modTime = modTime.Add(1 * time.Minute)
	mcpPath := filepath.Join(skillTwo, compozyconfig.MCPJSONName)
	writeFile(t, mcpPath, `{"mcpServers":{}}`)
	touchPath(t, mcpPath, modTime)
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after nested skill MCP change) error = %v", err)
	}
	if got := loader.Calls(); got != 6 {
		t.Fatalf("config loader calls after nested skill MCP invalidation = %d, want 6", got)
	}

	if err := os.Remove(filepath.Join(skillTwo, skillDefinitionFile)); err != nil {
		t.Fatalf("Remove(nested skill definition) error = %v", err)
	}
	currentTime = currentTime.Add(1 * time.Minute)
	afterSkillRemoval, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after nested skill removal) error = %v", err)
	}
	if got := loader.Calls(); got != 7 {
		t.Fatalf("config loader calls after nested skill removal invalidation = %d, want 7", got)
	}
	if got := skillNames(afterSkillRemoval.Skills); !slices.Equal(got, []string{"alpha"}) {
		t.Fatalf("skill names after nested skill removal = %#v, want %#v", got, []string{"alpha"})
	}

	resolver.Invalidate(ws.ID)
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after invalidate) error = %v", err)
	}
	if got := loader.Calls(); got != 8 {
		t.Fatalf("config loader calls after Invalidate = %d, want 8", got)
	}

	resolver.InvalidateAll()
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after InvalidateAll) error = %v", err)
	}
	if got := loader.Calls(); got != 9 {
		t.Fatalf("config loader calls after InvalidateAll = %d, want 9", got)
	}

	currentTime = currentTime.Add(11 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after TTL expiry) error = %v", err)
	}
	if got := loader.Calls(); got != 10 {
		t.Fatalf("config loader calls after TTL eviction = %d, want 10", got)
	}
}

type recordingUnregisterPreparation struct {
	beforeDeletes int
	commits       int
	rollbacks     int
	staged        bool
	committed     bool
	commitErr     error
	order         *[]string
}

func (p *recordingUnregisterPreparation) BeforeDelete(context.Context) error {
	p.beforeDeletes++
	p.staged = true
	if p.order != nil {
		*p.order = append(*p.order, "before_delete")
	}
	return nil
}

func (p *recordingUnregisterPreparation) Commit(context.Context) error {
	p.commits++
	if p.commitErr != nil {
		return p.commitErr
	}
	p.staged = false
	p.committed = true
	if p.order != nil {
		*p.order = append(*p.order, "commit")
	}
	return nil
}

func (p *recordingUnregisterPreparation) Rollback(context.Context) error {
	p.rollbacks++
	p.staged = false
	if p.order != nil {
		*p.order = append(*p.order, "rollback")
	}
	return nil
}

func removedWorkspaceRoot(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "removed-workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", root, err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatalf("Remove(%q) error = %v", root, err)
	}
	return root
}

type mockWorkspaceStore struct {
	mu sync.Mutex

	workspaces  map[string]Workspace
	intents     map[string]DeletionIntent
	deleteErr   error
	completeErr error
	deleteHook  func()

	insertCalls       []Workspace
	updateCalls       []Workspace
	deleteCalls       []string
	getWorkspaceCalls []string
	getByPathCalls    []string
	getByNameCalls    []string
	listCalls         int
}

type nameCollisionOnceStore struct {
	*mockWorkspaceStore
	collisionPending bool
}

func newMockWorkspaceStore(workspaces ...Workspace) *mockWorkspaceStore {
	store := &mockWorkspaceStore{
		workspaces: make(map[string]Workspace, len(workspaces)),
		intents:    make(map[string]DeletionIntent),
	}
	for _, ws := range workspaces {
		store.workspaces[ws.ID] = cloneWorkspace(ws)
	}
	return store
}

func (s *nameCollisionOnceStore) InsertWorkspace(ctx context.Context, ws Workspace) error {
	s.mu.Lock()
	if s.collisionPending {
		s.collisionPending = false
		s.insertCalls = append(s.insertCalls, cloneWorkspace(ws))
		competitor := Workspace{
			ID:        "ws_competitor",
			RootDir:   ws.RootDir + "-competitor",
			Name:      ws.Name,
			CreatedAt: ws.CreatedAt,
			UpdatedAt: ws.UpdatedAt,
		}
		s.workspaces[competitor.ID] = cloneWorkspace(competitor)
		s.mu.Unlock()
		return ErrWorkspaceNameTaken
	}
	s.mu.Unlock()

	return s.mockWorkspaceStore.InsertWorkspace(ctx, ws)
}

func (m *mockWorkspaceStore) InsertWorkspace(_ context.Context, ws Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.insertCalls = append(m.insertCalls, cloneWorkspace(ws))
	for _, existing := range m.workspaces {
		switch {
		case existing.RootDir == ws.RootDir:
			return ErrWorkspacePathTaken
		case existing.Name == ws.Name:
			return ErrWorkspaceNameTaken
		}
	}
	m.workspaces[ws.ID] = cloneWorkspace(ws)
	return nil
}

func (m *mockWorkspaceStore) UpdateWorkspace(_ context.Context, ws Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateCalls = append(m.updateCalls, cloneWorkspace(ws))
	if _, ok := m.workspaces[ws.ID]; !ok {
		return ErrWorkspaceNotFound
	}
	for _, existing := range m.workspaces {
		if existing.ID == ws.ID {
			continue
		}
		switch {
		case existing.RootDir == ws.RootDir:
			return ErrWorkspacePathTaken
		case existing.Name == ws.Name:
			return ErrWorkspaceNameTaken
		}
	}
	m.workspaces[ws.ID] = cloneWorkspace(ws)
	return nil
}

func (m *mockWorkspaceStore) DeleteWorkspace(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCalls = append(m.deleteCalls, id)
	if m.deleteHook != nil {
		m.deleteHook()
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.workspaces[id]; !ok {
		return ErrWorkspaceNotFound
	}
	delete(m.workspaces, id)
	return nil
}

func (m *mockWorkspaceStore) StageWorkspaceDeletion(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCalls = append(m.deleteCalls, id)
	if m.deleteHook != nil {
		m.deleteHook()
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	ws, ok := m.workspaces[id]
	if !ok {
		return ErrWorkspaceNotFound
	}
	m.intents[id] = DeletionIntent{
		Workspace: cloneWorkspace(ws), RequestedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	delete(m.workspaces, id)
	return nil
}

func (m *mockWorkspaceStore) GetWorkspaceDeletionIntent(
	_ context.Context,
	id string,
) (DeletionIntent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent, ok := m.intents[id]
	if !ok {
		return DeletionIntent{}, ErrWorkspaceDeletionIntentNotFound
	}
	intent.Workspace = cloneWorkspace(intent.Workspace)
	return intent, nil
}

func (m *mockWorkspaceStore) ListWorkspaceDeletionIntents(context.Context) ([]DeletionIntent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	intents := make([]DeletionIntent, 0, len(m.intents))
	for _, intent := range m.intents {
		intent.Workspace = cloneWorkspace(intent.Workspace)
		intents = append(intents, intent)
	}
	slices.SortFunc(intents, func(left, right DeletionIntent) int {
		return strings.Compare(left.Workspace.ID, right.Workspace.ID)
	})
	return intents, nil
}

func (m *mockWorkspaceStore) CompleteWorkspaceDeletion(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.completeErr != nil {
		return m.completeErr
	}
	if _, ok := m.intents[id]; !ok {
		return ErrWorkspaceDeletionIntentNotFound
	}
	delete(m.intents, id)
	return nil
}

func (m *mockWorkspaceStore) GetWorkspace(_ context.Context, id string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getWorkspaceCalls = append(m.getWorkspaceCalls, id)
	ws, ok := m.workspaces[id]
	if !ok {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return cloneWorkspace(ws), nil
}

func (m *mockWorkspaceStore) GetWorkspaceByPath(_ context.Context, rootDir string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getByPathCalls = append(m.getByPathCalls, rootDir)
	for _, ws := range m.workspaces {
		if ws.RootDir == rootDir {
			return cloneWorkspace(ws), nil
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

func (m *mockWorkspaceStore) GetWorkspaceByName(_ context.Context, name string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getByNameCalls = append(m.getByNameCalls, name)
	for _, ws := range m.workspaces {
		if ws.Name == name {
			return cloneWorkspace(ws), nil
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

func (m *mockWorkspaceStore) ListWorkspaces(_ context.Context) ([]Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listCalls++
	workspaces := make([]Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		workspaces = append(workspaces, cloneWorkspace(ws))
	}
	slices.SortFunc(workspaces, func(left, right Workspace) int {
		if compare := strings.Compare(left.Name, right.Name); compare != 0 {
			return compare
		}
		return strings.Compare(left.ID, right.ID)
	})
	return workspaces, nil
}

func (m *mockWorkspaceStore) mustWorkspace(id string) Workspace {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.workspaces[id]
	if !ok {
		panic("workspace not found: " + id)
	}
	return cloneWorkspace(ws)
}

type countingConfigLoader struct {
	mu    sync.Mutex
	cfg   compozyconfig.Config
	calls int
	roots []string
}

type concurrentPathStore struct {
	existing       Workspace
	getByPathCalls int
}

type serializedRegistrationStore struct {
	*mockWorkspaceStore
	mu               sync.Mutex
	listCalls        int
	firstListStarted chan struct{}
	releaseFirstList chan struct{}
	inserted         chan string
}

func newSerializedRegistrationStore() *serializedRegistrationStore {
	return &serializedRegistrationStore{
		mockWorkspaceStore: newMockWorkspaceStore(),
		firstListStarted:   make(chan struct{}),
		releaseFirstList:   make(chan struct{}),
		inserted:           make(chan string, 2),
	}
}

func (s *serializedRegistrationStore) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	s.mu.Lock()
	s.listCalls++
	first := s.listCalls == 1
	s.mu.Unlock()

	workspaces, err := s.mockWorkspaceStore.ListWorkspaces(ctx)
	if err != nil || !first {
		return workspaces, err
	}
	close(s.firstListStarted)
	select {
	case <-s.releaseFirstList:
		return workspaces, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *serializedRegistrationStore) InsertWorkspace(ctx context.Context, ws Workspace) error {
	if err := s.mockWorkspaceStore.InsertWorkspace(ctx, ws); err != nil {
		return err
	}
	s.inserted <- ws.RootDir
	return nil
}

type concurrentUnregisterStore struct {
	*mockWorkspaceStore
}

func (s *concurrentUnregisterStore) DeleteWorkspace(ctx context.Context, id string) error {
	if err := s.mockWorkspaceStore.DeleteWorkspace(ctx, id); err != nil {
		return err
	}
	return ErrWorkspaceNotFound
}

func (s *concurrentUnregisterStore) StageWorkspaceDeletion(ctx context.Context, id string) error {
	if err := s.mockWorkspaceStore.StageWorkspaceDeletion(ctx, id); err != nil {
		return err
	}
	return ErrWorkspaceNotFound
}

type cancelOnInsertStore struct {
	*mockWorkspaceStore
	cancel context.CancelFunc

	deleteCtxErr      error
	deleteHasDeadline bool
}

func (s *concurrentPathStore) InsertWorkspace(context.Context, Workspace) error {
	return ErrWorkspacePathTaken
}

func (s *concurrentPathStore) UpdateWorkspace(context.Context, Workspace) error {
	return nil
}

func (s *concurrentPathStore) DeleteWorkspace(context.Context, string) error {
	return nil
}

func (s *concurrentPathStore) StageWorkspaceDeletion(context.Context, string) error {
	return nil
}

func (s *concurrentPathStore) GetWorkspaceDeletionIntent(
	context.Context,
	string,
) (DeletionIntent, error) {
	return DeletionIntent{}, ErrWorkspaceDeletionIntentNotFound
}

func (s *concurrentPathStore) ListWorkspaceDeletionIntents(context.Context) ([]DeletionIntent, error) {
	return nil, nil
}

func (s *concurrentPathStore) CompleteWorkspaceDeletion(context.Context, string) error {
	return nil
}

func (s *concurrentPathStore) GetWorkspace(_ context.Context, id string) (Workspace, error) {
	if id != s.existing.ID {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return cloneWorkspace(s.existing), nil
}

func (s *concurrentPathStore) GetWorkspaceByPath(_ context.Context, rootDir string) (Workspace, error) {
	s.getByPathCalls++
	if s.getByPathCalls == 1 {
		return Workspace{}, ErrWorkspaceNotFound
	}
	if rootDir != s.existing.RootDir {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return cloneWorkspace(s.existing), nil
}

func (s *concurrentPathStore) GetWorkspaceByName(context.Context, string) (Workspace, error) {
	return Workspace{}, ErrWorkspaceNotFound
}

func (s *concurrentPathStore) ListWorkspaces(context.Context) ([]Workspace, error) {
	return nil, nil
}

func (s *cancelOnInsertStore) InsertWorkspace(ctx context.Context, ws Workspace) error {
	if err := s.mockWorkspaceStore.InsertWorkspace(ctx, ws); err != nil {
		return err
	}
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *cancelOnInsertStore) DeleteWorkspace(ctx context.Context, id string) error {
	s.deleteCtxErr = ctx.Err()
	_, s.deleteHasDeadline = ctx.Deadline()
	return s.mockWorkspaceStore.DeleteWorkspace(ctx, id)
}

func (l *countingConfigLoader) Load(root string) (compozyconfig.Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	l.roots = append(l.roots, root)
	return cloneConfig(&l.cfg), nil
}

func (l *countingConfigLoader) Calls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func (l *countingConfigLoader) LastRoot() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.roots) == 0 {
		return ""
	}
	return l.roots[len(l.roots)-1]
}

func newTestResolver(tb testing.TB, store Store, opts ...Option) *Resolver {
	tb.Helper()

	opts = append([]Option{WithLogger(discardLogger())}, opts...)
	resolver, err := NewResolver(store, opts...)
	if err != nil {
		tb.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func newTestHomePaths(tb testing.TB) compozyconfig.HomePaths {
	tb.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(tb.TempDir(), "home"))
	if err != nil {
		tb.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		tb.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func mustCanonicalRoot(t *testing.T, path string) string {
	t.Helper()

	root, err := canonicalRoot(path)
	if err != nil {
		t.Fatalf("canonicalRoot(%q) error = %v", path, err)
	}
	return root
}

func validConfig(homePaths compozyconfig.HomePaths) compozyconfig.Config {
	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Port = 2123
	return cfg
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeAgentDef(tb testing.TB, path string, name string, model string) {
	tb.Helper()
	writeFile(tb, path, strings.Join([]string{
		"---",
		"name: " + name,
		"provider: claude",
		"model: " + model,
		"---",
		"",
		"Prompt for " + name + ".",
		"",
	}, "\n"))
}

func writeSkill(tb testing.TB, dir string) {
	tb.Helper()
	writeFile(tb, filepath.Join(dir, skillDefinitionFile), strings.Join([]string{
		"---",
		"name: " + filepath.Base(dir),
		"description: test skill",
		"---",
		"",
		"Skill body.",
		"",
	}, "\n"))
}

func writeFile(tb testing.TB, path string, contents string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		tb.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func touchPath(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", path, err)
	}
}

func createSymlink(t *testing.T, target string, link string) {
	t.Helper()
	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Remove(%q) error = %v", link, err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%q -> %q) error = %v", link, target, err)
	}
}

func agentModel(agents []compozyconfig.AgentDef, name string) string {
	for _, agent := range agents {
		if agent.Name == name {
			return agent.Model
		}
	}
	return ""
}

func skillNames(skills []SkillPath) []string {
	if len(skills) == 0 {
		return nil
	}

	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		name := skill.Name
		if name == "" {
			name = filepath.Base(skill.Dir)
		}
		names = append(names, name)
	}
	return names
}

func skillPathSourceByName(skills []SkillPath, name string) string {
	for _, skill := range skills {
		if skill.Name == name {
			return skill.Source
		}
	}
	return ""
}

func nilTestContext() context.Context {
	return nil
}

func assertResolveCancellationRollback(
	t *testing.T,
	workspaceID string,
	run func(context.Context, *Resolver, string) error,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	store := &cancelOnInsertStore{
		mockWorkspaceStore: newMockWorkspaceStore(),
		cancel:             cancel,
	}

	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithIDGenerator(func(string) (string, error) { return workspaceID, nil }),
	)

	if err := run(ctx, resolver, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want %v", err, context.Canceled)
	}
	if got := len(store.deleteCalls); got != 1 {
		t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
	}
	if store.deleteCtxErr != nil {
		t.Fatalf("DeleteWorkspace() context error = %v, want nil", store.deleteCtxErr)
	}
	if !store.deleteHasDeadline {
		t.Fatal("DeleteWorkspace() context deadline missing, want bounded rollback timeout")
	}
	if _, err := store.GetWorkspace(context.Background(), workspaceID); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("GetWorkspace(rolled back after cancellation) error = %v, want %v", err, ErrWorkspaceNotFound)
	}
}
