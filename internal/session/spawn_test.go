package session

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	testToolEdit  = "compozy__edit"
	testToolRead  = "compozy__read"
	testToolShell = "compozy__shell"
)

func TestValidatePermissionSubset(t *testing.T) {
	t.Parallel()

	parent := store.SessionPermissionPolicy{
		Tools:           []string{testToolEdit, testToolRead},
		Skills:          []string{"go", "tests"},
		MCPServers:      []string{"filesystem"},
		WorkspacePaths:  []string{"/repo"},
		NetworkChannels: []string{"builders"},
		SandboxProfiles: []string{"default"},
	}

	tests := []struct {
		name    string
		child   store.SessionPermissionPolicy
		wantErr bool
	}{
		{
			name:  "Should accept exact permissions",
			child: parent,
		},
		{
			name: "Should accept subset permissions",
			child: store.SessionPermissionPolicy{
				Tools:           []string{testToolRead},
				Skills:          []string{"go"},
				MCPServers:      []string{"filesystem"},
				WorkspacePaths:  []string{"/repo"},
				NetworkChannels: []string{"builders"},
				SandboxProfiles: []string{"default"},
			},
		},
		{
			name: "Should reject superset permissions",
			child: store.SessionPermissionPolicy{
				Tools: []string{testToolEdit, testToolShell},
			},
			wantErr: true,
		},
		{
			name: "Should reject unknown atoms",
			child: store.SessionPermissionPolicy{
				MCPServers: []string{"unknown-server"},
			},
			wantErr: true,
		},
		{
			name: "Should reject blank atoms",
			child: store.SessionPermissionPolicy{
				Tools: []string{" "},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePermissionSubset(parent, tt.child)
			if tt.wantErr {
				if !errors.Is(err, ErrSpawnPermissionDenied) {
					t.Fatalf("ValidatePermissionSubset() error = %v, want %v", err, ErrSpawnPermissionDenied)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePermissionSubset() error = %v", err)
			}
		})
	}
}

func TestSpawnWorkspaceAccess(t *testing.T) {
	t.Parallel()

	t.Run("Should bind an authorized child to the foreign workspace after both validations", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		foreign, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(home) error = %v", err)
		}
		foreign.ID = "ws-other"
		foreign.Name = "other"
		h.resolver.upsert(&foreign)
		policy := &recordingSpawnWorkspaceAccessPolicy{decision: workspaceaccess.Decision{Allowed: true}}
		h.manager.SetWorkspaceAccessPolicy(policy)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{}, store.SessionSpawnBudget{
			MaxChildren:           2,
			MaxDepth:              1,
			MaxActivePerWorkspace: 1,
		})
		cleanupSessionStop(t, h, parent.ID)

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			Workspace:       foreign.ID,
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(foreign) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		if child.Info().WorkspaceID != foreign.ID {
			t.Fatalf("child workspace = %q, want %q", child.Info().WorkspaceID, foreign.ID)
		}
		if policy.calls == 0 {
			t.Fatal("workspace policy was not called")
		}
		wantRequest := workspaceaccess.Request{
			Actor: workspaceaccess.ActorRef{
				Kind:        workspaceaccess.ActorAgentSession,
				SessionID:   parent.ID,
				WorkspaceID: h.workspaceID,
				AgentName:   "coder",
			},
			TargetWorkspaceID: foreign.ID,
			Seam:              workspaceaccess.SeamSpawn,
		}
		for index, req := range policy.requests {
			if req != wantRequest {
				t.Fatalf("policy request[%d] = %#v, want %#v", index, req, wantRequest)
			}
		}

		_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			Workspace:       foreign.ID,
			TTL:             time.Minute,
		})
		if !errors.Is(err, ErrSpawnLimitExceeded) || !strings.Contains(err.Error(), foreign.ID) {
			t.Fatalf("Spawn(second foreign child) error = %v, want target workspace cap", err)
		}
	})

	t.Run("Should preserve child workspace resolution failures", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{}, store.SessionSpawnBudget{
			MaxChildren: 1,
			MaxDepth:    1,
		})
		cleanupSessionStop(t, h, parent.ID)
		resolveErr := errors.New("workspace registry unavailable")
		h.resolver.resolveErr = resolveErr

		_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			Workspace:       "ws-other",
			TTL:             time.Minute,
		})
		if !errors.Is(err, ErrSpawnValidation) || !errors.Is(err, resolveErr) {
			t.Fatalf("Spawn(resolve failure) error = %v, want validation joined with resolver cause", err)
		}
	})

	t.Run("Should preserve workspace policy failures", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		foreign, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(home) error = %v", err)
		}
		foreign.ID = "ws-other"
		h.resolver.upsert(&foreign)
		policyErr := errors.New("workspace policy unavailable")
		h.manager.SetWorkspaceAccessPolicy(&recordingSpawnWorkspaceAccessPolicy{err: policyErr})
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{}, store.SessionSpawnBudget{
			MaxChildren: 1,
			MaxDepth:    1,
		})
		cleanupSessionStop(t, h, parent.ID)

		_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			Workspace:       foreign.ID,
			TTL:             time.Minute,
		})
		if !errors.Is(err, ErrSpawnValidation) || !errors.Is(err, policyErr) {
			t.Fatalf("Spawn(policy failure) error = %v, want validation joined with policy cause", err)
		}
	})
}

