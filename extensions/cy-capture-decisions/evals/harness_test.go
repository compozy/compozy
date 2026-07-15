package evals

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/extensions/cy-capture-decisions/decisionlog"
)

func TestAllCasesDefinesCompleteAssignedMatrix(t *testing.T) {
	t.Run("Should define the complete assigned matrix", func(t *testing.T) {
		t.Parallel()

		want := []string{
			"E2E-001", "E2E-002", "E2E-003", "E2E-004", "E2E-005", "E2E-006", "E2E-007",
			"E2E-008", "E2E-009", "E2E-010", "E2E-011", "E2E-012", "E2E-013", "E2E-014",
			"E2E-015", "E2E-016", "E2E-017", "E2E-018", "E2E-019", "E2E-020", "E2E-021",
			"IT-001", "IT-005", "IT-006",
		}
		cases := allCases()
		got := make([]string, 0, len(cases))
		seen := make(map[string]struct{}, len(cases))
		for _, eval := range cases {
			if eval.Run == nil {
				t.Fatalf("%s has no executable assertion", eval.ID)
			}
			if _, duplicate := seen[eval.ID]; duplicate {
				t.Fatalf("duplicate case %s", eval.ID)
			}
			seen[eval.ID] = struct{}{}
			got = append(got, eval.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case IDs = %v, want %v", got, want)
		}
	})
}

func TestSelectCasesNormalizesIDsAndPreservesMatrixOrder(t *testing.T) {
	t.Run("Should normalize IDs while preserving matrix order", func(t *testing.T) {
		t.Parallel()

		selected, err := selectCases(allCases(), []string{"it-006", " e2e-001 "})
		if err != nil {
			t.Fatal(err)
		}
		got := []string{selected[0].ID, selected[1].ID}
		want := []string{"E2E-001", "IT-006"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("selected IDs = %v, want %v", got, want)
		}
	})
}

func TestDefaultConfigRequiresOptInModelAndThreeRuns(t *testing.T) {
	t.Run("Should require an opt-in model and default to three Codex runs", func(t *testing.T) {
		t.Parallel()

		config := DefaultConfig(t.TempDir())
		if config.Model != "" {
			t.Fatalf("default model = %q, want explicit opt-in", config.Model)
		}
		if config.Repetitions != 3 {
			t.Fatalf("repetitions = %d, want 3", config.Repetitions)
		}
		if config.IDE != "codex" || config.ReasoningEffort != "medium" {
			t.Fatalf("runtime = %s/%s, want codex/medium", config.IDE, config.ReasoningEffort)
		}
		if config.Timeout != 20*time.Minute {
			t.Fatalf("timeout = %s, want 20m", config.Timeout)
		}
	})
}

func TestFindByProvenance(t *testing.T) {
	t.Run("Should return the unique provenance match", func(t *testing.T) {
		t.Parallel()

		snapshot := logSnapshot{Records: map[string]decisionlog.DecisionRecordMeta{
			"AD-001": {ID: "AD-001", SourceSlug: "feat-orders", SourceADR: "adrs/adr-001.md"},
		}}
		record, err := findByProvenance(snapshot, "feat-orders", "adrs/adr-001.md")
		if err != nil {
			t.Fatal(err)
		}
		if record.ID != "AD-001" {
			t.Fatalf("record ID = %q, want AD-001", record.ID)
		}
	})

	t.Run("Should reject duplicate provenance matches", func(t *testing.T) {
		t.Parallel()

		snapshot := logSnapshot{Records: map[string]decisionlog.DecisionRecordMeta{
			"AD-001": {ID: "AD-001", SourceSlug: "feat-orders", SourceADR: "adrs/adr-001.md"},
			"AD-002": {ID: "AD-002", SourceSlug: "feat-orders", SourceADR: "adrs/adr-001.md"},
		}}
		_, err := findByProvenance(snapshot, "feat-orders", "adrs/adr-001.md")
		if err == nil || !strings.Contains(err.Error(), "AD-001, AD-002") {
			t.Fatalf("duplicate provenance error = %v", err)
		}
	})
}

func TestRunTrial(t *testing.T) {
	t.Run("Should report unsupported environments as skipped", func(t *testing.T) {
		t.Parallel()

		resultsDir := t.TempDir()
		h := &harness{config: Config{ResultsDir: resultsDir}}
		result := h.runTrial(context.Background(), evalCase{
			ID: "IT-005",
			Run: func(context.Context, *trial) error {
				return skipEval("permission model is unsupported")
			},
		}, 1)
		if !result.Skipped || result.Passed || result.Error != "" {
			t.Fatalf("result = %+v, want a clean skip", result)
		}
		if result.SkipReason != "permission model is unsupported" {
			t.Fatalf("skip reason = %q", result.SkipReason)
		}
	})
}

func TestRunCases(t *testing.T) {
	t.Run("Should stop scheduling trials after cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		called := false
		h := &harness{config: Config{Repetitions: 3}}
		results, err := h.runCases(ctx, []evalCase{{
			ID: "E2E-001",
			Run: func(context.Context, *trial) error {
				called = true
				return nil
			},
		}})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runCases error = %v, want context cancellation", err)
		}
		if called || len(results) != 0 {
			t.Fatalf("canceled run executed trials: called=%t results=%d", called, len(results))
		}
	})
}
