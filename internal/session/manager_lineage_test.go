package session

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestCreateManualSessionProducesRootLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should persist the complete root policy for bounded instance resume", func(t *testing.T) {
		t.Parallel()
		h := newHostedMCPHarness(t)
		resolved, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		additional := t.TempDir()
		resolved.AdditionalDirs = []string{additional}
		resolved.Skills = []workspacepkg.SkillPath{{Name: "review"}, {Dir: "/skills/directory"}, {Name: "disabled"}}
		h.resolver.upsert(&resolved)
		addHarnessAgent(
			t,
			h,
			compozyconfig.AgentDef{
				Name:       "bounded-coder",
				Provider:   "claude",
				Prompt:     "Review work.",
				Skills:     compozyconfig.AgentSkillsConfig{Disabled: []string{"disabled"}},
				MCPServers: []compozyconfig.MCPServer{{Name: "docs", Command: "test-mcp"}},
			},
		)
		created, err := h.manager.Create(
			t.Context(),
			new(CreateOpts{AgentName: "bounded-coder", Workspace: h.workspaceID}),
		)
		if err != nil {
			t.Fatal(err)
		}
		policy := readMeta(t, created.MetaPath()).Lineage.PermissionPolicy
		want := store.NormalizeSessionPermissionPolicy(
			store.SessionPermissionPolicy{
				Tools:                  policy.Tools,
				Skills:                 []string{"review", "directory"},
				MCPServersMode:         store.PermissionInherit,
				ToolsMode:              store.PermissionInherit,
				WorkspacePaths:         []string{h.workspace, additional},
				MaterializedTools:      true,
				MaterializedMCPServers: false,
			},
		)
		if !reflect.DeepEqual(policy, want) {
			t.Fatalf("persisted policy = %+v, want %+v", policy, want)
		}
		if err := h.manager.Stop(t.Context(), created.ID); err != nil {
			t.Fatal(err)
		}
		h.manager.SetInstanceCeilingReader(
			func(context.Context, string, string) (*InstanceCeiling, error) {
				return &InstanceCeiling{Policy: policy}, nil
			},
		)
		resumed, err := h.manager.Resume(t.Context(), created.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), resumed.ID); err != nil {
				t.Error(err)
			}
		})
		if err := waitForSessionStartRun(t.Context(), h.manager.sessionStartRun(resumed.ID)); err != nil {
			t.Fatal(err)
		}
		if got := resumed.Info().Lineage.PermissionPolicy; !reflect.DeepEqual(got, want) {
			t.Fatalf("resumed ceiling = %+v, want %+v", got, want)
		}
	})

	// Invariant: live authority follows the scope without lifecycle policy rewrites.
	// Owner: session lifecycle; canonical suite: manager lineage (UT-018).
	t.Run("Should preserve policy while scope changes across stop and resume [UT-018]", func(t *testing.T) {
		t.Parallel()
		h := newHostedMCPHarness(t)
		admitted := []string{"linear"}
		h.manager.SetMCPAuthorityResolver(
			lineageAuthorityFunc(func(ctx context.Context, id string) (store.SessionPermissionPolicy, error) {
				info := MCPAuthoritySession(ctx, id)
				if info == nil {
					var err error
					info, err = h.manager.StoredStatus(ctx, id)
					if err != nil {
						return store.SessionPermissionPolicy{}, err
					}
				}
				policy := info.Lineage.PermissionPolicy
				policy.MCPServers = store.BoundMCPServers(policy, admitted)
				return policy, nil
			}),
		)
		created := createSession(t, h)
		before := readMeta(t, created.MetaPath()).Lineage.PermissionPolicy
		if err := h.manager.Stop(t.Context(), created.ID); err != nil {
			t.Fatal(err)
		}
		admitted = []string{"linear", "sentry"}
		resumed, err := h.manager.Resume(t.Context(), created.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })
		if err := waitForSessionStartRun(t.Context(), h.manager.sessionStartRun(resumed.ID)); err != nil {
			t.Fatal(err)
		}
		after := readMeta(t, resumed.MetaPath()).Lineage.PermissionPolicy
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("policy rewritten: before=%+v after=%+v", before, after)
		}
		policy, err := h.manager.EffectiveMCPAuthority(t.Context(), resumed.ID)
		if err != nil || !slices.Equal(policy.MCPServers, admitted) {
			t.Fatalf("authority=%+v err=%v", policy, err)
		}
	})

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

		sess, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "tool-coder",
			Workspace: h.workspaceID,
			AllowedToolsOverride: []string{
				toolspkg.ToolIDTaskRead.String(),
			},
		}))
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

		_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "read-only-coder",
			Workspace: h.workspaceID,
			AllowedToolsOverride: []string{
				toolspkg.ToolIDTaskRead.String(),
				toolspkg.ToolIDTaskUpdate.String(),
			},
		}))
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
			name         string
			override     []string
			sessionType  Type
			denyTools    []string
			unrestricted bool
		}{
			{name: "Should leave nil override unchanged"},
			{name: "Should leave empty override unchanged", override: []string{}},
			{name: "Should materialize default native tools for a root user session", unrestricted: true},
			{
				name:         "Should materialize default native tools for a root system session",
				sessionType:  SessionTypeSystem,
				unrestricted: true,
			},
			{
				name:         "Should honor denies with default native tools",
				sessionType:  SessionTypeSystem,
				unrestricted: true,
				denyTools:    []string{toolspkg.ToolIDTaskUpdate.String()},
			},
			{
				name:         "Should preserve a narrower explicit root system override",
				sessionType:  SessionTypeSystem,
				unrestricted: true,
				override:     []string{toolspkg.ToolIDTaskRead.String()},
			},
			{name: "Should materialize a root system session budget", sessionType: SessionTypeSystem},
			{
				name:        "Should materialize a root system session budget with an empty override",
				sessionType: SessionTypeSystem,
				override:    []string{},
			},
			{
				name:        "Should exclude denied tools from a root system session budget",
				sessionType: SessionTypeSystem,
				denyTools:   []string{toolspkg.ToolIDTaskUpdate.String()},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				h := newHostedMCPHarness(t, WithToolUniverse([]toolspkg.ToolID{
					toolspkg.ToolIDTaskRead, toolspkg.ToolIDTaskUpdate,
				}))
				agentTools := []string{toolspkg.ToolIDTaskRead.String(), toolspkg.ToolIDTaskUpdate.String()}
				if tt.unrestricted {
					agentTools = nil
				}
				addHarnessAgent(t, h, compozyconfig.AgentDef{
					Name:      "unchanged-coder",
					DenyTools: tt.denyTools,
					Provider:  "claude",
					Prompt:    "Use the default tool profile.",
					Tools:     agentTools,
				})

				sess, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
					AgentName:            "unchanged-coder",
					Type:                 tt.sessionType,
					Workspace:            h.workspaceID,
					AllowedToolsOverride: tt.override,
				}))
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
				if len(tt.denyTools) > 0 || len(tt.override) > 0 {
					wantTools = []string{toolspkg.ToolIDTaskRead.String()}
				}
				if got := info.Lineage.PermissionPolicy.Tools; !testutil.EqualStringSlices(got, wantTools) {
					t.Fatalf("lineage tools = %#v, want concrete delegation policy %#v", got, wantTools)
				}
				if got := info.Lineage.PermissionPolicy.WorkspacePaths; !testutil.EqualStringSlices(
					got,
					[]string{h.workspace},
				) {
					t.Fatalf("root workspace paths = %v, want %s", got, h.workspace)
				}
				meta := readMeta(t, sess.MetaPath())
				if got := meta.Lineage.PermissionPolicy.WorkspacePaths; !testutil.EqualStringSlices(
					got,
					[]string{h.workspace},
				) {
					t.Fatalf("persisted root workspace paths = %v, want %s", got, h.workspace)
				}
				if got := meta.Lineage.PermissionPolicy.Tools; !testutil.EqualStringSlices(got, wantTools) {
					t.Fatalf("persisted lineage tools = %#v, want %#v", got, wantTools)
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
		h := newHostedMCPHarness(t, WithNow(func() time.Time { return now }))
		parent := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
				t.Fatalf("Stop(parent) error = %v", err)
			}
		})

		childTTL := now.Add(45 * time.Minute)
		child, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
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
					Tools:  []string{testToolEdit, testToolRead},
					Skills: []string{"go"},
				},
			},
		}))
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
		coordinator, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder",
			Name:      "coordinator",
			Workspace: h.workspaceID,
			Type:      SessionTypeCoordinator,
			Lineage: &store.SessionLineage{
				SpawnRole:    "coordinator",
				TTLExpiresAt: &coordinatorTTL,
				SpawnBudget:  store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 1},
			},
		}))
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

		child, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder",
			Name:      "provenance-child",
			Workspace: h.workspaceID,
			Lineage:   &store.SessionLineage{ParentSessionID: parent.ID},
		}))
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

		grandchild, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder",
			Name:      "provenance-grandchild",
			Workspace: h.workspaceID,
			Lineage:   &store.SessionLineage{ParentSessionID: child.ID},
		}))
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
				created, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
					AgentName: "coder",
					Workspace: h.workspaceID,
					Lineage:   tt.lineage,
				}))
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
		_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Lineage:   &store.SessionLineage{ParentSessionID: "sess-missing"},
		}))
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("Create(missing parent) error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("Should reject an inherited provenance root from another workspace", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		root := createSession(t, h)
		parent, err := h.manager.Create(t.Context(), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID,
			Lineage: &store.SessionLineage{ParentSessionID: root.ID},
		}))
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		if err := h.manager.Stop(t.Context(), root.ID); err != nil {
			t.Fatalf("Stop(root) error = %v", err)
		}
		meta := readMeta(t, root.MetaPath())
		meta.WorkspaceID = "ws-other"
		if err := store.WriteSessionMeta(root.MetaPath(), &meta); err != nil {
			t.Fatalf("WriteSessionMeta(root) error = %v", err)
		}
		_, err = h.manager.normalizeCreateLineage(t.Context(), "sess-foreign-root-child",
			SessionTypeUser, h.workspaceID, &store.SessionLineage{ParentSessionID: parent.ID}, "")
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "another workspace") {
			t.Fatalf("normalizeCreateLineage(foreign root) error = %v, want workspace validation", err)
		}
	})

	t.Run("Should reject a parent on system sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })

		_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeSystem,
			Lineage: &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
			},
		}))
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

		child, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		}))
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
		child, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		}))
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
		child, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parentID,
		}))
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
		parent, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		}))
		if err != nil {
			t.Fatalf("Create(system parent) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		if err := h.manager.Delete(testutil.Context(t), root.ID); err != nil {
			t.Fatalf("Delete(historical root) error = %v", err)
		}

		child, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		}))
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
		_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeUser,
			ProvenanceParentSessionID: parent.ID,
		}))
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "requires a system session") {
			t.Fatalf("Create(user with internal provenance) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject simultaneous public and internal lineage", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			Lineage:                   &store.SessionLineage{ParentSessionID: parent.ID},
			ProvenanceParentSessionID: parent.ID,
		}))
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
			created, err := h.manager.Create(testutil.Context(t), &(tt.opts))
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

