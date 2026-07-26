package loop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func TestStartBindingShouldDeriveActorsForEverySurface(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		kind       dsl.StartKind
		wantActor  task.ActorKind
		wantOrigin task.OriginKind
	}{
		{
			name:       "Should derive manual as web human",
			kind:       dsl.StartManual,
			wantActor:  task.ActorKindHuman,
			wantOrigin: task.OriginKindWeb,
		},
		{
			name:       "Should derive CLI as CLI human",
			kind:       dsl.StartCLI,
			wantActor:  task.ActorKindHuman,
			wantOrigin: task.OriginKindCLI,
		},
		{
			name:       "Should derive HTTP as HTTP human",
			kind:       dsl.StartHTTP,
			wantActor:  task.ActorKindHuman,
			wantOrigin: task.OriginKindHTTP,
		},
		{
			name:       "Should derive UDS as UDS human",
			kind:       dsl.StartUDS,
			wantActor:  task.ActorKindHuman,
			wantOrigin: task.OriginKindUDS,
		},
		{
			name:       "Should derive trigger as automation",
			kind:       dsl.StartTrigger,
			wantActor:  task.ActorKindAutomation,
			wantOrigin: task.OriginKindAutomation,
		},
		{
			name:       "Should derive schedule as automation",
			kind:       dsl.StartSchedule,
			wantActor:  task.ActorKindAutomation,
			wantOrigin: task.OriginKindAutomation,
		},
		{
			name:       "Should derive webhook as automation",
			kind:       dsl.StartWebhook,
			wantActor:  task.ActorKindAutomation,
			wantOrigin: task.OriginKindAutomation,
		},
		{
			name:       "Should derive network as network peer",
			kind:       dsl.StartNetwork,
			wantActor:  task.ActorKindNetworkPeer,
			wantOrigin: task.OriginKindNetwork,
		},
		{
			name:       "Should derive extension as extension",
			kind:       dsl.StartExtension,
			wantActor:  task.ActorKindExtension,
			wantOrigin: task.OriginKindExtension,
		},
		{
			name:       "Should derive native tool as agent session",
			kind:       dsl.StartNativeTool,
			wantActor:  task.ActorKindAgentSession,
			wantOrigin: task.OriginKindAgentSession,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actor, err := loop.DeriveStartActor(tt.kind, "ws-1", "actor-ref", "origin-ref")
			if err != nil {
				t.Fatalf("DeriveStartActor(%q) error = %v", tt.kind, err)
			}
			if actor.Actor.Kind != tt.wantActor {
				t.Fatalf("Actor.Kind = %q, want %q", actor.Actor.Kind, tt.wantActor)
			}
			if actor.Origin.Kind != tt.wantOrigin {
				t.Fatalf("Origin.Kind = %q, want %q", actor.Origin.Kind, tt.wantOrigin)
			}
		})
	}
}

