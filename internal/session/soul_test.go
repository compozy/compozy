package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestManagerSoulSessionSnapshots(t *testing.T) {
	t.Parallel()

	t.Run("Should snapshot valid soul on session start", func(t *testing.T) {
		t.Parallel()

		soulStore := newFakeSoulSnapshotStore()
		h := newHarness(t, WithSoulSnapshotStore(soulStore))
		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Lead with clarity."))

		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)

		info := session.Info()
		if info.SoulSnapshotID == "" || info.SoulDigest == "" {
			t.Fatalf(
				"session soul fields = snapshot %q digest %q, want populated",
				info.SoulSnapshotID,
				info.SoulDigest,
			)
		}
		stored, ok := soulStore.snapshot(info.SoulSnapshotID)
		if !ok {
			t.Fatalf("snapshot %q was not persisted", info.SoulSnapshotID)
		}
		if stored.Digest != info.SoulDigest || stored.AgentName != "coder" || stored.WorkspaceID != h.workspaceID {
			t.Fatalf("stored snapshot = %#v, want session digest/agent/workspace", stored)
		}
		meta := readMeta(t, session.MetaPath())
		if meta.SoulSnapshotID != info.SoulSnapshotID || meta.SoulDigest != info.SoulDigest {
			t.Fatalf("persisted meta soul = snapshot %q digest %q, want %q/%q",
				meta.SoulSnapshotID,
				meta.SoulDigest,
				info.SoulSnapshotID,
				info.SoulDigest,
			)
		}
	})

	t.Run("Should reject invalid soul on session start before driver starts", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithSoulSnapshotStore(newFakeSoulSnapshotStore()))
		writeSessionSoul(t, h.workspace, "coder", invalidSessionSoul())

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "invalid soul",
			Workspace: h.workspaceID,
		})
		if !errors.Is(err, soul.ErrInvalid) {
			t.Fatalf("Create() error = %v, want %v", err, soul.ErrInvalid)
		}
		if len(h.driver.startCalls) != 0 {
			t.Fatalf("driver start calls = %d, want 0 after invalid soul", len(h.driver.startCalls))
		}
	})

	t.Run("Should apply default soul config when workspace config omits agents soul", func(t *testing.T) {
		t.Parallel()

		soulStore := newFakeSoulSnapshotStore()
		h := newHarness(t, WithSoulSnapshotStore(soulStore))
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		resolved.Config = aghconfig.Config{
			Defaults: aghconfig.DefaultsConfig{Agent: "coder"},
		}
		h.resolver.upsert(&resolved)
		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Defaulted body."))

		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)

		info := session.Info()
		if info.SoulSnapshotID == "" || info.SoulDigest == "" {
			t.Fatalf(
				"session soul fields = snapshot %q digest %q, want default-backed snapshot",
				info.SoulSnapshotID,
				info.SoulDigest,
			)
		}
		if _, ok := soulStore.snapshot(info.SoulSnapshotID); !ok {
			t.Fatalf("snapshot %q was not persisted", info.SoulSnapshotID)
		}
	})

	t.Run("Should resolve global agent soul from home root", func(t *testing.T) {
		t.Parallel()

		soulStore := newFakeSoulSnapshotStore()
		h := newHarness(t, WithSoulSnapshotStore(soulStore))
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		agentPath := filepath.Join(h.homePaths.AgentsDir, "coder", "AGENT.md")
		if err := os.MkdirAll(filepath.Dir(agentPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(global agent dir) error = %v", err)
		}
		if err := os.WriteFile(
			agentPath,
			[]byte("---\nname: coder\nprovider: claude\n---\nGlobal prompt.\n"),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile(global agent) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(filepath.Dir(agentPath), soul.FileName),
			[]byte(validSessionSoul("Global Reviewer", "Use the global agent soul.")),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile(global soul) error = %v", err)
		}
		resolved.Agents = []aghconfig.AgentDef{{
			Name:       "coder",
			Provider:   "claude",
			Prompt:     "Global prompt.",
			SourcePath: agentPath,
		}}
		h.resolver.upsert(&resolved)

		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)

		info := session.Info()
		if info.SoulSnapshotID == "" || info.SoulDigest == "" {
			t.Fatalf(
				"session soul fields = snapshot %q digest %q, want global-home snapshot",
				info.SoulSnapshotID,
				info.SoulDigest,
			)
		}
		stored, ok := soulStore.snapshot(info.SoulSnapshotID)
		if !ok {
			t.Fatalf("snapshot %q was not persisted", info.SoulSnapshotID)
		}
		if got, want := stored.SourcePath, "agents/coder/SOUL.md"; got != want {
			t.Fatalf("stored.SourcePath = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve soul provenance across resume", func(t *testing.T) {
		t.Parallel()

		soulStore := newFakeSoulSnapshotStore()
		h := newHarness(t, WithSoulSnapshotStore(soulStore))
		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Persistent body."))
		session := createSession(t, h)
		original := session.Info()
		if original.SoulSnapshotID == "" || original.SoulDigest == "" {
			t.Fatalf(
				"original soul fields = snapshot %q digest %q, want populated",
				original.SoulSnapshotID,
				original.SoulDigest,
			)
		}
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop(%s) error = %v", session.ID, err)
		}

		h.manager = newManagerWithHarness(t, h, WithSoulSnapshotStore(soulStore))
		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(%s) error = %v", session.ID, err)
		}
		cleanupSessionStop(t, h, resumed.ID)

		info := resumed.Info()
		if info.SoulSnapshotID != original.SoulSnapshotID || info.SoulDigest != original.SoulDigest {
			t.Fatalf(
				"resumed soul fields = snapshot %q digest %q, want %q/%q",
				info.SoulSnapshotID,
				info.SoulDigest,
				original.SoulSnapshotID,
				original.SoulDigest,
			)
		}
		meta := readMeta(t, resumed.MetaPath())
		if meta.SoulSnapshotID != original.SoulSnapshotID || meta.SoulDigest != original.SoulDigest {
			t.Fatalf(
				"resumed meta soul fields = snapshot %q digest %q, want %q/%q",
				meta.SoulSnapshotID,
				meta.SoulDigest,
				original.SoulSnapshotID,
				original.SoulDigest,
			)
		}
	})
}

