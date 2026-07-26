package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

const (
	commandExpectExitZero       = "exit_zero"
	commandExpectExitNonZero    = "exit_nonzero"
	commandExpectStdoutContains = "stdout_contains"

	blockerCommandFailed         = "command_failed"
	blockerCommandExpectation    = "command_expectation_failed"
	blockerHumanRequestedChanges = "human_requested_changes"
	blockerHumanRejected         = "human_rejected"
	blockerHumanPending          = "human_decision_pending"
	blockerExtensionFailed       = "extension_failed"
	blockerExtensionInvalid      = "extension_invalid_output"
)

// Evaluate runs every criterion in one gate bundle and returns one aggregate verdict.
func (e *Evaluator) Evaluate(ctx context.Context, gate Gate, in GateInput) (Verdict, error) {
	if err := e.validateGate(gate, in); err != nil {
		return Verdict{}, err
	}
	results := make([]CriterionResult, 0, len(gate.Criteria))
	for _, criterion := range gate.Criteria {
		result := e.evaluateCriterion(ctx, gate, criterion, in)
		results = append(results, result)
	}
	if err := reportAggregateJudgeUsage(in.JudgeUsageReporter, results); err != nil {
		return Verdict{}, err
	}
	verdict := e.aggregate(gate, in, results)
	verdict.Route = e.route(gate, verdict, in)
	return verdict, nil
}

func reportAggregateJudgeUsage(reporter JudgeUsageReporter, results []CriterionResult) error {
	var total int64
	reported := false
	for _, result := range results {
		if !result.judgeTokensReported {
			continue
		}
		reported = true
		if result.judgeTokensUsed > 0 && total > math.MaxInt64-result.judgeTokensUsed {
			return wrapInvalidGate("aggregate judge token usage overflows int64")
		}
		total += result.judgeTokensUsed
	}
	if reported && reporter != nil {
		reporter.ReportJudgeTokensUsed(total)
	}
	return nil
}

func (e *Evaluator) validateGate(gate Gate, in GateInput) error {
	if strings.TrimSpace(gate.ID) == "" {
		return wrapInvalidGate("gate id is required")
	}
	if len(gate.Criteria) == 0 {
		return wrapInvalidGate("gate %q must declare at least one criterion", gate.ID)
	}
	if gate.MaxRevisions > e.maxRevisionsCeiling {
		return fmt.Errorf("%w: gate %q max_revisions=%d ceiling=%d",
			ErrMaxRevisionsCeiling,
			gate.ID,
			gate.MaxRevisions,
			e.maxRevisionsCeiling,
		)
	}
	if in.Revision < 0 {
		return wrapInvalidGate("gate %q revision must be non-negative", gate.ID)
	}
	ids := make(map[string]struct{}, len(gate.Criteria))
	hasVerdictSource := false
	for index, criterion := range gate.Criteria {
		id := strings.TrimSpace(criterion.ID)
		if id == "" {
			return wrapInvalidGate("gate %q criterion %d id is required", gate.ID, index)
		}
		if _, exists := ids[id]; exists {
			return wrapInvalidGate("gate %q criterion id %q must be unique", gate.ID, id)
		}
		ids[id] = struct{}{}
		switch criterion.Type {
		case dsl.CriterionCommand:
			if strings.TrimSpace(criterion.Check) == "" {
				return wrapInvalidGate("gate %q command criterion %q check is required", gate.ID, id)
			}
		case dsl.CriterionAgentJudge:
			hasVerdictSource = true
			if strings.TrimSpace(criterion.Rubric) == "" && strings.TrimSpace(criterion.Prompt) == "" {
				return wrapInvalidGate("gate %q agent-judge criterion %q rubric is required", gate.ID, id)
			}
		case dsl.CriterionHuman:
			hasVerdictSource = true
		case dsl.CriterionExtension:
			if extensionToolID(criterion) == "" {
				return wrapInvalidGate("gate %q extension criterion %q tool is required", gate.ID, id)
			}
		default:
			return wrapInvalidGate("gate %q criterion %q type %q is unsupported", gate.ID, id, criterion.Type)
		}
	}
	policy := gate.VerdictPolicy
	if policy == "" {
		policy = dsl.VerdictPolicyFixedPasses
	}
	switch policy {
	case dsl.VerdictPolicyFixedPasses:
	case dsl.VerdictPolicyReviseUntilClean:
		if !hasVerdictSource {
			return fmt.Errorf("%w: gate %q", ErrVerdictPolicyRequiresJudge, gate.ID)
		}
	default:
		return wrapInvalidGate("gate %q verdict_policy %q is unsupported", gate.ID, policy)
	}
	return nil
}

func (e *Evaluator) evaluateCriterion(
	ctx context.Context,
	gate Gate,
	criterion dsl.GateCriterion,
	in GateInput,
) CriterionResult {
	switch criterion.Type {
	case dsl.CriterionCommand:
		return e.evaluateCommand(ctx, gate, criterion)
	case dsl.CriterionAgentJudge:
		return e.evaluateAgentJudge(ctx, gate, criterion, in)
	case dsl.CriterionHuman:
		return e.evaluateHuman(gate, criterion, in)
	case dsl.CriterionExtension:
		return e.evaluateExtension(ctx, gate, criterion, in)
	default:
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeInvalidOutput,
			Warnings: []DiagnosticWarning{{
				Code:    "criterion_type_unsupported",
				Message: fmt.Sprintf("criterion type %q is unsupported", criterion.Type),
			}},
		}
	}
}

