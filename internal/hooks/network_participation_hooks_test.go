package hooks

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestNetworkParticipationHooks(t *testing.T) {
	t.Parallel()

	t.Run("Should deny pre_resolve when patch Deny is set", func(t *testing.T) {
		t.Parallel()

		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "deny-participation",
				Event:        HookNetworkParticipationPreResolve,
				Mode:         HookModeSync,
				Matcher:      HookMatcher{},
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"deny-participation": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ NetworkParticipationPreResolvePayload,
					) (NetworkParticipationPreResolvePatch, error) {
						return NetworkParticipationPreResolvePatch{
							ControlPatch: ControlPatch{Deny: true, DenyReason: "policy"},
						}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		_, err := hooks.DispatchNetworkParticipationPreResolve(
			t.Context(),
			NetworkParticipationPreResolvePayload{WorkspaceID: "ws-alpha"},
		)
		if err == nil {
			t.Fatal("expected deny error")
		}
		if !strings.Contains(err.Error(), "denied") {
			t.Fatalf("error = %v, want denied diagnostic", err)
		}
	})

	t.Run("Should derive and reject semantic widening without a caller supplied flag", func(t *testing.T) {
		t.Parallel()
		live := participation.ModeLive
		named := participation.StrategyNamed
		channel := "ops"

		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "widen-participation",
				Event:        HookNetworkParticipationPreResolve,
				Mode:         HookModeSync,
				Matcher:      HookMatcher{},
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"widen-participation": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ NetworkParticipationPreResolvePayload,
					) (NetworkParticipationPreResolvePatch, error) {
						return NetworkParticipationPreResolvePatch{Request: &participation.Request{
							Mode:            &live,
							ChannelStrategy: &named,
							ChannelID:       &channel,
						}}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		local := participation.ModeLocal
		got, err := hooks.DispatchNetworkParticipationPreResolve(
			t.Context(),
			NetworkParticipationPreResolvePayload{
				WorkspaceID: "ws-alpha",
				Request:     &participation.Request{Mode: &local},
			},
		)
		if err != nil {
			t.Fatalf("DispatchNetworkParticipationPreResolve() error = %v", err)
		}
		if got.Request == nil || got.Request.Mode == nil || *got.Request.Mode != participation.ModeLocal {
			t.Fatalf("request = %#v, want original local mode", got.Request)
		}
	})

	t.Run("Should narrow request on pre_resolve without widening", func(t *testing.T) {
		t.Parallel()

		mode := participation.ModeLocal
		narrowed := &participation.Request{Mode: &mode}
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "narrow-participation",
				Event:        HookNetworkParticipationPreResolve,
				Mode:         HookModeSync,
				Matcher:      HookMatcher{},
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"narrow-participation": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ NetworkParticipationPreResolvePayload,
					) (NetworkParticipationPreResolvePatch, error) {
						return NetworkParticipationPreResolvePatch{Request: narrowed}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		live := participation.ModeLive
		named := participation.StrategyNamed
		channel := "ops"
		got, err := hooks.DispatchNetworkParticipationPreResolve(
			t.Context(),
			NetworkParticipationPreResolvePayload{
				WorkspaceID: "ws-alpha",
				Request: &participation.Request{
					Mode:            &live,
					ChannelStrategy: &named,
					ChannelID:       &channel,
				},
			},
		)
		if err != nil {
			t.Fatalf("DispatchNetworkParticipationPreResolve() error = %v", err)
		}
		if got.Request == nil || got.Request.Mode == nil || *got.Request.Mode != participation.ModeLocal {
			t.Fatalf("request = %#v, want local mode", got.Request)
		}
	})

	t.Run("Should match participation events by workspace mode channel and source", func(t *testing.T) {
		t.Parallel()

		matcher := HookMatcher{
			WorkspaceID: "ws-alpha",
			NetworkMatcher: &NetworkMatcher{
				Channel:             "ops",
				ParticipationMode:   "live",
				ParticipationSource: "explicit_request",
			},
		}
		live := participation.ModeLive
		named := participation.StrategyNamed
		channel := "ops"
		preResolve := NetworkParticipationPreResolvePayload{
			WorkspaceID: "ws-alpha",
			Request: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &channel,
			},
			Source: participation.SourceExplicitRequest,
		}
		resolved := NetworkParticipationResolvedPayload{
			WorkspaceID: "ws-alpha",
			Spec: participation.Spec{
				Version:         participation.SpecVersion,
				Mode:            participation.ModeLive,
				WorkspaceID:     "ws-alpha",
				ChannelStrategy: participation.StrategyNamed,
				ChannelID:       "ops",
				Source:          participation.SourceExplicitRequest,
			},
		}
		if !matchNetworkParticipationPreResolve(matcher, preResolve) {
			t.Fatal("matchNetworkParticipationPreResolve() = false, want true")
		}
		if !matchNetworkParticipationResolved(matcher, resolved) {
			t.Fatal("matchNetworkParticipationResolved() = false, want true")
		}

		mismatches := []struct {
			name   string
			mutate func(*HookMatcher)
		}{
			{name: "Should reject another workspace", mutate: func(candidate *HookMatcher) {
				candidate.WorkspaceID = "ws-other"
			}},
			{name: "Should reject another mode", mutate: func(candidate *HookMatcher) {
				candidate.ParticipationMode = "local"
			}},
			{name: "Should reject another channel", mutate: func(candidate *HookMatcher) {
				candidate.Channel = "other"
			}},
			{name: "Should reject another source", mutate: func(candidate *HookMatcher) {
				candidate.ParticipationSource = "task_profile"
			}},
		}
		for _, testCase := range mismatches {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				candidate := matcher
				network := *matcher.NetworkMatcher
				candidate.NetworkMatcher = &network
				testCase.mutate(&candidate)
				if matchNetworkParticipationPreResolve(candidate, preResolve) {
					t.Fatal("matchNetworkParticipationPreResolve() = true, want false")
				}
				if matchNetworkParticipationResolved(candidate, resolved) {
					t.Fatal("matchNetworkParticipationResolved() = true, want false")
				}
			})
		}
	})

	t.Run("Should dispatch resolved payload with Spec object", func(t *testing.T) {
		t.Parallel()

		seen := make(chan participation.Spec, 1)
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "observe-resolved",
				Event:        HookNetworkParticipationResolved,
				Mode:         HookModeAsync,
				Matcher:      HookMatcher{},
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"observe-resolved": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						payload NetworkParticipationResolvedPayload,
					) (NetworkParticipationResolvedPatch, error) {
						seen <- payload.Spec
						return NetworkParticipationResolvedPatch{}, nil
					},
				),
			})),
			WithAsyncWorkerCount(1),
			WithAsyncQueueCapacity(1),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		spec := participation.Spec{
			Version: participation.SpecVersion,
			Mode:    participation.ModeLocal,
			Source:  participation.SourceBuiltInLocal,
		}
		if _, err := hooks.DispatchNetworkParticipationResolved(
			t.Context(),
			NetworkParticipationResolvedPayload{
				WorkspaceID: "ws-alpha",
				Spec:        spec,
			},
		); err != nil {
			t.Fatalf("DispatchNetworkParticipationResolved() error = %v", err)
		}
		select {
		case got := <-seen:
			if got.Mode != participation.ModeLocal {
				t.Fatalf("resolved spec = %#v, want local", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for resolved observation")
		}
	})
}