// lineageAuthorityFunc is the composition-root I/O boundary in session unit tests.
type lineageAuthorityFunc func(context.Context, string) (store.SessionPermissionPolicy, error)

func (f lineageAuthorityFunc) EffectiveMCPAuthority(
	ctx context.Context,
	id string,
) (store.SessionPermissionPolicy, error) {
	return f(ctx, id)
}

// Invariant: spawn persists intent and ceilings once, while hooks only narrow effective authority.
// Owner: session spawn admission; canonical suite: manager lineage.
func TestSpawnPermissionIntent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		named       []string
		hook        string
		wantMode    store.PermissionIntent
		wantServers []string
		denied      bool
	}{
		{name: "Should inherit new scope servers [UT-013]", wantMode: store.PermissionInherit, wantServers: []string{"linear", "sentry", "later"}},
		{name: "Should retain a named server list [UT-014]", named: []string{"linear"}, wantMode: store.PermissionList, wantServers: []string{"linear"}},
		{name: "Should accept a newly admitted named server before and after hooks [UT-104]", named: []string{"sentry"}, wantMode: store.PermissionList, wantServers: []string{"sentry"}},
		{name: "Should preserve inherit through an unchanged hook [UT-105]", hook: "unchanged", wantMode: store.PermissionInherit, wantServers: []string{"linear", "sentry", "later"}},
		{name: "Should persist a hook narrowing as a list [UT-105]", hook: "narrow", wantMode: store.PermissionList, wantServers: []string{"sentry"}},
		{name: "Should reject hook widening [UT-105]", hook: "widen", denied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hooks := &recordingSessionSpawnHooks{
				preCreatePatch: func(p hookspkg.SpawnPreCreatePayload) hookspkg.SpawnPreCreatePayload {
					switch tc.hook {
					case "narrow":
						p.ChildPermissions.MCPServers = []string{"sentry"}
					case "widen":
						p.ChildPermissions.MCPServers = append(p.ChildPermissions.MCPServers, "unauthorized")
					}
					return p
				},
			}
			h := newHostedMCPHarness(t, WithHookSet(HookSet{Spawn: hooks}))
			scopes := map[string][]string{h.workspaceID: {"linear"}}
			installLineageAuthority(h, scopes)
			parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
				Tools:          []string{"compozy__session_list", "compozy__session_spawn"},
				MCPServersMode: store.PermissionInherit, ToolsMode: store.PermissionInherit,
			}, store.SessionSpawnBudget{MaxDepth: 4, MaxChildren: 4})
			cleanupSessionStop(t, h, parent.ID)
			scopes[h.workspaceID] = []string{"linear", "sentry"}
			opts := SpawnOpts{ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour, InheritPermissions: true}
			if tc.named != nil {
				opts.PermissionNarrowing = &SubagentPermissionNarrowing{MCPServers: tc.named}
			}
			child, err := h.manager.Spawn(t.Context(), opts)
			if tc.denied {
				if !errors.Is(err, ErrSpawnPermissionDenied) {
					t.Fatalf("hook widening: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			cleanupSessionStop(t, h, child.ID)
			stored := readMeta(t, child.MetaPath()).Lineage.PermissionPolicy
			if stored.MCPServersMode != tc.wantMode {
				t.Fatalf("intent=%+v", stored)
			}
			scopes[h.workspaceID] = append(scopes[h.workspaceID], "later")
			got, err := h.manager.EffectiveMCPAuthority(t.Context(), child.ID)
			if err != nil || !slices.Equal(got.MCPServers, tc.wantServers) {
				t.Fatalf("authority=%+v err=%v", got, err)
			}
			public, err := h.manager.Status(t.Context(), child.ID)
			if err != nil || !slices.Equal(public.Lineage.EffectiveMCPServers, tc.wantServers) {
				t.Fatalf("public effective authority=%+v err=%v", public, err)
			}
			if len(hooks.preCreate) != 1 ||
				!slices.Equal(hooks.preCreate[0].ParentPermissions.MCPServers, []string{"linear", "sentry"}) {
				t.Fatalf("hook did not see live parent authority: %+v", hooks.preCreate)
			}
		})
	}
}

