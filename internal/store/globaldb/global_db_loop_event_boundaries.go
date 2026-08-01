package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
)

const (
	loopRunEventPayloadKeyBlockingIssues = "blocking_issues"
	loopRunEventPayloadKeyScore          = "score"
	loopRunEventVerdictPass              = "pass"
)

func appendLoopGenerationStartedEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	provenance looppkg.GenerationIntent,
	at time.Time,
) error {
	generation := int(provenance.Generation)
	if generation <= 0 || generation <= run.Generation {
		return nil
	}
	payload := map[string]any{
		loopRunEventPayloadKeyGeneration: generation,
		"parent_generation":              provenance.ParentGeneration,
		"origin":                         string(provenance.Origin),
		"reattempt_strategy":             string(run.ReattemptStrategy),
		columnLoopName:                   run.LoopName,
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		run.ID,
		run.WorkspaceID,
		loopRunEventGenerationStarted,
		payload,
		at,
	)
}

func appendLoopGateVerdictEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	verdict gate.VerdictIntent,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	payload, err := loopGateVerdictEventPayload(generation, verdict, &event)
	if err != nil {
		return err
	}
	return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID, loopRunEventGateVerdict, payload, at)
}

func loopGateVerdictEventPayload(
	generation int,
	verdict gate.VerdictIntent,
	event *looppkg.GenerationLifecycleEventIntent,
) (map[string]any, error) {
	gateID := strings.TrimSpace(verdict.GateID)
	if gateID == "" {
		return nil, fmt.Errorf("%w: gate verdict event gate_id is required", looppkg.ErrValidation)
	}
	var criteriaInput []gate.CriterionResult
	if err := json.Unmarshal(verdict.Criteria, &criteriaInput); err != nil {
		return nil, fmt.Errorf("store: decode sanitized gate verdict criteria: %w", err)
	}
	var blockingInput []gate.BlockingIssue
	if err := json.Unmarshal(verdict.BlockingIssues, &blockingInput); err != nil {
		return nil, fmt.Errorf("store: decode sanitized gate verdict blockers: %w", err)
	}
	verdictLabel := loopRunEventVerdictRevise
	if verdict.Outcome == gate.VerdictOutcomeApproved {
		verdictLabel = loopRunEventVerdictPass
	}
	criteria := make([]map[string]any, 0, len(criteriaInput))
	for _, criterion := range criteriaInput {
		status := loopRunEventVerdictRevise
		if criterion.Passed {
			status = loopRunEventVerdictPass
		}
		criterionPayload := map[string]any{
			"id":                         strings.TrimSpace(criterion.ID),
			loopRunEventPayloadKeyType:   string(criterion.Type),
			loopRunEventPayloadKeyStatus: status,
			"note":                       string(criterion.Outcome),
		}
		if criterion.Score != nil {
			criterionPayload[loopRunEventPayloadKeyScore] = *criterion.Score
		}
		criteria = append(criteria, criterionPayload)
	}
	issues := make([]map[string]any, 0, len(blockingInput))
	for _, issue := range blockingInput {
		issues = append(issues, map[string]any{
			"id":   strings.TrimSpace(issue.ID),
			"note": strings.TrimSpace(issue.Note),
		})
	}
	payload := map[string]any{
		loopRunEventPayloadKeyNodeID:         gateID,
		"gate_id":                            gateID,
		loopRunEventPayloadKeyItemIndex:      verdict.ItemIndex,
		loopRunEventPayloadKeyGeneration:     generation,
		loopRunEventPayloadKeyVerdict:        verdictLabel,
		loopRunEventPayloadKeyReason:         string(verdict.Outcome),
		"criteria":                           criteria,
		loopRunEventPayloadKeyBlockingIssues: issues,
	}
	if event != nil {
		payload["route"] = string(event.Route)
		payload[loopRunEventPayloadKeyReason] = firstNonEmptyString(event.Reason, string(verdict.Outcome))
		if event.BestGeneration != nil {
			payload["best_generation"] = *event.BestGeneration
		}
	}
	if score, ok := loopGateVerdictSummaryScore(criteriaInput); ok {
		payload[loopRunEventPayloadKeyScore] = score
	}
	return payload, nil
}

func loopGateVerdictSummaryScore(criteria []gate.CriterionResult) (float64, bool) {
	var score float64
	found := false
	for _, criterion := range criteria {
		if criterion.Score == nil {
			continue
		}
		if found {
			return 0, false
		}
		score = *criterion.Score
		found = true
	}
	return score, found
}

func appendLoopNeedsApprovalEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	gateID looppkg.NodeID,
	generation int,
	title string,
	facts []map[string]string,
	at time.Time,
) error {
	return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID, loopRunEventNeedsApproval, map[string]any{
		"gate_id":                        strings.TrimSpace(string(gateID)),
		loopRunEventPayloadKeyTitle:      firstNonEmptyString(title, "Approve to resume"),
		loopRunEventPayloadKeyGeneration: generation,
		"facts":                          facts,
	}, at)
}

func loopApprovalFact(label string, value string) map[string]string {
	return map[string]string{
		loopRunApprovalFactLabelKey: label,
		loopRunEventPayloadKeyValue: value,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