func TestManagerSpawnCreatesChildWithDurableLineageAndNarrowPermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit the parent profile ownership", func(t *testing.T) {
		t.Parallel()

		const profileID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		h := newHarness(t, WithProfileNameResolver(profileNameResolverMap{profileID: "marketing"}))
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			ProfileID: profileID,
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 1, MaxDepth: 1},
			},
		})
		if err != nil {
			t.Fatalf("Create(profile parent) error = %v", err)
		}
		cleanupSessionStop(t, h, parent.ID)

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(profile child) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		if got := child.Info().ProfileID; got != profileID {
			t.Fatalf("child profile = %q, want inherited %q", got, profileID)
		}
		if got := readMeta(t, child.MetaPath()).ProfileID; got != profileID {
			t.Fatalf("persisted child profile = %q, want inherited %q", got, profileID)
		}
	})

	t.Run("Should create child with durable lineage and narrowed permissions", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		h := newHostedMCPHarness(t, WithNow(func() time.Time { return now }))
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

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:  parent.ID,
			AgentName:        "coder",
			Name:             "child worker",
			PromptOverlay:    "Focus only on tests.",
			TTL:              30 * time.Minute,
			AutoStopOnParent: true,
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

		info := child.Info()
		if info.Type != SessionTypeSpawned {
			t.Fatalf("child type = %q, want %q", info.Type, SessionTypeSpawned)
		}
		if got, want := info.NetworkParticipation, participation.LocalSpec(); got != want {
			t.Fatalf("child participation = %#v, want %#v", got, want)
		}
		if info.Lineage == nil {
			t.Fatal("child lineage = nil, want durable lineage")
		}
		if info.Lineage.ParentSessionID != parent.ID ||
			info.Lineage.RootSessionID != parent.ID ||
			info.Lineage.SpawnDepth != 1 ||
			info.Lineage.SpawnRole != DefaultSpawnRole ||
			!info.Lineage.AutoStopOnParent ||
			!info.Lineage.NotifyCreator {
			t.Fatalf("child lineage = %#v", info.Lineage)
		}
		wantTTL := now.Add(30 * time.Minute)
		if info.Lineage.TTLExpiresAt == nil || !info.Lineage.TTLExpiresAt.Equal(wantTTL) {
			t.Fatalf("child TTL = %#v, want %s", info.Lineage.TTLExpiresAt, wantTTL)
		}
		if got := info.Lineage.PermissionPolicy.Tools; len(got) != 1 || got[0] != testToolRead {
			t.Fatalf("child permission tools = %#v, want narrowed read", got)
		}
		meta := readMeta(t, child.MetaPath())
		if meta.Lineage == nil || meta.Lineage.ParentSessionID != parent.ID || !meta.Lineage.NotifyCreator {
			t.Fatalf("persisted lineage = %#v, want parent %q", meta.Lineage, parent.ID)
		}
		if len(h.driver.startCalls) < 2 ||
			!strings.Contains(h.driver.startCalls[len(h.driver.startCalls)-1].SystemPrompt, "Focus only on tests.") {
			t.Fatalf("child prompt overlay was not appended to start prompt: %#v", h.driver.startCalls)
		}
	})

	t.Run("Should preserve an explicit creator notification opt out", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{}, store.SessionSpawnBudget{
			MaxChildren: 1,
			MaxDepth:    1,
		})
		cleanupSessionStop(t, h, parent.ID)

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:  parent.ID,
			AgentName:        "coder",
			TTL:              time.Minute,
			NotifyCreator:    false,
			NotifyCreatorSet: true,
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		if child.Info().Lineage == nil || child.Info().Lineage.NotifyCreator {
			t.Fatalf("child lineage = %#v, want notify_creator=false", child.Info().Lineage)
		}
		meta := readMeta(t, child.MetaPath())
		if meta.Lineage == nil || meta.Lineage.NotifyCreator {
			t.Fatalf("persisted lineage = %#v, want notify_creator=false", meta.Lineage)
		}
	})

	t.Run("Should create distinct children for duplicate spawn requests", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{}, store.SessionSpawnBudget{
			MaxChildren: 2,
			MaxDepth:    1,
		})
		cleanupSessionStop(t, h, parent.ID)
		opts := SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
			IdempotencyKey:  "duplicate-spawn",
		}

		first, err := h.manager.Spawn(testutil.Context(t), opts)
		if err != nil {
			t.Fatalf("Spawn(first) error = %v", err)
		}
		cleanupSessionStop(t, h, first.ID)
		second, err := h.manager.Spawn(testutil.Context(t), opts)
		if err != nil {
			t.Fatalf("Spawn(second) error = %v", err)
		}
		cleanupSessionStop(t, h, second.ID)
		if first.ID == second.ID {
			t.Fatalf("duplicate spawn ids = %q, want distinct children", first.ID)
		}
	})

	t.Run("Should inherit the parent worktree and refuse a missing binding", func(t *testing.T) {
		t.Parallel()

		worktreeRoot := filepath.Join(t.TempDir(), "worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		worktreeResolver := &fakeSessionWorktreeResolver{id: "wt-spawn", root: worktreeRoot}
		h := newHostedMCPHarness(t, WithWorktreeResolver(worktreeResolver))
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "bound parent",
			Workspace: h.workspaceID,
			Worktree:  "wt-spawn",
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
			},
		})
		if err != nil {
			t.Fatalf("Create(bound parent) error = %v", err)
		}
		cleanupSessionStop(t, h, parent.ID)

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(bound child) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		if got := child.Info().WorktreeID; got != "wt-spawn" {
			t.Fatalf("child worktree = %q, want %q", got, "wt-spawn")
		}
		canonicalRoot, err := canonicalDirectory(worktreeRoot)
		if err != nil {
			t.Fatalf("canonicalDirectory(worktree root) error = %v", err)
		}
		if got := h.driver.startCalls[len(h.driver.startCalls)-1].Cwd; got != canonicalRoot {
			t.Fatalf("child cwd = %q, want %q", got, canonicalRoot)
		}

		missingErr := errors.New("worktree: missing")
		worktreeResolver.setError(missingErr)
		startsBefore := len(h.driver.startCalls)
		_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if !errors.Is(err, missingErr) {
			t.Fatalf("Spawn(missing binding) error = %v, want %v", err, missingErr)
		}
		if got := len(h.driver.startCalls); got != startsBefore {
			t.Fatalf("driver starts after missing binding = %d, want %d", got, startsBefore)
		}
	})

	t.Run("Should keep children local by default and enforce their delegated channel scope", func(t *testing.T) {
		t.Parallel()

		h := newHostedMCPHarness(t, WithParticipationResolver(newTestSessionParticipationResolver(t, true)))
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:                    "coder",
			Workspace:                    h.workspaceID,
			ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 1},
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:           []string{testToolRead},
					NetworkChannels: []string{"builders", "restricted"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		cleanupSessionStop(t, h, parent.ID)

		localChild, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools: []string{testToolRead},
			},
		})
		if err != nil {
			t.Fatalf("Spawn(local) error = %v", err)
		}
		cleanupSessionStop(t, h, localChild.ID)
		if got, want := localChild.Info().NetworkParticipation, participation.LocalSpec(); got != want {
			t.Fatalf("local child participation = %#v, want %#v", got, want)
		}

		live := participation.ModeLive
		named := participation.StrategyNamed
		builders := "builders"
		liveChild, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &builders,
			},
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools:           []string{testToolRead},
				NetworkChannels: []string{"builders"},
			},
		})
		if err != nil {
			t.Fatalf("Spawn(live) error = %v", err)
		}
		cleanupSessionStop(t, h, liveChild.ID)
		if got, want := liveChild.Info().NetworkParticipation.ChannelID, builders; got != want {
			t.Fatalf("live child channel = %q, want %q", got, want)
		}

		activeBefore := len(h.manager.List())
		restricted := "restricted"
		_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &restricted,
			},
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools:           []string{testToolRead},
				NetworkChannels: []string{"builders"},
			},
		})
		if !errors.Is(err, participation.ErrAuthorityDenied) {
			t.Fatalf("Spawn(denied) error = %v, want %v", err, participation.ErrAuthorityDenied)
		}
		if got := len(h.manager.List()); got != activeBefore {
			t.Fatalf("active sessions after denial = %d, want %d", got, activeBefore)
		}
	})

	t.Run("Should keep an automatic-title child off collaboration channels", func(t *testing.T) {
		t.Parallel()

		wakeNotifier := &recordingSpawnWakeNotifier{}
		h := newHarness(
			t,
			WithParticipationResolver(newTestSessionParticipationResolver(t, true)),
			WithSpawnWakeNotifier(wakeNotifier),
		)
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:                    "coder",
			Workspace:                    h.workspaceID,
			ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 1, MaxDepth: 1},
				PermissionPolicy: store.SessionPermissionPolicy{
					NetworkChannels: []string{"builders"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		cleanupSessionStop(t, h, parent.ID)

		live := participation.ModeLive
		named := participation.StrategyNamed
		builders := "builders"
		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &builders,
			},
			SpawnRole:        SpawnRoleAutoTitle,
			TTL:              time.Minute,
			AutoStopOnParent: true,
			PermissionPolicy: store.SessionPermissionPolicy{
				NetworkChannels: []string{"builders"},
			},
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)

		if got, want := child.Info().NetworkParticipation, participation.LocalSpec(); got != want {
			t.Fatalf("child participation = %#v, want %#v", got, want)
		}
		if got, want := readMeta(t, child.MetaPath()).NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
			t.Fatalf("persisted child participation = %#v, want %#v", got, want)
		}
		if child.Info().Lineage == nil || child.Info().Lineage.NotifyCreator {
			t.Fatalf("child lineage = %#v, want creator wake disabled", child.Info().Lineage)
		}
		meta := readMeta(t, child.MetaPath())
		if meta.Lineage == nil || meta.Lineage.NotifyCreator {
			t.Fatalf("persisted child lineage = %#v, want creator wake disabled", meta.Lineage)
		}
		if parents, events := wakeNotifier.calls(); len(parents) != 0 || len(events) != 0 {
			t.Fatalf("internal child wakes = parents %#v events %#v, want none", parents, events)
		}
	})

	t.Run("Should inherit parent speed unless the child explicitly overrides it", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Speed:     speedpkg.SpeedFast,
			Workspace: h.workspaceID,
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
			},
		})
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		cleanupSessionStop(t, h, parent.ID)

		inherited, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(inherited) error = %v", err)
		}
		cleanupSessionStop(t, h, inherited.ID)
		if got, want := inherited.Info().Speed, speedpkg.SpeedFast; got != want {
			t.Fatalf("inherited child speed = %q, want %q", got, want)
		}

		overridden, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			Speed:           speedpkg.SpeedNormal,
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(overridden) error = %v", err)
		}
		cleanupSessionStop(t, h, overridden.ID)
		if got, want := overridden.Info().Speed, speedpkg.SpeedNormal; got != want {
			t.Fatalf("overridden child speed = %q, want %q", got, want)
		}
	})
}

func TestManagerSpawnRejectsPolicyViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func(t *testing.T, h *harness, parent *Session) error
		wantErr error
	}{
		{
			name: "missing TTL",
			run: func(t *testing.T, h *harness, parent *Session) error {
				t.Helper()
				_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
				})
				return err
			},
			wantErr: ErrSpawnValidation,
		},
		{
			name: "coordinator role",
			run: func(t *testing.T, h *harness, parent *Session) error {
				t.Helper()
				_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
					SpawnRole:       "coordinator",
					TTL:             time.Minute,
				})
				return err
			},
			wantErr: ErrSpawnValidation,
		},
		{
			name: "permission widening",
			run: func(t *testing.T, h *harness, parent *Session) error {
				t.Helper()
				_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
					TTL:             time.Minute,
					PermissionPolicy: store.SessionPermissionPolicy{
						Tools: []string{testToolShell},
					},
				})
				return err
			},
			wantErr: ErrSpawnPermissionDenied,
		},
		{
			name: "cross workspace",
			run: func(t *testing.T, h *harness, parent *Session) error {
				t.Helper()
				foreign, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
				if err != nil {
					t.Fatalf("Resolve(home) error = %v", err)
				}
				foreign.ID = "ws-other"
				foreign.Name = "other"
				h.resolver.upsert(&foreign)
				_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
					Workspace:       "ws-other",
					TTL:             time.Minute,
				})
				return err
			},
			wantErr: ErrSpawnPermissionDenied,
		},
		{
			name: "child cap",
			run: func(t *testing.T, h *harness, parent *Session) error {
				t.Helper()
				child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
					TTL:             time.Minute,
				})
				if err != nil {
					return err
				}
				cleanupSessionStop(t, h, child.ID)
				_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
					TTL:             time.Minute,
				})
				return err
			},
			wantErr: ErrSpawnLimitExceeded,
		},
		{
			name: "max depth",
			run: func(t *testing.T, h *harness, parent *Session) error {
				t.Helper()
				child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "coder",
					TTL:             time.Minute,
				})
				if err != nil {
					return err
				}
				cleanupSessionStop(t, h, child.ID)
				_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
					ParentSessionID: child.ID,
					AgentName:       "coder",
					TTL:             time.Minute,
				})
				return err
			},
			wantErr: ErrSpawnLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newHostedMCPHarness(t)
			parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
				Tools: []string{testToolRead},
			}, store.SessionSpawnBudget{MaxChildren: 1, MaxDepth: 1})
			cleanupSessionStop(t, h, parent.ID)

			err := tt.run(t, h, parent)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Spawn() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagerSpawnStoppedParentRules(t *testing.T) {
	t.Parallel()

	t.Run("Should reject stopped parents for regular spawned sessions", func(t *testing.T) {
		t.Parallel()

		h := newHostedMCPHarness(t)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
			Tools: []string{testToolRead},
		}, store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1})
		if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
			t.Fatalf("Stop(parent) error = %v", err)
		}

		_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if !errors.Is(err, ErrSpawnValidation) {
			t.Fatalf("Spawn() error = %v, want %v", err, ErrSpawnValidation)
		}
	})

	t.Run("Should allow daemon memory extractor spawns from stopped parents", func(t *testing.T) {
		t.Parallel()

		h := newHostedMCPHarness(t)
		parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:                    "coder",
			Name:                         "networked parent",
			Workspace:                    h.workspaceID,
			ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
			Type:                         SessionTypeUser,
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:           []string{testToolRead},
					NetworkChannels: []string{"builders"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
			t.Fatalf("Stop(parent) error = %v", err)
		}

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:    parent.ID,
			AgentName:          "coder",
			SpawnRole:          SpawnRoleMemoryExtractor,
			TTL:                time.Minute,
			AllowStoppedParent: true,
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools: []string{testToolRead},
			},
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)

		if got, want := child.Info().NetworkParticipation, participation.LocalSpec(); got != want {
			t.Fatalf("child participation = %#v, want %#v", got, want)
		}
		if got, want := readMeta(t, child.MetaPath()).NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
			t.Fatalf("persisted child participation = %#v, want %#v", got, want)
		}

		lineage := child.Info().Lineage
		if lineage == nil ||
			lineage.ParentSessionID != parent.ID ||
			lineage.RootSessionID != parent.ID ||
			lineage.SpawnRole != SpawnRoleMemoryExtractor ||
			lineage.AutoStopOnParent {
			t.Fatalf("child lineage = %#v, want extractor child linked to stopped parent without auto-stop", lineage)
		}
	})

	t.Run("Should reject stopped parent override outside memory extractor role", func(t *testing.T) {
		t.Parallel()

		h := newHostedMCPHarness(t)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
			Tools: []string{testToolRead},
		}, store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1})
		cleanupSessionStop(t, h, parent.ID)

		_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:    parent.ID,
			AgentName:          "coder",
			TTL:                time.Minute,
			AllowStoppedParent: true,
		})
		if !errors.Is(err, ErrSpawnValidation) {
			t.Fatalf("Spawn() error = %v, want %v", err, ErrSpawnValidation)
		}
	})

	t.Run("Should reject stopped parent override with auto stop lineage", func(t *testing.T) {
		t.Parallel()

		h := newHostedMCPHarness(t)
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
			Tools: []string{testToolRead},
		}, store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1})
		cleanupSessionStop(t, h, parent.ID)

		_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:    parent.ID,
			AgentName:          "coder",
			SpawnRole:          SpawnRoleMemoryExtractor,
			TTL:                time.Minute,
			AutoStopOnParent:   true,
			AllowStoppedParent: true,
		})
		if !errors.Is(err, ErrSpawnValidation) {
			t.Fatalf("Spawn() error = %v, want %v", err, ErrSpawnValidation)
		}
	})
}

