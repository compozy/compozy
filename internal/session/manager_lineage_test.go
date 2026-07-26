package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestCreateManualSessionProducesRootLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should produce root lineage for manual sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Fatalf("Stop(session) error = %v", err)
			}
		})

		info := sess.Info()
		if info.Lineage == nil {
			t.Fatal("session.Info().Lineage = nil, want root lineage")
		}
		if info.Lineage.ParentSessionID != "" ||
			info.Lineage.RootSessionID != sess.ID ||
			info.Lineage.SpawnDepth != 0 ||
			info.Lineage.AutoStopOnParent {
			t.Fatalf("root lineage = %#v, want no parent, own root, depth 0", info.Lineage)
		}

		meta := readMeta(t, sess.MetaPath())
		if meta.Lineage == nil {
			t.Fatal("meta.Lineage = nil, want persisted root lineage")
		}
		if got, want := meta.Lineage.RootSessionID, sess.ID; got != want {
			t.Fatalf("meta.Lineage.RootSessionID = %q, want %q", got, want)
		}
	})
}

func TestCreateAllowedToolsOverrideNarrowsAgentProfile(t *testing.T) {
	t.Parallel()

	t.Run("Should persist narrowed tool policy for a subset override", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		addHarnessAgent(t, h, aghconfig.AgentDef{
			Name:     "tool-coder",
			Provider: "claude",
			Prompt:   "Use the available tools.",
			Tools: []string{
				toolspkg.ToolIDTaskRead.String(),
				toolspkg.ToolIDTaskUpdate.String(),
			},
		})

		sess, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "tool-coder",
			Workspace: h.workspaceID,
			AllowedToolsOverride: []string{
				toolspkg.ToolIDTaskRead.String(),
			},
		})
		if err != nil {
			t.Fatalf("Create(narrowed tools) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Fatalf("Stop(session) error = %v", err)
			}
		})

		info := sess.Info()
		if info.Lineage == nil {
			t.Fatal("session lineage = nil, want narrowed policy")
		}
		wantTools := []string{toolspkg.ToolIDTaskRead.String()}
		if got := info.Lineage.PermissionPolicy.Tools; !testutil.EqualStringSlices(got, wantTools) {
			t.Fatalf("lineage tools = %#v, want %#v", got, wantTools)
		}
		meta := readMeta(t, sess.MetaPath())
		if meta.Lineage == nil {
			t.Fatal("meta lineage = nil, want persisted narrowed policy")
		}
		if got := meta.Lineage.PermissionPolicy.Tools; !testutil.EqualStringSlices(got, wantTools) {
			t.Fatalf("persisted lineage tools = %#v, want %#v", got, wantTools)
		}
	})

	t.Run("Should reject requested tools outside the agent profile", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		addHarnessAgent(t, h, aghconfig.AgentDef{
			Name:     "read-only-coder",
			Provider: "claude",
			Prompt:   "Use read-only tools.",
			Tools:    []string{toolspkg.ToolIDTaskRead.String()},
		})

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "read-only-coder",
			Workspace: h.workspaceID,
			AllowedToolsOverride: []string{
				toolspkg.ToolIDTaskRead.String(),
				toolspkg.ToolIDTaskUpdate.String(),
			},
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Create(widening tools) error = %v, want %v", err, ErrValidation)
		}
		if !strings.Contains(err.Error(), toolspkg.ToolIDTaskUpdate.String()) ||
			!strings.Contains(err.Error(), "widens agent profile") {
			t.Fatalf("Create(widening tools) error = %v, want deterministic widening message", err)
		}
	})

	t.Run("Should leave profile unchanged for nil and empty overrides", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			override []string
		}{
			{name: "Should leave nil override unchanged"},
			{name: "Should leave empty override unchanged", override: []string{}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				h := newHarness(t)
				addHarnessAgent(t, h, aghconfig.AgentDef{
					Name:     "unchanged-coder",
					Provider: "claude",
					Prompt:   "Use the default tool profile.",
					Tools: []string{
						toolspkg.ToolIDTaskRead.String(),
						toolspkg.ToolIDTaskUpdate.String(),
					},
				})

				sess, err := h.manager.Create(testutil.Context(t), CreateOpts{
					AgentName:            "unchanged-coder",
					Workspace:            h.workspaceID,
					AllowedToolsOverride: tt.override,
				})
				if err != nil {
					t.Fatalf("Create(%s) error = %v", tt.name, err)
				}
				t.Cleanup(func() {
					if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
						t.Fatalf("Stop(session) error = %v", err)
					}
				})

				info := sess.Info()
				if info.Lineage == nil {
					t.Fatal("session lineage = nil, want root lineage")
				}
				if got := info.Lineage.PermissionPolicy.Tools; len(got) != 0 {
					t.Fatalf("lineage tools = %#v, want unchanged empty session policy", got)
				}
			})
		}
	})
}

