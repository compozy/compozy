package session

import (
	"context"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/testutil"
)

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

func TestInputPreSubmitAttachmentMetadata(t *testing.T) {
	t.Run("Should expose byte-free attachment metadata to input pre-submit hooks", func(t *testing.T) {
		t.Parallel()

		metadata := hookAttachmentMetadata([]AttachmentMeta{{
			ID: "att_123", Name: "diagram.png", MIMEType: "image/png", Bytes: 42,
			SHA256: "not-exposed", Kind: AttachmentKindImage, Width: 10, Height: 20,
		}})
		if len(metadata) != 1 {
			t.Fatalf("len(hookAttachmentMetadata()) = %d, want 1", len(metadata))
		}
		want := hookspkg.InputAttachmentMetadata{
			ID: "att_123", Name: "diagram.png", MIME: "image/png", Bytes: 42, Kind: AttachmentKindImage,
		}
		if metadata[0] != want {
			t.Fatalf("hookAttachmentMetadata() = %#v, want %#v", metadata[0], want)
		}
	})
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
	enableSyntheticQueue(t, h, session)
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
					return hookspkg.PromptPatch{Prompt: new("patched system prompt")}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	t.Run("Should record the hook-modified startup manifest", func(t *testing.T) {
		manifest := h.driver.startCalls[0].StartupManifest
		if !manifest.HookModified || len(manifest.Spans) != 1 || manifest.Spans[0].Key != "system_prompt" ||
			!manifest.Spans[0].HookModified ||
			manifest.Spans[0].Bytes != int64(len("patched system prompt")) {
			t.Fatalf("patched manifest = %#v", manifest)
		}
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
						return hookspkg.PromptPatch{Prompt: new("patched system prompt")}, nil
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

// Invariant: input.pre_submit sees the admitted mentions, and after a rewrite only admitted mentions whose
// token is still present survive; a hook can never add one (Safety Invariant 4).
// Owner: session input.pre_submit hook suite.
func TestInputPreSubmitMentions(t *testing.T) {
	t.Parallel()

	t.Run("Should keep only admitted mentions still present after an input hook rewrite", func(t *testing.T) {
		t.Parallel()
		dispatcher := &spyHookDispatcher{}
		var seen []hookspkg.InputMentionMetadata
		dispatcher.dispatchInputPreSubmitFn = func(
			_ context.Context, payload hookspkg.InputPreSubmitPayload,
		) (hookspkg.InputPreSubmitPayload, error) {
			seen = payload.Mentions
			payload.Message = "only @a.go and now @c.go"
			return payload, nil
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		writeMentionFixture(t, h.workspace, "a.go", "b.go", "c.go")
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })

		result, err := h.manager.SendPrompt(t.Context(), sess.ID, SendPromptOpts{Message: "@a.go @b.go"})
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, result.Events)
		if len(seen) != 2 || seen[0].Target != "a.go" || seen[1].Target != "b.go" {
			t.Fatalf("hook mentions = %#v, want a.go and b.go", seen)
		}
		calls := managerPromptCalls(h)
		if len(calls) != 1 || len(calls[0].MentionLinks) != 1 || calls[0].MentionLinks[0].Name != "a.go" {
			t.Fatalf("links after rewrite = %#v, want only a.go", calls)
		}
		stored := storedInputMentions(t, sess)
		if got := mentionTargets(stored); !slices.Equal(got, []string{"a.go"}) {
			t.Fatalf("persisted mentions after rewrite = %v, want a.go", got)
		}
		// The transcript displays the authored text, so offsets must keep pointing at its token.
		if stored[0].Start != 0 || stored[0].End != len("@a.go") {
			t.Fatalf(
				"persisted a.go offsets = [%d,%d), want [0,5) in the authored text",
				stored[0].Start,
				stored[0].End,
			)
		}
	})
}