func TestManagerRefreshSoul(t *testing.T) {
	t.Parallel()

	t.Run("Should update session snapshot on explicit refresh", func(t *testing.T) {
		t.Parallel()

		soulStore := newFakeSoulSnapshotStore()
		h := newHarness(t, WithSoulSnapshotStore(soulStore))
		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Initial body."))
		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)
		initial := session.Info()

		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Refreshed body."))
		result, err := h.manager.RefreshSoul(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("RefreshSoul() error = %v", err)
		}

		info := session.Info()
		if result.SoulSnapshotID == "" || result.SoulDigest == "" {
			t.Fatalf("RefreshSoul() result = %#v, want snapshot provenance", result)
		}
		if result.SoulDigest == initial.SoulDigest {
			t.Fatalf("RefreshSoul() digest = %q, want changed from %q", result.SoulDigest, initial.SoulDigest)
		}
		if info.SoulSnapshotID != result.SoulSnapshotID || info.SoulDigest != result.SoulDigest {
			t.Fatalf("session info soul = %q/%q, want refresh result %q/%q",
				info.SoulSnapshotID,
				info.SoulDigest,
				result.SoulSnapshotID,
				result.SoulDigest,
			)
		}
		updates := soulStore.updatesList()
		if len(updates) != 1 ||
			updates[0].ID != session.ID ||
			updates[0].SoulSnapshotID != result.SoulSnapshotID ||
			updates[0].SoulDigest != result.SoulDigest {
			t.Fatalf("UpdateSessionSoulSnapshot calls = %#v, want refreshed session provenance", updates)
		}
		meta := readMeta(t, session.MetaPath())
		if meta.SoulSnapshotID != result.SoulSnapshotID || meta.SoulDigest != result.SoulDigest {
			t.Fatalf("persisted meta soul = %q/%q, want %q/%q",
				meta.SoulSnapshotID,
				meta.SoulDigest,
				result.SoulSnapshotID,
				result.SoulDigest,
			)
		}
	})

	t.Run("Should reject refresh while session soul lock is busy", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithSoulSnapshotStore(newFakeSoulSnapshotStore()))
		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)
		release, ok := h.manager.tryAcquireSoulLock(session.ID)
		if !ok {
			t.Fatal("tryAcquireSoulLock() ok = false, want true for test setup")
		}
		defer release()

		_, err := h.manager.RefreshSoul(testutil.Context(t), session.ID)
		if !errors.Is(err, ErrSoulRefreshConflict) {
			t.Fatalf("RefreshSoul() error = %v, want %v", err, ErrSoulRefreshConflict)
		}
	})

	t.Run("Should reject refresh while session has active task run", func(t *testing.T) {
		t.Parallel()

		h := newHarness(
			t,
			WithSoulSnapshotStore(newFakeSoulSnapshotStore()),
			WithSoulRunActivityChecker(fakeSoulRunActivityChecker{active: true}),
		)
		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)

		_, err := h.manager.RefreshSoul(testutil.Context(t), session.ID)
		if !errors.Is(err, ErrSoulRefreshConflict) {
			t.Fatalf("RefreshSoul() error = %v, want %v", err, ErrSoulRefreshConflict)
		}
	})

	t.Run("Should fail closed on invalid refreshed soul and preserve prior snapshot", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithSoulSnapshotStore(newFakeSoulSnapshotStore()))
		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Initial body."))
		session := createSession(t, h)
		cleanupSessionStop(t, h, session.ID)
		initial := session.Info()

		writeSessionSoul(t, h.workspace, "coder", invalidSessionSoul())
		_, err := h.manager.RefreshSoul(testutil.Context(t), session.ID)
		if !errors.Is(err, soul.ErrInvalid) {
			t.Fatalf("RefreshSoul() error = %v, want %v", err, soul.ErrInvalid)
		}
		info := session.Info()
		if info.SoulSnapshotID != initial.SoulSnapshotID || info.SoulDigest != initial.SoulDigest {
			t.Fatalf("session soul after failed refresh = %q/%q, want preserved %q/%q",
				info.SoulSnapshotID,
				info.SoulDigest,
				initial.SoulSnapshotID,
				initial.SoulDigest,
			)
		}
	})
}

func TestManagerSpawnSoulLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should record parent soul digest without implicit inheritance", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithSoulSnapshotStore(newFakeSoulSnapshotStore()))
		writeSessionSoul(t, h.workspace, "coder", validSessionSoul("Reviewer", "Parent body."))
		addHarnessAgent(t, h, aghconfig.AgentDef{
			Name:     "reviewer",
			Provider: "claude",
			Prompt:   "Review code.",
		})
		parentPolicy := store.SessionPermissionPolicy{
			Tools:           []string{testToolEdit, testToolRead},
			Skills:          []string{"go", "tests"},
			MCPServers:      []string{"filesystem"},
			WorkspacePaths:  []string{h.workspace},
			NetworkChannels: []string{"builders"},
			SandboxProfiles: []string{"default"},
		}
		parent := createSpawnParent(t, h, parentPolicy, store.SessionSpawnBudget{
			MaxChildren:           2,
			MaxDepth:              1,
			MaxActivePerWorkspace: 2,
		})
		cleanupSessionStop(t, h, parent.ID)
		parentInfo := parent.Info()
		if parentInfo.SoulDigest == "" {
			t.Fatal("parent SoulDigest = empty, want parent snapshot")
		}

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "reviewer",
			Name:            "child reviewer",
			PromptOverlay:   "Use explicit spawn overlay only.",
			TTL:             30 * time.Minute,
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools:           []string{testToolRead},
				Skills:          []string{"go"},
				MCPServers:      []string{"filesystem"},
				WorkspacePaths:  []string{h.workspace},
				NetworkChannels: []string{"builders"},
				SandboxProfiles: []string{"default"},
			},
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)

		childInfo := child.Info()
		if childInfo.ParentSoulDigest != parentInfo.SoulDigest {
			t.Fatalf("child ParentSoulDigest = %q, want %q", childInfo.ParentSoulDigest, parentInfo.SoulDigest)
		}
		if childInfo.SoulDigest != "" || childInfo.SoulSnapshotID != "" {
			t.Fatalf(
				"child inherited soul = snapshot %q digest %q, want empty own snapshot",
				childInfo.SoulSnapshotID,
				childInfo.SoulDigest,
			)
		}
		meta := readMeta(t, child.MetaPath())
		if meta.ParentSoulDigest != parentInfo.SoulDigest {
			t.Fatalf("persisted ParentSoulDigest = %q, want %q", meta.ParentSoulDigest, parentInfo.SoulDigest)
		}
	})
}