func TestManagerSpawnHooksCarryLineageAndCannotWidenPermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should carry lineage through hook payloads", func(t *testing.T) {
		t.Parallel()

		hooks := &recordingSessionSpawnHooks{}
		h := newHostedMCPHarness(t, WithHookSet(HookSet{Spawn: hooks}))
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
			Tools: []string{testToolRead},
		}, store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1})
		cleanupSessionStop(t, h, parent.ID)

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:  parent.ID,
			AgentName:        "coder",
			TTL:              time.Minute,
			AutoStopOnParent: true,
			PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)

		if len(hooks.preCreate) != 1 || len(hooks.created) != 1 {
			t.Fatalf("hook counts pre=%d created=%d, want 1 each", len(hooks.preCreate), len(hooks.created))
		}
		pre := hooks.preCreate[0]
		if pre.ParentSessionID != parent.ID ||
			pre.RootSessionID != parent.ID ||
			pre.SpawnDepth != 1 ||
			pre.ChildSessionID != "" ||
			len(pre.ChildPermissions.Tools) != 1 ||
			pre.ChildPermissions.Tools[0] != testToolRead {
			t.Fatalf("pre-create payload = %#v, want parent/root/depth and narrowed permissions", pre)
		}
		created := hooks.created[0]
		if created.ParentSessionID != parent.ID ||
			created.RootSessionID != parent.ID ||
			created.ChildSessionID != child.ID ||
			created.SpawnDepth != 1 ||
			created.AgentName != "coder" {
			t.Fatalf("created payload = %#v, want durable child lineage", created)
		}
	})

	t.Run("Should reject hook permission widening", func(t *testing.T) {
		t.Parallel()

		hooks := &recordingSessionSpawnHooks{
			preCreatePatch: func(payload hookspkg.SpawnPreCreatePayload) hookspkg.SpawnPreCreatePayload {
				payload.ChildPermissions.Tools = []string{testToolShell}
				return payload
			},
		}
		h := newHostedMCPHarness(t, WithHookSet(HookSet{Spawn: hooks}))
		parent := createSpawnParent(t, h, store.SessionPermissionPolicy{
			Tools: []string{testToolRead},
		}, store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1})
		cleanupSessionStop(t, h, parent.ID)

		_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID:  parent.ID,
			AgentName:        "coder",
			TTL:              time.Minute,
			AutoStopOnParent: true,
			PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
		})
		if !errors.Is(err, ErrSpawnPermissionDenied) {
			t.Fatalf("Spawn() error = %v, want %v", err, ErrSpawnPermissionDenied)
		}
	})
}

