package memory

import (
	"context"

	"errors"
	"fmt"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// RevertDecision restores curated Markdown from one previously applied Decision row.
func (s *Store) RevertDecision(ctx context.Context, id string) (DecisionRevertResult, error) {
	if ctx == nil {
		return DecisionRevertResult{}, errors.New("memory: revert decision context is required")
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return DecisionRevertResult{}, err
	}
	decision, err := s.catalog.loadDecision(ctx, id)
	if err != nil {
		return DecisionRevertResult{}, err
	}
	target, err := s.storeForStoredDecision(ctx, decision)
	if err != nil {
		return DecisionRevertResult{}, err
	}

	reverted := false
	switch decision.Op {
	case memcontract.OpAdd:
		if err := target.revertAddDecision(ctx, decision); err != nil {
			return DecisionRevertResult{}, err
		}
		reverted = true
	case memcontract.OpUpdate:
		if err := target.revertUpdateDecision(ctx, decision); err != nil {
			return DecisionRevertResult{}, err
		}
		reverted = true
	case memcontract.OpDelete:
		if strings.TrimSpace(decision.PriorContent) == "" {
			return DecisionRevertResult{}, fmt.Errorf("memory: decision %q has no prior_content", decision.ID)
		}
		if err := target.ensureTargetAbsentForRevert(decision); err != nil {
			return DecisionRevertResult{}, err
		}
		if err := target.writeRaw(
			ctx,
			decisionScope(decision.Decision),
			decision.TargetFilename,
			[]byte(decision.PriorContent),
			false,
		); err != nil {
			return DecisionRevertResult{}, err
		}
		reverted = true
	case memcontract.OpNoop, memcontract.OpReject:
	default:
		return DecisionRevertResult{}, fmt.Errorf("memory: unsupported decision op %q", decision.Op.String())
	}
	if reverted {
		if err := s.catalog.logRevertEvent(ctx, decision); err != nil {
			return DecisionRevertResult{}, err
		}
	}
	return DecisionRevertResult{
		DecisionID:     decision.ID,
		TargetFilename: decision.TargetFilename,
		Reverted:       reverted,
	}, nil
}
