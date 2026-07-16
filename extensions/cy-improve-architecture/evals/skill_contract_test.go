// Suite: workflow draft ADR authorization
// Invariant: A confirmed durable cross-feature outcome can create one workflow draft ADR without allowing direct durable-decision writes.
// Boundary IN: the shipped cy-improve-architecture skill contract.
// Boundary OUT: live agent execution, covered by E2E-035 in SCENARIOS.md.
package evals

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWorkflowDraftADRIsExplicitlyAuthorized(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate evaluation source directory")
	}
	skillPath := filepath.Join(
		filepath.Dir(testFile),
		"..",
		"skills",
		"cy-improve-architecture",
		"SKILL.md",
	)
	skill, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read shipped skill %s: %v", skillPath, err)
	}

	for _, check := range []struct {
		name string
		text string
	}{
		{
			name: "write boundary allows one user-confirmed workflow draft ADR",
			text: "A user-confirmed durable cross-feature outcome may additionally create one workflow-scoped draft ADR under `.compozy/tasks/<workflow>/adrs/`; include the selected workflow and a concise draft summary in the run summary.",
		},
		{
			name: "workflow and draft summary require confirmation before creation",
			text: "For a durable cross-feature outcome, ask the user to confirm the target workflow and concise draft summary. After confirmation, create one workflow-scoped draft ADR under `.compozy/tasks/<workflow>/adrs/`",
		},
		{
			name: "direct durable-decision writes remain forbidden",
			text: "never write `.compozy/DECISIONS.md` or `.compozy/decisions/`.",
		},
	} {
		check := check
		t.Run(check.name, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(string(skill), check.text) {
				t.Fatalf("shipped skill does not preserve contract text: %q", check.text)
			}
		})
	}
}