func TestSpawnGovernanceUsesOnlyContiguousSpawnedLineage(t *testing.T) {
	t.Parallel()

	t.Run("Should start governed depth at one below a system provenance parent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		root := createSession(t, h)
		cleanupSessionStop(t, h, root.ID)
		goal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		})
		if err != nil {
			t.Fatalf("Create(system provenance parent) error = %v", err)
		}
		cleanupSessionStop(t, h, goal.ID)

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: goal.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(system provenance parent) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		info := child.Info()
		if info.Type != SessionTypeSpawned || info.Lineage == nil ||
			info.Lineage.ParentSessionID != goal.ID || info.Lineage.RootSessionID != root.ID ||
			info.Lineage.SpawnDepth != 2 {
			t.Fatalf("spawned child = %#v, want display depth 2 under system Goal", info)
		}
	})

	t.Run("Should rebase a child below a live system parent when its provenance root was deleted", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		root := createSession(t, h)
		goal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		})
		if err != nil {
			t.Fatalf("Create(system provenance parent) error = %v", err)
		}
		cleanupSessionStop(t, h, goal.ID)
		if err := h.manager.Delete(testutil.Context(t), root.ID); err != nil {
			t.Fatalf("Delete(provenance root) error = %v", err)
		}

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: goal.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(system parent with deleted provenance root) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		info := child.Info()
		if info.Type != SessionTypeSpawned || info.Lineage == nil ||
			info.Lineage.ParentSessionID != goal.ID || info.Lineage.RootSessionID != goal.ID ||
			info.Lineage.SpawnDepth != 2 {
			t.Fatalf("spawned child = %#v, want display depth 2 rebased below live system parent", info)
		}
	})

	t.Run("Should rebase when the provenance root is deleted after early create validation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		root := createSession(t, h)
		goal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		})
		if err != nil {
			t.Fatalf("Create(system provenance parent) error = %v", err)
		}
		cleanupSessionStop(t, h, goal.ID)

		ttl := time.Now().Add(time.Minute)
		spec, err := h.manager.prepareCreateStart(testutil.Context(t), spawnedCreateOpts(goal.ID, &store.SessionLineage{
			ParentSessionID: goal.ID,
			RootSessionID:   root.ID,
			SpawnDepth:      2,
			SpawnRole:       "coder",
			TTLExpiresAt:    &ttl,
		}))
		if err != nil {
			t.Fatalf("prepareCreateStart() error = %v", err)
		}
		if err := h.manager.Delete(testutil.Context(t), root.ID); err != nil {
			t.Fatalf("Delete(provenance root after early validation) error = %v", err)
		}

		child, err := h.manager.startSession(testutil.Context(t), &spec)
		if err != nil {
			t.Fatalf("startSession(after provenance deletion) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
		info := child.Info()
		if info.Lineage == nil || info.Lineage.ParentSessionID != goal.ID ||
			info.Lineage.RootSessionID != goal.ID {
			t.Fatalf("spawned child lineage = %#v, want rebased below live system parent", info.Lineage)
		}
	})

	t.Run("Should exclude provenance-only children from max children", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		root := createSession(t, h)
		cleanupSessionStop(t, h, root.ID)
		for range DefaultSpawnMaxChildren {
			goal, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
				ProvenanceParentSessionID: root.ID,
			})
			if err != nil {
				t.Fatalf("Create(system provenance child) error = %v", err)
			}
			cleanupSessionStop(t, h, goal.ID)
		}

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: root.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(with provenance siblings) error = %v", err)
		}
		cleanupSessionStop(t, h, child.ID)
	})

	t.Run("Should isolate max active counts by the nearest non-spawned parent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		root := createSession(t, h)
		cleanupSessionStop(t, h, root.ID)
		firstGoal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		})
		if err != nil {
			t.Fatalf("Create(first Goal) error = %v", err)
		}
		cleanupSessionStop(t, h, firstGoal.ID)
		secondGoal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: root.ID,
		})
		if err != nil {
			t.Fatalf("Create(second Goal) error = %v", err)
		}
		cleanupSessionStop(t, h, secondGoal.ID)
		spawned, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: firstGoal.ID, AgentName: "coder", TTL: time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(first Goal) error = %v", err)
		}
		cleanupSessionStop(t, h, spawned.ID)

		governance, err := h.manager.spawnGovernanceForParent(testutil.Context(t), secondGoal.Info())
		if err != nil {
			t.Fatalf("spawnGovernanceForParent(second Goal) error = %v", err)
		}
		err = h.manager.validateSpawnCaps(
			testutil.Context(t), secondGoal.Info(), governance,
			store.SessionSpawnBudget{
				MaxChildren: DefaultSpawnMaxChildren, MaxDepth: DefaultSpawnMaxDepth,
				MaxActivePerWorkspace: 1,
			},
			h.workspaceID,
		)
		if err != nil {
			t.Fatalf("validateSpawnCaps(second Goal) error = %v, want independent governed root", err)
		}
	})
}