type fakeSoulSnapshotStore struct {
	mu        sync.Mutex
	snapshots map[string]soul.Snapshot
	updates   []store.SessionSoulSnapshotUpdate
}

func newFakeSoulSnapshotStore() *fakeSoulSnapshotStore {
	return &fakeSoulSnapshotStore{snapshots: make(map[string]soul.Snapshot)}
}

func (s *fakeSoulSnapshotStore) UpsertSoulSnapshot(_ context.Context, snapshot soul.Snapshot) (soul.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := snapshot.Normalize()
	if err := normalized.Validate(); err != nil {
		return soul.Snapshot{}, err
	}
	s.snapshots[normalized.ID] = normalized
	return normalized, nil
}

func (s *fakeSoulSnapshotStore) GetSoulSnapshot(_ context.Context, id string) (soul.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[id]
	if !ok {
		return soul.Snapshot{}, soul.ErrSnapshotNotFound
	}
	return snapshot, nil
}

func (s *fakeSoulSnapshotStore) UpdateSessionSoulSnapshot(
	_ context.Context,
	update store.SessionSoulSnapshotUpdate,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.updates = append(s.updates, update)
	return nil
}

func (s *fakeSoulSnapshotStore) snapshot(id string) (soul.Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[id]
	return snapshot, ok
}

func (s *fakeSoulSnapshotStore) updatesList() []store.SessionSoulSnapshotUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]store.SessionSoulSnapshotUpdate(nil), s.updates...)
}

type fakeSoulRunActivityChecker struct {
	active bool
	err    error
}

func (f fakeSoulRunActivityChecker) HasActiveRunForSession(context.Context, string, time.Time) (bool, error) {
	return f.active, f.err
}

func writeSessionSoul(t *testing.T, workspaceRoot string, agentName string, content string) {
	t.Helper()

	soulDir := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.AgentsDirName, agentName)
	if err := os.MkdirAll(soulDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", soulDir, err)
	}
	soulPath := filepath.Join(soulDir, soul.FileName)
	if err := os.WriteFile(soulPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", soulPath, err)
	}
}

func validSessionSoul(role string, body string) string {
	return "---\nrole: " + role + "\ntone:\n  - direct\nprinciples:\n  - protect correctness\n---\n" + body + "\n"
}

func invalidSessionSoul() string {
	return "---\nprovider: claude\n---\nThis invalid file must fail closed.\n"
}

func addHarnessAgent(t *testing.T, h *harness, agent aghconfig.AgentDef) {
	t.Helper()

	resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
	}
	resolved.Agents = append(resolved.Agents, agent)
	h.resolver.upsert(&resolved)
}
