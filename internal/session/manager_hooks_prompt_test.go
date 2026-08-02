package session

import (
	"context"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/testutil"
)

func TestPromptNetworkUsesNetworkInputClass(t *testing.T) {
	t.Parallel()

	t.Run("ShouldUseNetworkInputClass", func(t *testing.T) {
		dispatcher := &spyHookDispatcher{}
		var (
			inputPayload     hookspkg.InputPreSubmitPayload
			turnStartPayload hookspkg.TurnStartPayload
		)
		dispatcher.dispatchInputPreSubmitFn = func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
			inputPayload = payload
			return payload, nil
		}
		dispatcher.dispatchTurnStartFn = func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			turnStartPayload = payload
			return payload, nil
		}

		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session := createLiveNetworkSession(t, h)
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		eventsCh, err := h.manager.PromptNetwork(testutil.Context(t), session.ID, "network message")
		if err != nil {
			t.Fatalf("PromptNetwork() error = %v", err)
		}
		_ = collectEvents(t, eventsCh)

		if inputPayload.InputClass != hookInputClassNetworkMessage {
			t.Fatalf(
				"input.pre_submit input class = %q, want %q",
				inputPayload.InputClass,
				hookInputClassNetworkMessage,
			)
		}
		if turnStartPayload.InputClass != hookInputClassNetworkMessage {
			t.Fatalf("turn.start input class = %q, want %q", turnStartPayload.InputClass, hookInputClassNetworkMessage)
		}
		if turnStartPayload.UserMessage != "network message" {
			t.Fatalf("turn.start user message = %q, want %q", turnStartPayload.UserMessage, "network message")
		}
	})
}

func TestNewPromptTurnDispatchStateNormalizesInputClass(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		turnSource     TurnSource
		wantTurnSource TurnSource
		wantInputClass string
	}{
		{
			name:           "unknown source falls back to user",
			turnSource:     TurnSource("unexpected"),
			wantTurnSource: TurnSourceUser,
			wantInputClass: hookInputClassUserMessage,
		},
		{
			name:           "synthetic source keeps synthetic class",
			turnSource:     TurnSourceSynthetic,
			wantTurnSource: TurnSourceSynthetic,
			wantInputClass: hookInputClassSynthetic,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := newPromptTurnDispatchState(nil, " turn-1 ", tc.turnSource, "message")
			if got := state.turnSource; got != tc.wantTurnSource {
				t.Fatalf("turnSource = %q, want %q", got, tc.wantTurnSource)
			}
			if got := state.inputClass; got != tc.wantInputClass {
				t.Fatalf("inputClass = %q, want %q", got, tc.wantInputClass)
			}
			if got := state.turnID; got != "turn-1" {
				t.Fatalf("turnID = %q, want %q", got, "turn-1")
			}
		})
	}
}

func TestPromptSyntheticUsesSyntheticInputClass(t *testing.T) {
	t.Parallel()

	dispatcher := &spyHookDispatcher{}
	var (
		inputPayload     hookspkg.InputPreSubmitPayload
		turnStartPayload hookspkg.TurnStartPayload
	)
	dispatcher.dispatchInputPreSubmitFn = func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
		inputPayload = payload
		return payload, nil
	}
	dispatcher.dispatchTurnStartFn = func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
		turnStartPayload = payload
		return payload, nil
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "daemon wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
			Reason:    "task_run_completed",
			Summary:   "background work finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	if inputPayload.InputClass != hookInputClassSynthetic {
		t.Fatalf(
			"input.pre_submit input class = %q, want %q",
			inputPayload.InputClass,
			hookInputClassSynthetic,
		)
	}
	if turnStartPayload.InputClass != hookInputClassSynthetic {
		t.Fatalf("turn.start input class = %q, want %q", turnStartPayload.InputClass, hookInputClassSynthetic)
	}
	if turnStartPayload.UserMessage != "daemon wake-up" {
		t.Fatalf("turn.start user message = %q, want %q", turnStartPayload.UserMessage, "daemon wake-up")
	}
}

func TestCreateUsesPatchedPrompt(t *testing.T) {
	t.Parallel()

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-prompt",
			Event:        hookspkg.HookPromptPostAssemble,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-prompt": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.PromptPayload) (hookspkg.PromptPatch, error) {
					prompt := "patched system prompt"
					return hookspkg.PromptPatch{Prompt: &prompt}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := h.driver.startCalls[0].SystemPrompt; got != "patched system prompt" {
		t.Fatalf("start system prompt = %q, want %q", got, "patched system prompt")
	}
}

func TestCreateAppliesStartupPromptOverlayAfterPromptPatch(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve daemon startup overlay after prompt hook replacement", func(t *testing.T) {
		t.Parallel()

		hooks := newNativeHookDispatcher(t,
			[]hookspkg.HookDecl{{
				Name:         "patch-prompt",
				Event:        hookspkg.HookPromptPostAssemble,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			}},
			map[string]hookspkg.Executor{
				"patch-prompt": hookspkg.NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ hookspkg.RegisteredHook,
						_ hookspkg.PromptPayload,
					) (hookspkg.PromptPatch, error) {
						prompt := "patched system prompt"
						return hookspkg.PromptPatch{Prompt: &prompt}, nil
					},
				),
			},
		)

		h := newHarness(
			t,
			WithHookSet(fullHookSet(hooks)),
			WithStartupPromptOverlay(
				startupPromptOverlayFunc(func(
					_ context.Context,
					_ StartupPromptContext,
					prompt string,
				) (string, error) {
					return "protected runtime envelope\n\n" + prompt, nil
				}),
			),
		)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		got := h.driver.startCalls[0].SystemPrompt
		want := "protected runtime envelope\n\npatched system prompt"
		if got != want {
			t.Fatalf("start system prompt = %q, want %q", got, want)
		}
	})
}