func createSpawnParent(
	t *testing.T,
	h *harness,
	policy store.SessionPermissionPolicy,
	budget store.SessionSpawnBudget,
) *Session {
	t.Helper()

	parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "parent",
		Workspace: h.workspaceID,
		Type:      SessionTypeUser,
		Lineage: &store.SessionLineage{
			SpawnBudget:      budget,
			PermissionPolicy: policy,
		},
	})
	if err != nil {
		t.Fatalf("Create(parent) error = %v", err)
	}
	return parent
}

func cleanupSessionStop(t *testing.T, h *harness, sessionID string) {
	t.Helper()

	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), sessionID); err != nil {
			t.Fatalf("Stop(%s) error = %v", sessionID, err)
		}
	})
}

type recordingSessionSpawnHooks struct {
	preCreate      []hookspkg.SpawnPreCreatePayload
	created        []hookspkg.SpawnCreatedPayload
	preCreatePatch func(hookspkg.SpawnPreCreatePayload) hookspkg.SpawnPreCreatePayload
}

type recordingSpawnWorkspaceAccessPolicy struct {
	decision workspaceaccess.Decision
	err      error
	calls    int
	requests []workspaceaccess.Request
}

func (p *recordingSpawnWorkspaceAccessPolicy) Authorize(
	_ context.Context,
	req workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	p.calls++
	p.requests = append(p.requests, req)
	return p.decision, p.err
}

