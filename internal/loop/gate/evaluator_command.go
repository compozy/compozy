package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

func (e *Evaluator) evaluateCommand(
	ctx context.Context,
	gate Gate,
	criterion dsl.GateCriterion,
) CriterionResult {
	if e.commands == nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerCommandFailed,
				Note: "command runner is not configured",
			}},
		}
	}
	expect := strings.TrimSpace(criterion.Expect)
	if expect == "" {
		expect = commandExpectExitZero
	}
	expect = normalizeCommandExpectation(expect)
	req := CommandRequest{
		GateID:      gate.ID,
		CriterionID: criterion.ID,
		Command:     criterion.Check,
		Expect:      expect,
		Contains:    criterionStringExtra(criterion, "contains"),
	}
	result, err := e.commands.RunCommand(ctx, req)
	if err != nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerCommandFailed,
				Note: err.Error(),
			}},
		}
	}
	exitCode := result.ExitCode
	criterionResult := CriterionResult{
		ID:       criterion.ID,
		Type:     criterion.Type,
		ExitCode: &exitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}
	passed, warning := commandExpectationPassed(req, result)
	if passed {
		criterionResult.Outcome = VerdictOutcomeApproved
		criterionResult.Passed = true
		return criterionResult
	}
	criterionResult.Outcome = VerdictOutcomeRejected
	if warning != nil {
		criterionResult.Warnings = []DiagnosticWarning{*warning}
	}
	criterionResult.BlockingIssues = []BlockingIssue{{
		ID:   blockerCommandExpectation,
		Note: commandFailureNote(req, result),
	}}
	return criterionResult
}

func commandExpectationPassed(req CommandRequest, result CommandResult) (bool, *DiagnosticWarning) {
	switch req.Expect {
	case commandExpectExitZero:
		return result.ExitCode == 0, nil
	case commandExpectExitNonZero:
		return result.ExitCode != 0, nil
	case commandExpectStdoutContains:
		if req.Contains == "" {
			return false, &DiagnosticWarning{
				Code:    "command_contains_missing",
				Message: "stdout_contains expectation requires contains",
			}
		}
		return strings.Contains(result.Stdout, req.Contains), nil
	default:
		return false, &DiagnosticWarning{
			Code:    "command_expect_unknown",
			Message: fmt.Sprintf("command expectation %q is unsupported", req.Expect),
		}
	}
}

func normalizeCommandExpectation(expect string) string {
	switch strings.TrimSpace(expect) {
	case "exit 0":
		return commandExpectExitZero
	case "exit != 0":
		return commandExpectExitNonZero
	default:
		return expect
	}
}

func commandFailureNote(req CommandRequest, result CommandResult) string {
	switch req.Expect {
	case commandExpectExitZero:
		return fmt.Sprintf("command exited with code %d", result.ExitCode)
	case commandExpectExitNonZero:
		return "command exited successfully"
	case commandExpectStdoutContains:
		return fmt.Sprintf("stdout did not contain %q", req.Contains)
	default:
		return fmt.Sprintf("unsupported command expectation %q", req.Expect)
	}
}
