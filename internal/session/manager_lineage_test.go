package session

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
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

		h := newHostedMCPHarness(t)
		addHarnessAgent(t, h, compozyconfig.AgentDef{
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
		addHarnessAgent(t, h, compozyconfig.AgentDef{
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

	t.Run("Should persist concrete root delegation tools for nil and empty overrides", func(t *testing.T) {
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

				h := newHostedMCPHarness(t)
				addHarnessAgent(t, h, compozyconfig.AgentDef{
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
				wantTools := []string{
					toolspkg.ToolIDTaskRead.String(),
					toolspkg.ToolIDTaskUpdate.String(),
				}
				if got := info.Lineage.PermissionPolicy.Tools; !testutil.EqualStringSlices(got, wantTools) {
					t.Fatalf("lineage tools = %#v, want concrete delegation policy %#v", got, wantTools)
				}
				meta := readMeta(t, sess.MetaPath())
				if got := meta.Lineage.PermissionPolicy.Tools; !testutil.EqualStringSlices(got, wantTools) {
					t.Fatalf("persisted lineage tools = %#v, want %#v", got, wantTools)
				}
			})
		}
	})

	t.Run("Should materialize the native universe for an unrestricted logical root", func(t *testing.T) {
		t.Parallel()

		universe := []toolspkg.ToolID{
			toolspkg.ToolIDAgentCall,
			toolspkg.ToolIDAgentMessage,
			toolspkg.ToolIDCallReturn,
		}
		h := newHostedMCPHarness(t, WithToolUniverse(universe))
		accepted, err := h.manager.CreateAccepted(t.Context(), CreateAcceptedOpts{Session: CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		}})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
				t.Errorf("Stop(session) error = %v", err)
			}
		})

		want := []string{
			toolspkg.ToolIDAgentCall.String(),
			toolspkg.ToolIDAgentMessage.String(),
			toolspkg.ToolIDCallReturn.String(),
		}
		if accepted.Lineage == nil || !testutil.EqualStringSlices(accepted.Lineage.PermissionPolicy.Tools, want) {
			t.Fatalf("accepted lineage = %#v, want native tool universe %#v", accepted.Lineage, want)
		}
		meta := readMeta(t, filepath.Join(h.homePaths.SessionsDir, accepted.ID, "meta.json"))
		if meta.Lineage == nil || !testutil.EqualStringSlices(meta.Lineage.PermissionPolicy.Tools, want) {
			t.Fatalf("persisted lineage = %#v, want native tool universe %#v", meta.Lineage, want)
		}
	})
}

func TestCreateSpawnedAndCoordinatorSessionsValidateLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should validate spawned and coordinator lineage", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		h := newHostedMCPHarness(t, WithNow(func() time.Time { return now }))
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

func TestCreateUserSessionRecordsProvenanceLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should record server-computed provenance for a user session with a parent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })

		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "provenance-child",
			Workspace: h.workspaceID,
			Lineage:   &store.SessionLineage{ParentSessionID: parent.ID},
		})
		if err != nil {
			t.Fatalf("Create(provenance child) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })

		info := child.Info()
		if info.Type != SessionTypeUser {
			t.Fatalf("child type = %q, want %q", info.Type, SessionTypeUser)
		}
		if info.Lineage == nil {
			t.Fatal("child lineage = nil, want provenance metadata")
		}
		if info.Lineage.ParentSessionID != parent.ID ||
			info.Lineage.RootSessionID != parent.ID ||
			info.Lineage.SpawnDepth != 1 {
			t.Fatalf("child lineage = %#v, want parent %q root %q depth 1", info.Lineage, parent.ID, parent.ID)
		}
		if info.Lineage.TTLExpiresAt != nil || info.Lineage.AutoStopOnParent || info.Lineage.SpawnRole != "" {
			t.Fatalf("child lineage = %#v, want no governance fields", info.Lineage)
		}

		meta := readMeta(t, child.MetaPath())
		if meta.Lineage == nil || meta.Lineage.ParentSessionID != parent.ID {
			t.Fatalf("meta lineage = %#v, want persisted parent %q", meta.Lineage, parent.ID)
		}

		grandchild, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "provenance-grandchild",
			Workspace: h.workspaceID,
			Lineage:   &store.SessionLineage{ParentSessionID: child.ID},
		})
		if err != nil {
			t.Fatalf("Create(provenance grandchild) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, grandchild.ID) })
		grandInfo := grandchild.Info()
		if grandInfo.Lineage == nil ||
			grandInfo.Lineage.ParentSessionID != child.ID ||
			grandInfo.Lineage.RootSessionID != parent.ID ||
			grandInfo.Lineage.SpawnDepth != 2 {
			t.Fatalf("grandchild lineage = %#v, want parent %q root %q depth 2", grandInfo.Lineage, child.ID, parent.ID)
		}
	})

	t.Run("Should reject governance fields on provenance lineage", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		h := newHarness(t, WithNow(func() time.Time { return now }))
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		future := now.Add(time.Hour)

		tests := []struct {
			name    string
			lineage *store.SessionLineage
			wantErr string
		}{
			{
				name:    "Should reject a ttl deadline",
				lineage: &store.SessionLineage{ParentSessionID: parent.ID, TTLExpiresAt: &future},
				wantErr: "ttl deadline",
			},
			{
				name:    "Should reject auto-stop on parent",
				lineage: &store.SessionLineage{ParentSessionID: parent.ID, AutoStopOnParent: true},
				wantErr: "auto-stop",
			},
			{
				name:    "Should reject a spawn role",
				lineage: &store.SessionLineage{ParentSessionID: parent.ID, SpawnRole: "worker"},
				wantErr: "spawn role",
			},
			{
				name: "Should reject a spawn budget",
				lineage: &store.SessionLineage{
					ParentSessionID: parent.ID,
					SpawnBudget:     store.SessionSpawnBudget{MaxChildren: 1, MaxDepth: 1},
				},
				wantErr: "spawn budget",
			},
			{
				name: "Should reject a permission policy",
				lineage: &store.SessionLineage{
					ParentSessionID:  parent.ID,
					PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"go"}},
				},
				wantErr: "permission policy",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				created, err := h.manager.Create(testutil.Context(t), CreateOpts{
					AgentName: "coder",
					Workspace: h.workspaceID,
					Lineage:   tt.lineage,
				})
				if err == nil {
					cleanupSessionStop(t, h, created.ID)
					t.Fatalf("Create(%s) error = nil, want failure", tt.name)
				}
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("Create(%s) error = %v, want %v", tt.name, err, ErrValidation)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Create(%s) error = %v, want substring %q", tt.name, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("Should reject a provenance parent from another workspace", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })

		_, err := h.manager.normalizeCreateLineage(
			testutil.Context(t),
			"sess-foreign-child",
			SessionTypeUser,
			"ws-other",
			&store.SessionLineage{ParentSessionID: parent.ID},
			"",
		)
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "another workspace") {
			t.Fatalf("normalizeCreateLineage(cross-workspace) error = %v, want workspace validation", err)
		}
	})

	t.Run("Should reject a missing provenance parent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Lineage:   &store.SessionLineage{ParentSessionID: "sess-missing"},
		})
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("Create(missing parent) error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("Should reject a parent on system sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeSystem,
			Lineage: &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "provenance-linked system sessions") {
			t.Fatalf("Create(system with parent) error = %v, want type rule rejection", err)
		}
	})
}

func TestCreateSystemSessionRecordsInternalProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Should persist lineage without spawn governance", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })

		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		})
		if err != nil {
			t.Fatalf("Create(system provenance) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })

		info := child.Info()
		if info.Type != SessionTypeSystem || info.Lineage == nil ||
			info.Lineage.ParentSessionID != parent.ID ||
			info.Lineage.RootSessionID != parent.ID || info.Lineage.SpawnDepth != 1 {
			t.Fatalf("system provenance info = %#v, want system child of %q", info, parent.ID)
		}
		if info.Lineage.TTLExpiresAt != nil || info.Lineage.AutoStopOnParent ||
			info.Lineage.SpawnRole != "" || !sessionPermissionPolicyIsEmpty(info.Lineage.PermissionPolicy) {
			t.Fatalf("system provenance lineage = %#v, want no spawn governance", info.Lineage)
		}
		meta := readMeta(t, child.MetaPath())
		if meta.Lineage == nil || meta.Lineage.ParentSessionID != parent.ID {
			t.Fatalf("persisted system provenance = %#v, want parent %q", meta.Lineage, parent.ID)
		}
	})

	t.Run("Should accept a stopped provenance parent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
			t.Fatalf("Stop(parent) error = %v", err)
		}
		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		})
		if err != nil {
			t.Fatalf("Create(system with stopped parent) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })
		if child.Info().Lineage == nil || child.Info().Lineage.ParentSessionID != parent.ID {
			t.Fatalf("system lineage = %#v, want stopped parent %q", child.Info().Lineage, parent.ID)
		}
	})

	t.Run("Should persist a missing provenance parent as an orphan root", func(t *testing.T) {
		t.Parallel()

		const parentID = "sess-deleted"
		h := newHarness(t)
		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parentID,
		})
		if err != nil {
			t.Fatalf("Create(system with deleted parent) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })
		lineage := child.Info().Lineage
		if lineage == nil || lineage.ParentSessionID != parentID ||
			lineage.RootSessionID != parentID || lineage.SpawnDepth != 1 {
			t.Fatalf("system lineage = %#v, want orphaned child of %q", lineage, parentID)
		}
		meta := readMeta(t, child.MetaPath())
		if meta.Lineage == nil || meta.Lineage.ParentSessionID != parentID ||
			meta.Lineage.RootSessionID != parentID || meta.Lineage.SpawnDepth != 1 {
			t.Fatalf("persisted system lineage = %#v, want orphaned child of %q", meta.Lineage, parentID)
		}
	})

	t.Run("Should rebase a new system child when the historical root is missing", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		root := createSession(t, h)
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		})
		if err != nil {
			t.Fatalf("Create(system parent) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		if err := h.manager.Delete(testutil.Context(t), root.ID); err != nil {
			t.Fatalf("Delete(historical root) error = %v", err)
		}

		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		})
		if err != nil {
			t.Fatalf("Create(system child with deleted historical root) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })
		lineage := child.Info().Lineage
		if lineage == nil || lineage.ParentSessionID != parent.ID ||
			lineage.RootSessionID != parent.ID || lineage.SpawnDepth != 2 {
			t.Fatalf("system child lineage = %#v, want rebased root %q at display depth 2", lineage, parent.ID)
		}
	})

	t.Run("Should reject provenance for a non-system session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeUser,
			ProvenanceParentSessionID: parent.ID,
		})
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "requires a system session") {
			t.Fatalf("Create(user with internal provenance) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject simultaneous public and internal lineage", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			Lineage:                   &store.SessionLineage{ParentSessionID: parent.ID},
			ProvenanceParentSessionID: parent.ID,
		})
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "cannot be combined with lineage") {
			t.Fatalf("Create(system with two lineage inputs) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject an internal provenance parent from another workspace", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		_, err := h.manager.normalizeCreateLineage(
			testutil.Context(t), "sess-system-foreign", SessionTypeSystem, "ws-other", nil, parent.ID,
		)
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "another workspace") {
			t.Fatalf("normalizeCreateLineage(system cross-workspace) error = %v, want %v", err, ErrValidation)
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