func (h *recordingSessionSpawnHooks) DispatchSpawnPreCreate(
	_ context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	h.preCreate = append(h.preCreate, payload)
	if h.preCreatePatch != nil {
		return h.preCreatePatch(payload), nil
	}
	return payload, nil
}

func (h *recordingSessionSpawnHooks) DispatchSpawnCreated(
	_ context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	h.created = append(h.created, payload)
	return payload, nil
}

func (h *recordingSessionSpawnHooks) DispatchSpawnParentStopped(
	_ context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return payload, nil
}

func (h *recordingSessionSpawnHooks) DispatchSpawnTTLExpired(
	_ context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return payload, nil
}

func (h *recordingSessionSpawnHooks) DispatchSpawnReaped(
	_ context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return payload, nil
}

func TestSpawnProviderCommandPrecedence(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name             string
		childCommand     string
		childProvider    string
		override         string
		want             string
		isolatedHome     bool
		isolatedEnv      bool
		foreignWorkspace bool
	}{
		{name: "Should inherit the resolved creator command for the same provider", childProvider: "claude", want: "account-b"},
		{name: "Should preserve an explicit child command", childProvider: "claude", childCommand: "account-c", want: "account-c"},
		{name: "Should preserve inheritance with an explicit same provider selection", childProvider: "claude", override: "claude", want: "account-b"},
		{name: "Should use the selected different provider command", childProvider: "claude", override: "codex", want: "test-acp-driver"},
		{name: "Should preserve a named child with a different provider", childProvider: "codex", want: "test-acp-driver"},
		{name: "Should keep isolated child home routing", childProvider: "claude", isolatedHome: true, want: "account-a"},
		{name: "Should keep changed child environment routing", childProvider: "claude", isolatedEnv: true, want: "account-a"},
		{name: "Should keep an authorized foreign workspace routing", childProvider: "claude", foreignWorkspace: true, want: "account-a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
			if err != nil {
				t.Fatal(err)
			}
			provider := workspace.Config.Providers["claude"]
			provider.Command = "account-a"
			workspace.Config.Providers["claude"] = provider
			workspace.Agents = append(
				workspace.Agents,
				compozyconfig.AgentDef{
					Name:     "parent",
					Provider: "claude",
					Command:  "account-b",
					Prompt:   "Delegate work.",
				},
				compozyconfig.AgentDef{
					Name:     "child",
					Provider: tt.childProvider,
					Command:  tt.childCommand,
					Prompt:   "Review work.",
				},
			)
			h.resolver.upsert(&workspace)
			parent, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "parent", Workspace: h.workspaceID})
			if err != nil {
				t.Fatal(err)
			}
			cleanupSessionStop(t, h, parent.ID)
			if tt.isolatedHome {
				provider.HomePolicy = compozyconfig.ProviderHomePolicyIsolated
			}
			if tt.isolatedEnv {
				provider.EnvPolicy = compozyconfig.ProviderEnvPolicyIsolated
			}
			workspace.Config.Providers["claude"] = provider
			if tt.foreignWorkspace {
				workspace.ID = "ws-foreign"
				h.manager.SetWorkspaceAccessPolicy(
					&recordingSpawnWorkspaceAccessPolicy{decision: workspaceaccess.Decision{Allowed: true}},
				)
			}
			h.resolver.upsert(&workspace)
			child, err := h.manager.Spawn(
				t.Context(),
				SpawnOpts{
					ParentSessionID: parent.ID,
					AgentName:       "child",
					Provider:        tt.override,
					Workspace:       workspace.ID,
					TTL:             time.Minute,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			cleanupSessionStop(t, h, child.ID)
			if got := h.driver.startCalls[1].Command; got != tt.want {
				t.Fatalf("spawn command = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpawnProviderRouteDiagnostics(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should correlate startup failure and selected route without duplicate keys or raw commands",
		func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "routing.jsonl")
			logFile, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := logFile.Close(); err != nil {
					t.Error(err)
				}
			})
			h := newHarness(t, WithLogger(slog.New(slog.NewJSONHandler(logFile, nil))))
			workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
			if err != nil {
				t.Fatal(err)
			}
			command := "env ROUTING_TEST_SECRET=private-routing-value account-b"
			workspace.Agents = append(workspace.Agents,
				compozyconfig.AgentDef{Name: "parent", Provider: "claude", Command: command, Prompt: "Delegate."},
				compozyconfig.AgentDef{Name: "child", Provider: "claude", Prompt: "Review."})
			h.resolver.upsert(&workspace)
			parent, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "parent", Workspace: h.workspaceID})
			if err != nil {
				t.Fatal(err)
			}
			cleanupSessionStop(t, h, parent.ID)
			denied := errors.New("authentication required")
			h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
				return nil, acp.WrapFailure(store.FailureProviderAuth, "native authentication failed", denied)
			}
			if _, err := h.manager.Spawn(
				t.Context(),
				SpawnOpts{ParentSessionID: parent.ID, AgentName: "child", TTL: time.Minute},
			); !errors.Is(
				err,
				denied,
			) {
				t.Fatalf("spawn error = %v, want authentication failure", err)
			}
			infos, err := h.manager.ListAll(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			childID := ""
			for _, info := range infos {
				if info.AgentName == "child" {
					childID = info.ID
					if info.State != StateStopped || info.Failure == nil {
						t.Fatalf("failed child = %#v", info)
					}
				}
			}
			if childID == "" {
				t.Fatal("failed child was not retained for inspection")
			}
			fingerprint := providerCommandFingerprint(command)
			marker := requireTranscriptMarker(t, h.manager, childID, transcript.MarkerProviderFailure)
			if marker.Evidence["provider_command_fingerprint"] != fingerprint {
				t.Fatalf("failure marker = %#v", marker)
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(contents), "private-routing-value") {
				t.Fatal("raw routing command leaked to logs")
			}
			selected, failed := false, false
			for line := range strings.SplitSeq(strings.TrimSpace(string(contents)), "\n") {
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}
				if record["session_id"] != childID {
					continue
				}
				message := record["msg"]
				if message != "session.provider_route.selected" && message != "session.start.driver_start_failed" {
					continue
				}
				if strings.Count(line, "\"provider_command_fingerprint\":") != 1 ||
					record["provider_command_fingerprint"] != fingerprint {
					t.Fatalf("ambiguous or incorrect route fingerprint: %s", line)
				}
				selected = selected || message == "session.provider_route.selected"
				failed = failed || message == "session.start.driver_start_failed"
			}
			if !selected || !failed {
				t.Fatalf("route logs selected=%t failed=%t", selected, failed)
			}
		},
	)
}
