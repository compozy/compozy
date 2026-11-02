package daemon

import (
	"context"
	"errors"
	"slices"
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

func TestAutomationLoopStarterRetiredDependencies(t *testing.T) {
	t.Parallel()
	t.Run("Should classify a definition removed between validation and resolution", func(t *testing.T) {
		t.Parallel()
		definition, err := loopdsl.Parse(
			[]byte(strings.ReplaceAll(testLoopYAML("vanishing", "Vanishing loop"), "kind: cli", "kind: schedule")),
		)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := looppkg.NewCompiler().Compile(definition)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		starter := &automationLoopStarter{
			resolver: looppkg.DefinitionResolverFunc(
				func(context.Context, looppkg.WorkspaceID, string, string) (*looppkg.ResolvedDefinition, error) {
					calls++
					if calls == 1 {
						return resolved, nil
					}
					return nil, looppkg.ErrDefinitionNotFound
				},
			),
		}
		err = starter.ValidateLoopTarget(
			t.Context(),
			automationpkg.LoopTargetValidationRequest{
				WorkspaceID: "ws",
				ProfileID:   "default",
				LoopName:    "vanishing",
				Kind:        automationpkg.LoopStartKindSchedule,
				Inputs:      map[string]any{"target": "fixture"},
			},
		)
		if !errors.Is(err, automationpkg.ErrLoopTargetNotFound) {
			t.Fatalf("second resolution = %v", err)
		}
	})
	for _, tc := range []struct {
		name  string
		kind  string
		extra string
		want  bool
	}{
		{"import", "ext__spec_cycle__import_tasks", "", true},
		{"artifacts", "ext__spec_cycle__write_review_artifacts", "", true},
		{"finalize", "ext__spec_cycle__finalize_review_round", "", true},
		{"unknown tool", "ext__other__missing", "", false},
		{"reviewed output", "ext__spec_cycle__import_tasks", "\n      review: {decisions: [respond]}", true},
		{"result contract", "ext__spec_cycle__import_tasks", "\n      result_contract: {failure_field: passed}", true},
		{"malformed contract on retired node", "ext__spec_cycle__import_tasks", "\n      result_contract: {message_field: message}", false},
		{"malformed review on retired node", "ext__spec_cycle__import_tasks", "\n      review: {decisions: [respond, invalid]}", false},
		{"independent missing schema", "ext__spec_cycle__import_tasks", "\n    - id: independent\n      class: action\n      kind: transform\n      result_contract: {failure_field: passed}", false},
	} {
		t.Run("Should classify "+tc.name, func(t *testing.T) {
			t.Parallel()
			definition, err := loopdsl.Parse(
				[]byte(strings.ReplaceAll(
					testLoopYAML("custom", "Custom loop"),
					"kind: transform",
					"kind: "+tc.kind+tc.extra,
				)),
			)
			if err != nil {
				t.Fatal(err)
			}
			_, compileErr := looppkg.NewCompiler().Compile(definition)
			if compileErr == nil {
				t.Fatal("expected unavailable action")
			}
			starter := &automationLoopStarter{
				resolver: looppkg.DefinitionResolverFunc(
					func(context.Context, looppkg.WorkspaceID, string, string) (*looppkg.ResolvedDefinition, error) {
						return nil, compileErr
					},
				),
			}
			err = starter.ValidateLoopTarget(
				t.Context(),
				automationpkg.LoopTargetValidationRequest{
					ProfileID:   "default",
					WorkspaceID: "ws",
					LoopName:    "custom",
					Kind:        automationpkg.LoopStartKindSchedule,
				},
			)
			want := tc.want
			if err == nil || errors.Is(err, automationpkg.ErrLoopTargetUnavailable) != want {
				t.Fatalf("validation error = %v", err)
			}
			if !want {
				requireLintFailure(t, err, "", "")
			}
			_, err = starter.DefaultLoopCatchUpPolicy(
				t.Context(),
				automationpkg.LoopCatchUpPolicyRequest{
					ProfileID:   "default",
					WorkspaceID: "ws",
					LoopName:    "custom",
					Kind:        automationpkg.LoopStartKindSchedule,
				},
			)
			if err == nil || errors.Is(err, automationpkg.ErrLoopTargetUnavailable) != want {
				t.Fatalf("policy error = %v", err)
			}
			if !want {
				requireLintFailure(t, err, "", "")
			}
		})
	}
	for _, code := range []string{looppkg.CodeReviewRespondSchemaRequired, looppkg.CodeResultContractInvalid} {
		t.Run("Should reject independent schema diagnostic "+code, func(t *testing.T) {
			t.Parallel()
			err := &looppkg.LintFailedError{Errors: []looppkg.LintError{
				{NodeID: "retired", Code: looppkg.CodeUnknownActionKind,
					Message: `action ToolID "ext__spec_cycle__import_tasks" is not resolvable`},
				{NodeID: "independent", Code: code, Message: "result_contract requires a declared output schema"},
			}}
			got := classifyRetiredLoopTarget(err)
			if got == nil || errors.Is(got, automationpkg.ErrLoopTargetUnavailable) {
				t.Fatal("independent schema error accepted")
			}
			requireLintFailure(t, got, "independent", code)
		})
	}
	t.Run("Should keep unrelated malformed definitions strict", func(t *testing.T) {
		t.Parallel()
		err := &looppkg.LintFailedError{
			Errors: []looppkg.LintError{
				{
					Code:    looppkg.CodeUnknownActionKind,
					Message: `action ToolID "ext__spec_cycle__import_tasks" is not resolvable`,
				},
				{Code: looppkg.CodeCycle},
			},
		}
		got := classifyRetiredLoopTarget(err)
		if got == nil || errors.Is(got, automationpkg.ErrLoopTargetUnavailable) {
			t.Fatal("malformed definition accepted")
		}
		requireLintFailure(t, got, "", looppkg.CodeCycle)
	})
}

// requireLintFailure proves a negative classification kept the original lint failure.
func requireLintFailure(t *testing.T, err error, nodeID, code string) {
	t.Helper()
	lint, ok := errors.AsType[*looppkg.LintFailedError](err)
	if !ok {
		t.Fatalf("error = %v, want the original lint failure", err)
	}
	if code != "" && !slices.ContainsFunc(lint.Errors, func(entry looppkg.LintError) bool {
		return string(entry.NodeID) == nodeID && entry.Code == code
	}) {
		t.Fatalf("lint errors = %+v, want node %q code %q", lint.Errors, nodeID, code)
	}
}
