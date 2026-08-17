package session

import (
	"context"
	"errors"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestCreateFailsWhenSessionPreCreateDenied(t *testing.T) {
	t.Parallel()

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "deny-create",
			Event:        hookspkg.HookSessionPreCreate,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"deny-create": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreCreatePayload) (hookspkg.SessionCreatePatch, error) {
					return hookspkg.SessionCreatePatch{
						ControlPatch: hookspkg.ControlPatch{
							Deny:       true,
							DenyReason: "blocked",
						},
					}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	_, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err == nil {
		t.Fatal("Create() error = nil, want pre-create denial")
	}
	if len(h.manager.List()) != 0 {
		t.Fatalf("List() = %d active sessions, want 0", len(h.manager.List()))
	}
	if got := h.notifier.createdCount(); got != 0 {
		t.Fatalf("created notifications = %d, want 0", got)
	}
}

func TestCreateUsesPatchedSessionPreCreatePayload(t *testing.T) {
	t.Parallel()

	const patchedName = "patched-session"
	sessionName := patchedName
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-create",
			Event:        hookspkg.HookSessionPreCreate,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-create": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreCreatePayload) (hookspkg.SessionCreatePatch, error) {
					sessionType := string(SessionTypeDream)
					return hookspkg.SessionCreatePatch{
						SessionName: &sessionName,
						SessionType: &sessionType,
					}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "original",
		Workspace: h.workspaceID,
		Type:      SessionTypeUser,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := session.Info().Name; got != patchedName {
		t.Fatalf("session name = %q, want %q", got, patchedName)
	}
	if got := session.Info().Type; got != SessionTypeDream {
		t.Fatalf("session type = %q, want %q", got, SessionTypeDream)
	}
	if got := h.driver.startCalls[0].Permissions; got != compozyconfig.PermissionModeApproveAll {
		t.Fatalf("start permissions = %q, want %q", got, compozyconfig.PermissionModeApproveAll)
	}
}

func TestCreateRejectsInvalidPreCreateProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Should reject provenance when pre-create changes the system type", func(t *testing.T) {
		t.Parallel()

		sessionType := string(SessionTypeUser)
		hooks := newNativeHookDispatcher(t,
			[]hookspkg.HookDecl{{
				Name:         "change-system-type",
				Event:        hookspkg.HookSessionPreCreate,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			}},
			map[string]hookspkg.Executor{
				"change-system-type": hookspkg.NewTypedNativeExecutor(
					func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreCreatePayload) (hookspkg.SessionCreatePatch, error) {
						return hookspkg.SessionCreatePatch{SessionType: &sessionType}, nil
					},
				),
			},
		)

		h := newHarness(t, WithHookSet(fullHookSet(hooks)))
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
			ProvenanceParentSessionID: parent.ID,
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Create(pre-create changed system type) error = %v, want %v", err, ErrValidation)
		}
	})
}

func TestCreateProtectsDaemonOwnedSessionTypeFromPreCreatePatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requestedType Type
		patchedType   Type
		lineage       func(time.Time) *store.SessionLineage
		spawnLineage  bool
	}{
		{
			name:          "Should reject changing a coordinator into a user session",
			requestedType: SessionTypeCoordinator,
			patchedType:   SessionTypeUser,
			lineage: func(ttl time.Time) *store.SessionLineage {
				return &store.SessionLineage{
					SpawnRole:    string(SessionTypeCoordinator),
					TTLExpiresAt: &ttl,
					SpawnBudget:  store.SessionSpawnBudget{MaxChildren: 1, MaxDepth: 1},
				}
			},
		},
		{
			name:          "Should reject changing a user into a coordinator session",
			requestedType: SessionTypeUser,
			patchedType:   SessionTypeCoordinator,
		},
		{
			name:          "Should reject changing a spawned session into a user session",
			requestedType: SessionTypeSpawned,
			patchedType:   SessionTypeUser,
			spawnLineage:  true,
		},
		{
			name:          "Should reject changing a user into a spawned session",
			requestedType: SessionTypeUser,
			patchedType:   SessionTypeSpawned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			patchedType := string(tt.patchedType)
			hooks := newNativeHookDispatcher(t,
				[]hookspkg.HookDecl{{
					Name:         "change-daemon-owned-type",
					Event:        hookspkg.HookSessionPreCreate,
					Mode:         hookspkg.HookModeSync,
					ExecutorKind: hookspkg.HookExecutorNative,
				}},
				map[string]hookspkg.Executor{
					"change-daemon-owned-type": hookspkg.NewTypedNativeExecutor(
						func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SessionPreCreatePayload) (hookspkg.SessionCreatePatch, error) {
							if payload.SessionName != "protected-type-target" {
								return hookspkg.SessionCreatePatch{}, nil
							}
							return hookspkg.SessionCreatePatch{SessionType: &patchedType}, nil
						},
					),
				},
			)
			h := newHarness(t, WithHookSet(fullHookSet(hooks)))
			var lineage *store.SessionLineage
			if tt.lineage != nil {
				lineage = tt.lineage(time.Now().Add(time.Hour))
			}
			if tt.spawnLineage {
				parent := createSession(t, h)
				t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
				ttl := time.Now().Add(time.Hour)
				lineage = &store.SessionLineage{
					ParentSessionID: parent.ID,
					RootSessionID:   parent.ID,
					SpawnDepth:      1,
					SpawnRole:       "coder",
					TTLExpiresAt:    &ttl,
				}
			}
			startCallsBeforeCreate := len(h.driver.startCalls)

			_, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder",
				Name:      "protected-type-target",
				Workspace: h.workspaceID,
				Type:      tt.requestedType,
				Lineage:   lineage,
			})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf(
					"Create(type transition %q -> %q) error = %v, want %v",
					t.requestedType,
					t.patchedType,
					err,
					ErrValidation,
				)
			}
			if got := len(h.driver.startCalls); got != startCallsBeforeCreate {
				t.Fatalf("driver start calls = %d, want unchanged %d", got, startCallsBeforeCreate)
			}
		})
	}
}

func TestPostCreateHookFiresAfterSessionActive(t *testing.T) {
	t.Parallel()

	payloadCh := make(chan hookspkg.SessionPostCreatePayload, 1)
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "observe-post-create",
			Event:        hookspkg.HookSessionPostCreate,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"observe-post-create": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePatch, error) {
					payloadCh <- payload
					return hookspkg.SessionPostCreatePatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	select {
	case payload := <-payloadCh:
		if payload.SessionID != session.ID {
			t.Fatalf("payload.SessionID = %q, want %q", payload.SessionID, session.ID)
		}
		if payload.State != string(StateActive) {
			t.Fatalf("payload.State = %q, want %q", payload.State, StateActive)
		}
		if payload.ACPSessionID == "" {
			t.Fatal("payload.ACPSessionID = empty, want active ACP session id")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session.post_create hook")
	}
}

func TestPostCreateAsyncHookSurvivesRequestCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	observedErr := make(chan error, 1)
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "observe-post-create-after-request",
			Event:        hookspkg.HookSessionPostCreate,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"observe-post-create-after-request": hookspkg.NewTypedNativeExecutor(
				func(ctx context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePatch, error) {
					<-release
					observedErr <- ctx.Err()
					return hookspkg.SessionPostCreatePatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	requestCtx, cancelRequest := context.WithCancel(testutil.Context(t))
	session, err := h.manager.Create(requestCtx, CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	cancelRequest()
	close(release)

	select {
	case err := <-observedErr:
		if err != nil {
			t.Fatalf("async post-create context error = %v, want nil after request cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async session.post_create hook")
	}
}
