package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (c *Controller) ambiguousDecision(
	ctx context.Context,
	candidate memcontract.Candidate,
	targets []Target,
	trace []memcontract.RuleHit,
) (memcontract.Decision, error) {
	ambiguousTrace := slices.Clone(trace)
	ambiguousTrace = append(
		ambiguousTrace,
		failedRule("ambiguous_targets", "multiple plausible targets require tiebreaker", targetIDs(targets)),
	)
	if c.tiebreaker == nil {
		return c.rulesOnlyAmbiguousDecision(candidate, targets, ambiguousTrace)
	}
	result, err := c.tiebreaker.BreakTie(ctx, TiebreakerRequest{
		Candidate: candidate,
		Targets:   append([]Target(nil), targets...),
		RuleTrace: append([]memcontract.RuleHit(nil), ambiguousTrace...),
	})
	if errors.Is(err, ErrTiebreakerDisabled) {
		return c.rulesOnlyAmbiguousDecision(candidate, targets, ambiguousTrace)
	}
	if err != nil {
		return c.tiebreakerFailureDecision(candidate, targets, ambiguousTrace, result, err)
	}
	decision, applyErr := c.applyTiebreakerResult(candidate, targets, ambiguousTrace, result)
	if applyErr != nil {
		return c.tiebreakerFailureDecision(candidate, targets, ambiguousTrace, result, applyErr)
	}
	return decision, nil
}

func (c *Controller) applyTiebreakerResult(
	candidate memcontract.Candidate,
	targets []Target,
	trace []memcontract.RuleHit,
	result TiebreakerResult,
) (memcontract.Decision, error) {
	op := result.Op.Normalize()
	if err := op.Validate(); err != nil {
		return memcontract.Decision{}, fmt.Errorf("memory controller: validate tiebreaker op: %w", err)
	}
	target := targetByID(targets, result.TargetID)
	var (
		decision memcontract.Decision
		err      error
	)
	switch op {
	case memcontract.OpAdd:
		decision, err = c.addDecision(candidate, trace)
	case memcontract.OpUpdate:
		if target == nil {
			return memcontract.Decision{}, errors.New(
				"memory controller: update tiebreaker result requires a valid target_id",
			)
		}
		decision, err = c.updateDecision(candidate, *target, trace)
	case memcontract.OpDelete:
		if target == nil {
			return memcontract.Decision{}, errors.New(
				"memory controller: delete tiebreaker result requires a valid target_id",
			)
		}
		decision, err = c.decision(
			candidate,
			memcontract.OpDelete,
			[]Target{*target},
			target.TargetFilename,
			"",
			trace,
			result.Reason,
			target,
		)
	case memcontract.OpNoop:
		selected := targets
		if target != nil {
			selected = []Target{*target}
		}
		decision, err = c.decision(
			candidate,
			memcontract.OpNoop,
			selected,
			targetFilename(candidate),
			"",
			trace,
			result.Reason,
			target,
		)
	case memcontract.OpReject:
		decision, err = c.decision(candidate, memcontract.OpReject, nil, "", "", trace, result.Reason, nil)
	default:
		return memcontract.Decision{}, fmt.Errorf("memory controller: unsupported tiebreaker op %q", op.String())
	}
	if err != nil {
		return memcontract.Decision{}, err
	}
	return applyTiebreakerMetadata(decision, result), nil
}

func (c *Controller) tiebreakerFailureDecision(
	candidate memcontract.Candidate,
	targets []Target,
	trace []memcontract.RuleHit,
	result TiebreakerResult,
	cause error,
) (memcontract.Decision, error) {
	call := result.Call
	if call == nil {
		call = &memcontract.LLMCall{PromptVersion: c.promptVersion}
	} else {
		cloned := *call
		call = &cloned
	}
	call.Error = boundString(cause.Error(), maxDecisionReasonBytes)
	result.Call = call
	result.Op = c.defaultOpOnFail
	result.Reason = "tiebreaker failed; configured deterministic fallback selected " + c.defaultOpOnFail.String()
	result.Confidence = confidenceForOp(c.defaultOpOnFail)
	return c.applyTiebreakerResult(candidate, targets, trace, result)
}

func applyTiebreakerMetadata(decision memcontract.Decision, result TiebreakerResult) memcontract.Decision {
	decision.Source = memcontract.SourceLLM
	decision.Confidence = result.Confidence
	decision.Reason = boundString(result.Reason, maxDecisionReasonBytes)
	if result.Call != nil {
		call := *result.Call
		decision.LLMTrace = &call
		if strings.TrimSpace(call.PromptVersion) != "" {
			decision.PromptVersion = strings.TrimSpace(call.PromptVersion)
		}
	}
	decision.IdempotencyKey = IdempotencyKey(decision)
	decision.ID = "dec_" + hashString(decision.IdempotencyKey)[:24]
	return decision
}

func targetByID(targets []Target, targetID string) *Target {
	trimmed := strings.TrimSpace(targetID)
	if trimmed == "" {
		return nil
	}
	for i := range targets {
		if strings.TrimSpace(targets[i].ID) == trimmed {
			return &targets[i]
		}
	}
	return nil
}
