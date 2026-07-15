package evals

import (
	"reflect"
	"testing"
	"time"
)

func TestAllCasesDefinesCompleteAssignedMatrix(t *testing.T) {
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
}

func TestSelectCasesNormalizesIDsAndPreservesMatrixOrder(t *testing.T) {
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
}

func TestDefaultConfigRequiresOptInModelAndThreeRuns(t *testing.T) {
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
}
