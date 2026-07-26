package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/tools"
)

func (e *Evaluator) evaluateExtension(
	ctx context.Context,
	gate Gate,
	criterion dsl.GateCriterion,
	in GateInput,
) CriterionResult {
	if e.tools == nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerExtensionFailed,
				Note: "tool caller is not configured",
			}},
		}
	}
	input, err := json.Marshal(criterion.Inputs)
	if err != nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeInvalidOutput,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerExtensionInvalid,
				Note: err.Error(),
			}},
		}
	}
	if criterion.Inputs == nil {
		input = []byte("{}")
	}
	req := tools.CallRequest{
		ToolID:               tools.ToolID(extensionToolID(criterion)),
		ToolCallID:           fmt.Sprintf("%s:%s", gate.ID, criterion.ID),
		SessionID:            in.ToolScope.SessionID,
		WorkspaceID:          in.ToolScope.WorkspaceID,
		AgentName:            in.ToolScope.AgentName,
		ActorKind:            in.ToolScope.ActorKind,
		CorrelationID:        in.ToolCallCorrelationID,
		Input:                input,
		SensitiveInputFields: append([]string(nil), in.ToolSensitiveInputFields...),
	}
	result, err := e.tools.Call(ctx, in.ToolScope, req)
	if err != nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerExtensionFailed,
				Note: err.Error(),
			}},
		}
	}
	return parseExtensionResult(criterion, result)
}

func parseExtensionResult(criterion dsl.GateCriterion, result tools.ToolResult) CriterionResult {
	criterionResult := CriterionResult{
		ID:      criterion.ID,
		Type:    criterion.Type,
		Payload: cloneRawMessage(result.Structured),
	}
	if len(bytes.TrimSpace(result.Structured)) == 0 {
		criterionResult.Outcome = VerdictOutcomeInvalidOutput
		criterionResult.BlockingIssues = []BlockingIssue{{
			ID:   blockerExtensionInvalid,
			Note: "extension returned no structured output",
		}}
		return criterionResult
	}
	parsed, warnings, err := parseStructuredVerdict(result.Structured, false, false)
	if err == nil {
		criterionResult.Outcome = parsed.Outcome
		criterionResult.Passed = parsed.Outcome == VerdictOutcomeApproved
		criterionResult.Confidence = parsed.Confidence
		criterionResult.Evidence = parsed.Evidence
		criterionResult.BlockingIssues = parsed.BlockingIssues
		criterionResult.Warnings = warnings
		if parsed.Outcome != VerdictOutcomeApproved && len(criterionResult.BlockingIssues) == 0 {
			criterionResult.BlockingIssues = []BlockingIssue{{
				ID:   blockerExtensionFailed,
				Note: "extension gate check did not pass",
			}}
		}
		return criterionResult
	}
	exitResult, exitErr := parseExtensionExit(result.Structured)
	if exitErr == nil {
		criterionResult.Outcome = exitResult.Outcome
		criterionResult.Passed = exitResult.Outcome == VerdictOutcomeApproved
		criterionResult.ExitCode = exitResult.ExitCode
		criterionResult.Stdout = exitResult.Stdout
		criterionResult.Stderr = exitResult.Stderr
		criterionResult.BlockingIssues = exitResult.BlockingIssues
		return criterionResult
	}
	criterionResult.Outcome = VerdictOutcomeInvalidOutput
	criterionResult.BlockingIssues = []BlockingIssue{{
		ID:   blockerExtensionInvalid,
		Note: err.Error(),
	}}
	criterionResult.Warnings = []DiagnosticWarning{{
		Code:    "extension_output_invalid",
		Message: err.Error(),
	}}
	return criterionResult
}

type extensionExitResult struct {
	Outcome        VerdictOutcome
	ExitCode       *int
	Stdout         string
	Stderr         string
	BlockingIssues []BlockingIssue
}

func parseExtensionExit(raw json.RawMessage) (extensionExitResult, error) {
	var output struct {
		Passed   *bool  `json:"passed"`
		ExitCode *int   `json:"exit_code"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		return extensionExitResult{}, fmt.Errorf("decode extension output: %w", err)
	}
	switch {
	case output.Passed != nil && *output.Passed:
		return extensionExitResult{Outcome: VerdictOutcomeApproved}, nil
	case output.Passed != nil:
		return extensionExitResult{
			Outcome: VerdictOutcomeRejected,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerExtensionFailed,
				Note: "extension reported failed check",
			}},
		}, nil
	case output.ExitCode != nil && *output.ExitCode == 0:
		return extensionExitResult{
			Outcome:  VerdictOutcomeApproved,
			ExitCode: output.ExitCode,
			Stdout:   output.Stdout,
			Stderr:   output.Stderr,
		}, nil
	case output.ExitCode != nil:
		return extensionExitResult{
			Outcome:  VerdictOutcomeRejected,
			ExitCode: output.ExitCode,
			Stdout:   output.Stdout,
			Stderr:   output.Stderr,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerExtensionFailed,
				Note: fmt.Sprintf("extension exited with code %d", *output.ExitCode),
			}},
		}, nil
	default:
		return extensionExitResult{}, errors.New("extension output did not include verdict, passed, or exit_code")
	}
}