// Invariant: a descendant cannot lose an inherited tool restriction when its parent disappears.
// Owner: spawn policy persistence; canonical suite: manager lineage (UT-108).
func TestSpawnRetainsStoredToolIntent(t *testing.T) {
	t.Parallel()
	h := newHostedMCPHarness(t)
	scopes := map[string][]string{h.workspaceID: {"linear"}}
	installLineageAuthority(h, scopes)
	tools := []string{"compozy__session_list", "compozy__session_spawn"}
	parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
		Tools: tools, ToolsMode: store.PermissionList, MCPServersMode: store.PermissionInherit,
	}, store.SessionSpawnBudget{MaxDepth: 4, MaxChildren: 4})
	child, err := h.manager.Spawn(
		t.Context(),
		SpawnOpts{ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour, InheritPermissions: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := h.manager.Spawn(
		t.Context(),
		SpawnOpts{ParentSessionID: child.ID, AgentName: "coder", TTL: time.Hour, InheritPermissions: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	cleanupSessionStop(t, h, grandchild.ID)
	if err := h.manager.Delete(t.Context(), child.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.manager.Delete(t.Context(), parent.ID); err != nil {
		t.Fatal(err)
	}
	got, err := h.manager.EffectiveMCPAuthority(t.Context(), grandchild.ID)
	if err != nil || got.ToolsMode != store.PermissionList || !slices.Equal(got.Tools, tools) {
		t.Fatalf("grandchild authority=%+v err=%v", got, err)
	}
}

// Invariant: cross-workspace children use their own scope and the parent's spawn-time ceiling.
// Owner: spawn authority; canonical suite: manager lineage (UT-109).
func TestCrossWorkspaceSpawnIntent(t *testing.T) {
	t.Parallel()
	h := newHostedMCPHarness(t)
	scopes := map[string][]string{h.workspaceID: {"linear"}, "ws-other": {"linear", "only-b"}}
	installLineageAuthority(h, scopes)
	parent := &Info{ID: "parent", WorkspaceID: h.workspaceID, ProfileID: store.DefaultProfileID, Type: SessionTypeUser,
		Lineage: &store.SessionLineage{PermissionPolicy: store.SessionPermissionPolicy{
			MCPServersMode: store.PermissionInherit,
			ToolsMode:      store.PermissionInherit,
			Tools:          []string{"compozy__session_list"},
		}}}
	policy, err := h.manager.resolveSpawnPermissionPolicy(
		t.Context(),
		parent,
		&SpawnOpts{Workspace: "ws-other", InheritPermissions: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if policy.AncestorMCPCeiling == nil || !slices.Equal(*policy.AncestorMCPCeiling, []string{"linear"}) ||
		policy.ToolsMode != store.PermissionList {
		t.Fatalf("cross-scope ceiling=%+v", policy)
	}
	scopes[h.workspaceID] = append(scopes[h.workspaceID], "later-a")
	scopes["ws-other"] = append(scopes["ws-other"], "later-a")
	got, err := h.manager.effectiveSpawnAuthority(
		t.Context(),
		parent,
		&SpawnOpts{Workspace: "ws-other", PermissionPolicy: policy},
	)
	if err != nil || !slices.Equal(got.MCPServers, []string{"linear"}) {
		t.Fatalf("cross-scope authority=%+v err=%v", got, err)
	}
}

func installLineageAuthority(h *harness, scopes map[string][]string) {
	h.manager.SetMCPAuthorityResolver(
		lineageAuthorityFunc(func(ctx context.Context, id string) (store.SessionPermissionPolicy, error) {
			info := MCPAuthoritySession(ctx, id)
			if info == nil {
				var err error
				info, err = h.manager.StoredStatus(ctx, id)
				if err != nil {
					return store.SessionPermissionPolicy{}, err
				}
			}
			lineage := store.NormalizeSessionLineage(id, info.Lineage)
			policy := store.DecodeIntent(lineage.PermissionPolicy, lineage.Kind, string(info.Type))
			policy.MCPServers = store.BoundMCPServers(policy, scopes[info.WorkspaceID])
			return policy, nil
		}),
	)
}

// Invariant: a parent's named MCP list becomes a durable ceiling for its descendants.
// Owner: spawn policy persistence; canonical suite: manager lineage (UT-103).
func TestSpawnRetainsAncestorMCPCeiling(t *testing.T) {
	t.Parallel()
	h := newHostedMCPHarness(t)
	scopes := map[string][]string{h.workspaceID: {"linear"}}
	installLineageAuthority(h, scopes)
	root := createSpawnParent(t, h, store.SessionPermissionPolicy{
		Tools:     []string{"compozy__session_list", "compozy__session_spawn"},
		ToolsMode: store.PermissionInherit, MCPServersMode: store.PermissionInherit,
	}, store.SessionSpawnBudget{MaxDepth: 4, MaxChildren: 4})
	cleanupSessionStop(t, h, root.ID)
	parent, err := h.manager.Spawn(t.Context(), SpawnOpts{
		ParentSessionID: root.ID, AgentName: "coder", TTL: time.Hour, InheritPermissions: true,
		PermissionNarrowing: &SubagentPermissionNarrowing{MCPServers: []string{"linear"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := h.manager.Spawn(t.Context(), SpawnOpts{
		ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour, InheritPermissions: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupSessionStop(t, h, child.ID)
	stored := readMeta(t, child.MetaPath()).Lineage.PermissionPolicy
	if stored.MCPServersMode != store.PermissionInherit || stored.AncestorMCPCeiling == nil ||
		!slices.Equal(*stored.AncestorMCPCeiling, []string{"linear"}) {
		t.Fatalf("descendant ceiling=%+v", stored)
	}
	if err := h.manager.Delete(t.Context(), parent.ID); err != nil {
		t.Fatal(err)
	}
	scopes[h.workspaceID] = append(scopes[h.workspaceID], "sentry")
	got, err := h.manager.EffectiveMCPAuthority(t.Context(), child.ID)
	if err != nil || !slices.Equal(got.MCPServers, []string{"linear"}) {
		t.Fatalf("authority after ancestor deletion=%+v err=%v", got, err)
	}
}
