package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
)

func TestNewAutomationLoopStarter(t *testing.T) {
	t.Parallel()

	t.Run("Should reject non Loop stores when Loop catalog is configured", func(t *testing.T) {
		t.Parallel()

		starter, err := newAutomationLoopStarter(
			struct{}{},
			newResourceCatalog(looppkg.CloneResourceSpec),
			nil,
			compozyconfig.HomePaths{},
			nil,
			nil,
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

	t.Run("Should canonicalize admission identity before persistence", func(t *testing.T) {
		t.Parallel()

		admission := automationLoopAdmission(automationpkg.LoopStartRequest{
			SourceKey: "  trigger:deploy  ",
			EventKey:  "  delivery:42  ",
		})
		if admission == nil || admission.SourceKey != "trigger:deploy" || admission.EventKey != "delivery:42" {
			t.Fatalf("automationLoopAdmission() = %#v, want trimmed stable identity", admission)
		}
	})
}

func TestAutomationLoopStarterDefaultCatchUpPolicyShouldCoalesceWatchLoops(t *testing.T) {
	t.Parallel()

	t.Run("Should coalesce watch-sourced scheduled Loops", func(t *testing.T) {
		t.Parallel()

		starter := &automationLoopStarter{
			resolver: looppkg.DefinitionResolverFunc(
				func(context.Context, looppkg.WorkspaceID, string, string) (*looppkg.ResolvedDefinition, error) {
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

		policy, err := starter.DefaultLoopCatchUpPolicy(t.Context(), automationpkg.LoopCatchUpPolicyRequest{
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
