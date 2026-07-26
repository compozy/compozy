package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestNewAutomationLoopStarter(t *testing.T) {
	t.Parallel()

	t.Run("Should reject non Loop stores when Loop catalog is configured", func(t *testing.T) {
		t.Parallel()

		starter, err := newAutomationLoopStarter(
			struct{}{},
			newResourceCatalog(looppkg.CloneResourceSpec),
			nil,
			aghconfig.HomePaths{},
			nil,
			nil,
		)

		if err == nil {
			t.Fatal("newAutomationLoopStarter() error = nil, want loop store requirement")
		}
		if starter != nil {
			t.Fatalf("newAutomationLoopStarter() starter = %#v, want nil", starter)
		}
		if !strings.Contains(err.Error(), "requires loop store") {
			t.Fatalf("newAutomationLoopStarter() error = %q, want loop store requirement", err)
		}
	})
}

func TestAutomationLoopStartMetadataShouldIncludeCatchUpEvidence(t *testing.T) {
	t.Parallel()

	t.Run("Should include catch-up evidence in Loop start metadata", func(t *testing.T) {
		t.Parallel()

		scheduledAt := time.Date(2026, 7, 7, 9, 45, 0, 0, time.UTC)
		metadata := automationLoopStartMetadata(automationpkg.LoopStartRequest{
			AutomationRunID: "autorun-1",
			ScheduledAt:     &scheduledAt,
			CatchUp:         true,
			CatchUpPolicy:   automationpkg.SchedulerCatchUpPolicyCoalesce,
		})

		if got, want := metadata["automation_run_id"], "autorun-1"; got != want {
			t.Fatalf("automation_run_id = %#v, want %q", got, want)
		}
		if got, want := metadata["scheduled_at"], scheduledAt.Format(time.RFC3339Nano); got != want {
			t.Fatalf("scheduled_at = %#v, want %q", got, want)
		}
		if got, want := metadata["original_due_at"], scheduledAt.Format(time.RFC3339Nano); got != want {
			t.Fatalf("original_due_at = %#v, want %q", got, want)
		}
		if got, want := metadata["catch_up"], true; got != want {
			t.Fatalf("catch_up = %#v, want %v", got, want)
		}
		if got, want := metadata["catch_up_policy"], "coalesce"; got != want {
			t.Fatalf("catch_up_policy = %#v, want %q", got, want)
		}
	})
}

func TestAutomationLoopStarterShouldForwardDefinitionParticipationAsAutomationIntent(t *testing.T) {
	t.Parallel()

	t.Run("Should forward definition participation as Automation intent", func(t *testing.T) {
		t.Parallel()

		service := &recordingAutomationLoopService{}
		starter := &automationLoopStarter{
			service: service,
			resolver: looppkg.DefinitionResolverFunc(
				func(context.Context, looppkg.WorkspaceID, string) (*looppkg.ResolvedDefinition, error) {
					return &looppkg.ResolvedDefinition{Definition: loopdsl.Definition{
						DefinitionExtensionState: &loopdsl.DefinitionExtensionState{
							Start: []loopdsl.StartBinding{{Kind: loopdsl.StartWebhook}},
						},
					}}, nil
				},
			),
		}
		request := automationLoopNamedParticipation("deployments")
		actor, err := taskpkg.DeriveAutomationActorContext("trigger-1", "run:autorun-1")
		if err != nil {
			t.Fatalf("DeriveAutomationActorContext() error = %v", err)
		}

		result, err := starter.StartLoop(context.Background(), automationpkg.LoopStartRequest{
			WorkspaceID:          "ws-1",
			LoopName:             "deploy",
			Kind:                 automationpkg.LoopStartKindWebhook,
			Actor:                actor,
			AutomationRunID:      "autorun-1",
			NetworkParticipation: request,
		})
		if err != nil {
			t.Fatalf("StartLoop() error = %v", err)
		}
		if got, want := result.RunID, "looprun-automation"; got != want {
			t.Fatalf("StartLoop().RunID = %q, want %q", got, want)
		}
		if service.inputs.NetworkParticipationSource != participation.SourceAutomationJob {
			t.Fatalf(
				"Start().NetworkParticipationSource = %q, want %q",
				service.inputs.NetworkParticipationSource,
				participation.SourceAutomationJob,
			)
		}
		assertAutomationLoopNamedParticipation(t, service.inputs.NetworkParticipation, "deployments")
	})
}

type recordingAutomationLoopService struct {
	inputs looppkg.Inputs
}

func (s *recordingAutomationLoopService) Start(
	_ context.Context,
	_ looppkg.WorkspaceID,
	_ string,
	inputs looppkg.Inputs,
	_ taskpkg.ActorContext,
) (*looppkg.Run, error) {
	s.inputs = inputs
	return &looppkg.Run{ID: "looprun-automation"}, nil
}

func automationLoopNamedParticipation(channelID string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
	}
}

func assertAutomationLoopNamedParticipation(
	t *testing.T,
	request *participation.Request,
	channelID string,
) {
	t.Helper()
	if request == nil || request.Mode == nil || *request.Mode != participation.ModeLive {
		t.Fatalf("network participation = %#v, want live", request)
	}
	if request.ChannelStrategy == nil || *request.ChannelStrategy != participation.StrategyNamed {
		t.Fatalf("network participation strategy = %#v, want named", request.ChannelStrategy)
	}
	if request.ChannelID == nil || *request.ChannelID != channelID {
		t.Fatalf("network participation channel = %#v, want %q", request.ChannelID, channelID)
	}
}

func TestAutomationLoopStarterDefaultCatchUpPolicyShouldCoalesceWatchLoops(t *testing.T) {
	t.Parallel()

	t.Run("Should coalesce watch-sourced scheduled Loops", func(t *testing.T) {
		t.Parallel()

		starter := &automationLoopStarter{
			resolver: looppkg.DefinitionResolverFunc(
				func(context.Context, looppkg.WorkspaceID, string) (*looppkg.ResolvedDefinition, error) {
					return &looppkg.ResolvedDefinition{
						Definition: loopdsl.Definition{
							Graph: loopdsl.Graph{Nodes: []loopdsl.Node{{
								ID:    "watch",
								Class: loopdsl.NodeClassSource,
								Kind:  string(loopdsl.SourceWatchSource),
							}}},
						},
					}, nil
				},
			),
		}

		policy, err := starter.DefaultLoopCatchUpPolicy(context.Background(), automationpkg.LoopCatchUpPolicyRequest{
			WorkspaceID: "ws-1",
			LoopName:    "watch-loop",
			Kind:        automationpkg.LoopStartKindSchedule,
		})
		if err != nil {
			t.Fatalf("DefaultLoopCatchUpPolicy() error = %v", err)
		}
		if policy != automationpkg.SchedulerCatchUpPolicyCoalesce {
			t.Fatalf("DefaultLoopCatchUpPolicy() = %q, want coalesce", policy)
		}
	})
}