func (e *Evaluator) aggregate(gate Gate, in GateInput, results []CriterionResult) Verdict {
	verdict := Verdict{
		Criteria: results,
	}
	seenIssues := make(map[string]BlockingIssue)
	seenWarnings := make(map[string]DiagnosticWarning)
	nonBrokenFailure := false
	broken := false
	for _, result := range results {
		if result.Broken {
			broken = true
		}
		if result.Outcome != VerdictOutcomeApproved && !result.Broken {
			nonBrokenFailure = true
		}
		for _, issue := range result.BlockingIssues {
			normalized := normalizeIssue(issue, result)
			if _, exists := seenIssues[normalized.ID]; !exists {
				seenIssues[normalized.ID] = normalized
			}
		}
		for _, warning := range result.Warnings {
			if strings.TrimSpace(warning.Code) == "" {
				continue
			}
			seenWarnings[warning.Code] = warning
		}
	}
	verdict.Broken = broken && !nonBrokenFailure
	verdict.Outcome = aggregateOutcome(results, verdict.Broken)
	verdict.BlockingIssues = sortedIssues(seenIssues)
	verdict.BlockingIssueSignature = sortedIssueIDs(verdict.BlockingIssues)
	verdict.Warnings = sortedWarnings(seenWarnings)
	verdict.Payload = aggregatePayload(results)
	if verdict.Broken {
		verdict.NextBrokenJudgeStreak = in.BrokenJudgeStreak + 1
	} else {
		verdict.NextBrokenJudgeStreak = 0
	}
	if len(verdict.BlockingIssues) == 0 && verdict.Outcome != VerdictOutcomeApproved {
		verdict.BlockingIssues = []BlockingIssue{{
			ID:   defaultAggregateIssueID(verdict),
			Note: fmt.Sprintf("gate %q produced %s", gate.ID, verdict.Outcome),
		}}
		verdict.BlockingIssueSignature = sortedIssueIDs(verdict.BlockingIssues)
	}
	return verdict
}

func aggregateOutcome(results []CriterionResult, brokenFailOpen bool) VerdictOutcome {
	if brokenFailOpen {
		return VerdictOutcomeError
	}
	order := []VerdictOutcome{
		VerdictOutcomeAwaitingApproval,
		VerdictOutcomeBlocked,
		VerdictOutcomeTimeout,
		VerdictOutcomeError,
		VerdictOutcomeInvalidOutput,
		VerdictOutcomeRejected,
	}
	for _, outcome := range order {
		for _, result := range results {
			if result.Broken {
				continue
			}
			if result.Outcome == outcome {
				return outcome
			}
		}
	}
	return VerdictOutcomeApproved
}

func aggregatePayload(results []CriterionResult) json.RawMessage {
	for _, result := range results {
		if len(bytes.TrimSpace(result.Evidence)) > 0 {
			return cloneRawMessage(result.Evidence)
		}
	}
	for _, result := range results {
		if len(bytes.TrimSpace(result.Payload)) > 0 {
			return cloneRawMessage(result.Payload)
		}
	}
	return nil
}

func normalizeIssue(issue BlockingIssue, result CriterionResult) BlockingIssue {
	id := strings.TrimSpace(issue.ID)
	if id == "" {
		id = fmt.Sprintf("%s_%s", result.ID, result.Outcome)
	}
	note := strings.TrimSpace(issue.Note)
	if note == "" {
		note = fmt.Sprintf("criterion %q produced %s", result.ID, result.Outcome)
	}
	return BlockingIssue{ID: id, Note: note}
}

func sortedIssues(issues map[string]BlockingIssue) []BlockingIssue {
	if len(issues) == 0 {
		return nil
	}
	ids := make([]string, 0, len(issues))
	for id := range issues {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sorted := make([]BlockingIssue, 0, len(ids))
	for _, id := range ids {
		sorted = append(sorted, issues[id])
	}
	return sorted
}

func sortedIssueIDs(issues []BlockingIssue) []string {
	if len(issues) == 0 {
		return nil
	}
	ids := make([]string, 0, len(issues))
	for _, issue := range issues {
		if strings.TrimSpace(issue.ID) != "" {
			ids = append(ids, issue.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func sortedWarnings(warnings map[string]DiagnosticWarning) []DiagnosticWarning {
	if len(warnings) == 0 {
		return nil
	}
	codes := make([]string, 0, len(warnings))
	for code := range warnings {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	sorted := make([]DiagnosticWarning, 0, len(codes))
	for _, code := range codes {
		sorted = append(sorted, warnings[code])
	}
	return sorted
}

func defaultAggregateIssueID(verdict Verdict) string {
	if verdict.Broken {
		return JudgeTransportFailureIssueID
	}
	switch verdict.Outcome {
	case VerdictOutcomeRejected:
		return "gate_rejected"
	case VerdictOutcomeBlocked:
		return "gate_blocked"
	case VerdictOutcomeTimeout:
		return "gate_timeout"
	case VerdictOutcomeInvalidOutput:
		return "gate_invalid_output"
	default:
		return "gate_error"
	}
}

func extensionToolID(criterion dsl.GateCriterion) string {
	if strings.TrimSpace(criterion.Tool) != "" {
		return strings.TrimSpace(criterion.Tool)
	}
	return strings.TrimSpace(criterion.Check)
}

func criterionStringExtra(criterion dsl.GateCriterion, key string) string {
	if criterion.Extra == nil {
		return ""
	}
	value, ok := criterion.Extra[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make([]byte, len(raw))
	copy(cloned, raw)
	return cloned
}
