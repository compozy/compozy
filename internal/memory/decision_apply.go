package memory

import (
	"context"

	"errors"
	"fmt"
	"os"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// ApplyDecision persists the Decision WAL row before applying the corresponding file mutation.
func (s *Store) ApplyDecision(ctx context.Context, decision memcontract.Decision) (DecisionApplyResult, error) {
	unlock := s.lockControllerDecisions()
	defer unlock()
	return s.applyDecision(ctx, decision)
}

func (s *Store) applyDecision(ctx context.Context, decision memcontract.Decision) (DecisionApplyResult, error) {
	if ctx == nil {
		return DecisionApplyResult{}, errors.New("memory: apply decision context is required")
	}
	normalized, err := normalizeDecision(decision)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return DecisionApplyResult{}, err
	}
	workspaceID, err := s.workspaceIDForDecision(ctx, decisionScope(normalized))
	if err != nil {
		return DecisionApplyResult{}, err
	}
	existing, found, err := s.catalog.loadDecisionByIdempotencyKey(ctx, normalized.IdempotencyKey)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	if found {
		normalized = existing.Decision
		workspaceID = existing.WorkspaceID
		if existing.AppliedAt != nil {
			return DecisionApplyResult{Decision: normalized, Applied: false}, nil
		}
	} else if err := s.catalog.insertDecision(ctx, normalized, workspaceID); err != nil {
		return DecisionApplyResult{}, err
	}

	applied := false
	switch normalized.Op {
	case memcontract.OpAdd, memcontract.OpUpdate:
		if strings.TrimSpace(normalized.PostContent) == "" {
			return DecisionApplyResult{}, fmt.Errorf("memory: decision %q missing post_content", normalized.ID)
		}
		if err := s.writeRaw(
			ctx,
			decisionScope(normalized),
			normalized.TargetFilename,
			[]byte(normalized.PostContent),
			false,
		); err != nil {
			return DecisionApplyResult{}, err
		}
		applied = true
	case memcontract.OpDelete:
		err := s.deleteRaw(ctx, decisionScope(normalized), normalized.TargetFilename, false)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return DecisionApplyResult{}, err
		}
		applied = err == nil
	case memcontract.OpNoop, memcontract.OpReject:
	default:
		return DecisionApplyResult{}, fmt.Errorf("memory: unsupported decision op %q", normalized.Op.String())
	}

	if err := s.catalog.markDecisionApplied(ctx, normalized.ID); err != nil {
		return DecisionApplyResult{}, err
	}
	if err := s.catalog.logDecisionEvent(ctx, normalized, workspaceID, applied); err != nil {
		return DecisionApplyResult{}, err
	}
	return DecisionApplyResult{Decision: normalized, Applied: applied}, nil
}
