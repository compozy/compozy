package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

const (
	testToolEdit  = "agh__edit"
	testToolRead  = "agh__read"
	testToolShell = "agh__shell"
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

func TestManagerSpawnCreatesChildWithDurableLineageAndNarrowPermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should create child with durable lineage and narrowed permissions", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		h := newHarness(t, WithNow(func() time.Time { return now }))
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
			!info.Lineage.AutoStopOnParent {
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
		if meta.Lineage == nil || meta.Lineage.ParentSessionID != parent.ID {
			t.Fatalf("persisted lineage = %#v, want parent %q", meta.Lineage, parent.ID)
		}
		if len(h.driver.startCalls) < 2 ||
			!strings.Contains(h.driver.startCalls[len(h.driver.startCalls)-1].SystemPrompt, "Focus only on tests.") {
			t.Fatalf("child prompt overlay was not appended to start prompt: %#v", h.driver.startCalls)
		}
	})
	t.Run("Should keep children local by default and enforce their delegated channel scope", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithParticipationResolver(newTestSessionParticipationResolver(t, true)))
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

		h := newHarness(t, WithParticipationResolver(newTestSessionParticipationResolver(t, true)))
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
				_, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
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

			h := newHarness(t)
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

		h := newHarness(t)
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

		h := newHarness(t)
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

		h := newHarness(t)
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

		h := newHarness(t)
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
		h := newHarness(t, WithHookSet(HookSet{Spawn: hooks}))
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
		h := newHarness(t, WithHookSet(HookSet{Spawn: hooks}))
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