func TestCreateSpawnedAndCoordinatorSessionsValidateLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should validate spawned and coordinator lineage", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		h := newHarness(t, WithNow(func() time.Time { return now }))
		parent := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
				t.Fatalf("Stop(parent) error = %v", err)
			}
		})

		childTTL := now.Add(45 * time.Minute)
		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "worker",
			Workspace: h.workspaceID,
			Type:      SessionTypeSpawned,
			Lineage: &store.SessionLineage{
				ParentSessionID:  parent.ID,
				RootSessionID:    parent.ID,
				SpawnDepth:       1,
				SpawnRole:        "worker",
				TTLExpiresAt:     &childTTL,
				AutoStopOnParent: true,
				SpawnBudget: store.SessionSpawnBudget{
					MaxChildren:           2,
					MaxDepth:              1,
					MaxActivePerWorkspace: 4,
				},
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:           []string{testToolEdit, testToolRead},
					Skills:          []string{"go"},
					NetworkChannels: []string{"builders"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Create(spawned) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), child.ID); err != nil {
				t.Fatalf("Stop(child) error = %v", err)
			}
		})

		childInfo := child.Info()
		if childInfo.Type != SessionTypeSpawned {
			t.Fatalf("child type = %q, want %q", childInfo.Type, SessionTypeSpawned)
		}
		if childInfo.Lineage == nil {
			t.Fatal("child lineage = nil, want metadata")
		}
		if childInfo.Lineage.ParentSessionID != parent.ID ||
			childInfo.Lineage.RootSessionID != parent.ID ||
			childInfo.Lineage.SpawnDepth != 1 ||
			childInfo.Lineage.SpawnRole != "worker" ||
			!childInfo.Lineage.AutoStopOnParent {
			t.Fatalf("child lineage = %#v", childInfo.Lineage)
		}
		if childInfo.Lineage.TTLExpiresAt == nil || !childInfo.Lineage.TTLExpiresAt.Equal(childTTL) {
			t.Fatalf("child TTL = %#v, want %s", childInfo.Lineage.TTLExpiresAt, childTTL)
		}
		if childInfo.Lineage.SpawnBudget.TTLSeconds <= 0 {
			t.Fatalf(
				"child budget TTLSeconds = %d, want derived positive value",
				childInfo.Lineage.SpawnBudget.TTLSeconds,
			)
		}
		if got := childInfo.Lineage.PermissionPolicy.Tools; len(got) != 2 ||
			got[0] != testToolEdit ||
			got[1] != testToolRead {
			t.Fatalf("child policy tools = %#v, want normalized atoms", got)
		}

		coordinatorTTL := now.Add(2 * time.Hour)
		coordinator, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "coordinator",
			Workspace: h.workspaceID,
			Type:      SessionTypeCoordinator,
			Lineage: &store.SessionLineage{
				SpawnRole:    "coordinator",
				TTLExpiresAt: &coordinatorTTL,
				SpawnBudget:  store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 1},
			},
		})
		if err != nil {
			t.Fatalf("Create(coordinator) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), coordinator.ID); err != nil {
				t.Fatalf("Stop(coordinator) error = %v", err)
			}
		})
		if coordinator.Info().Type != SessionTypeCoordinator ||
			coordinator.Info().Lineage == nil ||
			coordinator.Info().Lineage.RootSessionID != coordinator.ID ||
			coordinator.Info().Lineage.ParentSessionID != "" {
			t.Fatalf("coordinator info = %#v", coordinator.Info())
		}
	})
}

func TestCreateRejectsInvalidLineage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, WithNow(func() time.Time { return now }))
	parent := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
			t.Fatalf("Stop(parent) error = %v", err)
		}
	})
	future := now.Add(time.Hour)
	expired := now.Add(-time.Second)

	tests := []struct {
		name    string
		opts    CreateOpts
		wantErr string
	}{
		{
			name: "Should reject invalid depth",
			opts: spawnedCreateOpts(parent.ID, &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      -1,
				TTLExpiresAt:    &future,
			}),
			wantErr: "spawn depth",
		},
		{
			name: "Should reject missing root",
			opts: spawnedCreateOpts(parent.ID, &store.SessionLineage{
				ParentSessionID: parent.ID,
				SpawnDepth:      1,
				TTLExpiresAt:    &future,
			}),
			wantErr: "root session id is required",
		},
		{
			name: "Should reject expired ttl",
			opts: spawnedCreateOpts(parent.ID, &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
				TTLExpiresAt:    &expired,
			}),
			wantErr: "ttl deadline must be in the future",
		},
		{
			name: "Should reject malformed policy",
			opts: spawnedCreateOpts(parent.ID, &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
				TTLExpiresAt:    &future,
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools: []string{testToolEdit, " "},
				},
			}),
			wantErr: "empty atom",
		},
		{
			name: "Should reject missing parent",
			opts: spawnedCreateOpts(parent.ID, &store.SessionLineage{
				ParentSessionID: "sess-missing",
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
				TTLExpiresAt:    &future,
			}),
			wantErr: "validate parent lineage",
		},
		{
			name: "Should reject coordinator missing ttl",
			opts: CreateOpts{
				AgentName: "coder",
				Workspace: h.workspaceID,
				Type:      SessionTypeCoordinator,
				Lineage:   &store.SessionLineage{SpawnRole: "coordinator"},
			},
			wantErr: "require a ttl deadline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := h.manager.Create(testutil.Context(t), tt.opts)
			if err == nil {
				cleanupSessionStop(t, h, created.ID)
				t.Fatalf("Create(%s) error = nil, want failure", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Create(%s) error = %v, want substring %q", tt.name, err, tt.wantErr)
			}
			if !errors.Is(err, ErrSessionNotFound) && strings.Contains(tt.wantErr, "validate parent lineage") {
				t.Fatalf("Create(%s) error = %v, want ErrSessionNotFound wrapping", tt.name, err)
			}
		})
	}
}

func spawnedCreateOpts(rootID string, lineage *store.SessionLineage) CreateOpts {
	return CreateOpts{
		AgentName: "coder",
		Workspace: "ws-primary",
		Type:      SessionTypeSpawned,
		Lineage:   lineage,
		Name:      "child-of-" + rootID,
	}
}
