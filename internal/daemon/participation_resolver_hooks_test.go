package daemon

import (
	"context"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
)

func TestHookAwareParticipationResolverUsesSelectedIntent(t *testing.T) {
	t.Parallel()

	t.Run("Should match a task profile before resolution", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultNetworkConfig()
		resolver, err := participation.NewResolver(participation.ResolverOptions{
			Defaults: cfg.Live.Defaults.ParticipationBounds(),
			Limits:   cfg.Live.Limits.ParticipationLimits(),
			Availability: func(context.Context) (bool, error) {
				return true, nil
			},
			ChannelExists: func(context.Context, string, string) (bool, error) {
				return true, nil
			},
		})
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}

		called := make(chan struct{}, 1)
		hooks := hookspkg.NewHooks(
			hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
				Name:  "task-profile-policy",
				Event: hookspkg.HookNetworkParticipationPreResolve,
				Mode:  hookspkg.HookModeSync,
				Matcher: hookspkg.HookMatcher{
					WorkspaceID: "ws-alpha",
					NetworkMatcher: &hookspkg.NetworkMatcher{
						Channel:             "ops",
						ParticipationMode:   "live",
						ParticipationSource: "task_profile",
					},
				},
				ExecutorKind: hookspkg.HookExecutorNative,
			}}),
			hookspkg.WithExecutorResolver(func(hookspkg.HookDecl) (hookspkg.Executor, error) {
				return hookspkg.NewTypedNativeExecutor(
					func(
						context.Context,
						hookspkg.RegisteredHook,
						hookspkg.NetworkParticipationPreResolvePayload,
					) (hookspkg.NetworkParticipationPreResolvePatch, error) {
						called <- struct{}{}
						return hookspkg.NetworkParticipationPreResolvePatch{
							ControlPatch: hookspkg.ControlPatch{
								Deny:       true,
								DenyReason: "task profile hook executed",
							},
						}, nil
					},
				), nil
			}),
		)
		t.Cleanup(hooks.Close)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		mode := participation.ModeLive
		strategy := participation.StrategyNamed
		channel := "ops"
		wrapped := wrapParticipationResolverWithHooks(resolver, hooks)
		_, err = wrapped.Resolve(t.Context(), participation.ResolveInput{
			WorkspaceID: "ws-alpha",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-alpha",
				Kind:        participation.OwnerKindTaskRun,
				ID:          "run-alpha",
			},
			Definition: &participation.Request{
				Mode:            &mode,
				ChannelStrategy: &strategy,
				ChannelID:       &channel,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "task profile hook executed") {
			t.Fatalf("Resolve() error = %v, want matched task profile hook error", err)
		}
		select {
		case <-called:
		default:
			t.Fatal("task profile hook was not executed")
		}
	})
}