func TestStartBindingShouldValidateAllowlistAndResolveMappedInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve static and trigger payload mapped inputs", func(t *testing.T) {
		t.Parallel()

		def := startBindingDefinition()
		resolver := startBindingResolver(t, def)
		values, err := loop.ResolveStartTargetInputs(context.Background(), resolver, loop.StartTargetResolution{
			StartTargetValidation: loop.StartTargetValidation{
				WorkspaceID: "ws-1",
				LoopName:    "valid-loop",
				Kind:        dsl.StartTrigger,
				Inputs: map[string]any{
					"tasks": "task-ref",
					"count": 2,
				},
				InputMapping: map[string]string{
					"title": "{{ .trigger.payload.title }}",
				},
			},
			TriggerPayload: map[string]any{"title": "mapped-title"},
		})
		if err != nil {
			t.Fatalf("ResolveStartTargetInputs() error = %v", err)
		}
		if values["title"] != "mapped-title" {
			t.Fatalf("resolved title = %#v, want mapped-title", values["title"])
		}
		if values["count"] != 2 {
			t.Fatalf("resolved count = %#v, want 2", values["count"])
		}
	})

	t.Run("Should reject undeclared start surface with reason code", func(t *testing.T) {
		t.Parallel()

		err := loop.ValidateStartTarget(context.Background(), startBindingResolver(t, startBindingDefinition()),
			loop.StartTargetValidation{
				WorkspaceID: "ws-1",
				LoopName:    "valid-loop",
				Kind:        dsl.StartHTTP,
				Inputs:      map[string]any{"tasks": "task-ref"},
			})
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeStartKindNotAllowed {
			t.Fatalf("ValidateStartTarget() error = %v, want %q", err, loop.ReasonCodeStartKindNotAllowed)
		}
	})

	t.Run("Should reject missing required mapped input before start", func(t *testing.T) {
		t.Parallel()

		err := loop.ValidateStartTarget(context.Background(), startBindingResolver(t, startBindingDefinition()),
			loop.StartTargetValidation{
				WorkspaceID: "ws-1",
				LoopName:    "valid-loop",
				Kind:        dsl.StartWebhook,
				Inputs:      map[string]any{"tasks": "task-ref"},
			})
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeStartInputInvalid {
			t.Fatalf("ValidateStartTarget() error = %v, want %q", err, loop.ReasonCodeStartInputInvalid)
		}
	})

	t.Run("Should reject non payload mapping grammar", func(t *testing.T) {
		t.Parallel()

		err := loop.ValidateStartTarget(context.Background(), startBindingResolver(t, startBindingDefinition()),
			loop.StartTargetValidation{
				WorkspaceID: "ws-1",
				LoopName:    "valid-loop",
				Kind:        dsl.StartTrigger,
				Inputs: map[string]any{
					"tasks": "task-ref",
					"count": 1,
				},
				InputMapping: map[string]string{
					"title": "{{ .inputs.title }}",
				},
			})
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeStartInputMappingInvalid {
			t.Fatalf("ValidateStartTarget() error = %v, want %q", err, loop.ReasonCodeStartInputMappingInvalid)
		}
	})
}

func TestStartBindingShouldStartThroughServiceForEveryDirectSurface(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind dsl.StartKind
	}{
		{name: "Should start manually", kind: dsl.StartManual},
		{name: "Should start from CLI", kind: dsl.StartCLI},
		{name: "Should start from HTTP", kind: dsl.StartHTTP},
		{name: "Should start from UDS", kind: dsl.StartUDS},
		{name: "Should start from network", kind: dsl.StartNetwork},
		{name: "Should start from extension", kind: dsl.StartExtension},
		{name: "Should start from native tool", kind: dsl.StartNativeTool},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def := startBindingDefinition()
			def.Start = append(def.Start, dsl.StartBinding{Kind: tt.kind})
			store := newFakeLoopStore()
			resolver := startBindingResolver(t, def)
			svc := newTestService(t, store, def)
			run, err := loop.StartFromBinding(context.Background(), svc, resolver, loop.StartBindingRequest{
				WorkspaceID: "ws-1",
				LoopName:    "valid-loop",
				Kind:        tt.kind,
				ActorRef:    "actor-ref",
				OriginRef:   "origin-ref",
				Inputs: loop.Inputs{
					Values: map[string]any{
						"tasks": "task-ref",
						"title": "direct-title",
						"count": 1,
					},
				},
			})
			if err != nil {
				t.Fatalf("StartFromBinding(%q) error = %v", tt.kind, err)
			}
			if run.ID == "" {
				t.Fatal("StartFromBinding() run ID is empty")
			}
			if got := store.createCount(); got != 1 {
				t.Fatalf("CreateLoopRunForStart calls = %d, want 1", got)
			}
		})
	}
}

func startBindingDefinition() dsl.Definition {
	def := validDefinition()
	def.Inputs = map[string]dsl.Input{
		"tasks": {Type: dsl.InputTypeRef, Required: true},
		"title": {Type: dsl.InputTypeString, Required: true},
		"count": {Type: dsl.InputTypeNumber, Required: true},
	}
	def.Start = []dsl.StartBinding{
		{Kind: dsl.StartManual},
		{Kind: dsl.StartTrigger, InputMapping: map[string]string{"title": "{{ .trigger.payload.title }}"}},
		{Kind: dsl.StartWebhook, InputMapping: map[string]string{"title": "{{ .trigger.payload.title }}"}},
	}
	return def
}

func startBindingResolver(t *testing.T, def dsl.Definition) loop.DefinitionResolver {
	t.Helper()
	resolved := compileDefinition(t, def)
	return loop.DefinitionResolverFunc(
		func(context.Context, loop.WorkspaceID, string) (*loop.ResolvedDefinition, error) {
			return resolved, nil
		},
	)
}
