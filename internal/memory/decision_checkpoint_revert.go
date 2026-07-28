package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func (s *Store) revertAddDecision(ctx context.Context, decision storedDecision) error {
	if err := s.ensureCurrentHashRequired(decision); err != nil {
		return err
	}
	handled, err := s.revertCheckpointAdd(ctx, decision)
	if err != nil || handled {
		return err
	}
	if err := s.deleteRaw(
		ctx,
		decisionScope(decision.Decision),
		decision.TargetFilename,
		false,
	); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) revertUpdateDecision(ctx context.Context, decision storedDecision) error {
	if strings.TrimSpace(decision.PriorContent) == "" {
		return fmt.Errorf("memory: decision %q has no prior_content", decision.ID)
	}
	if err := s.ensureCurrentHashRequired(decision); err != nil {
		return err
	}
	content, err := s.checkpointUpdateRevertContent(decision)
	if err != nil {
		return err
	}
	return s.writeRaw(
		ctx,
		decisionScope(decision.Decision),
		decision.TargetFilename,
		content,
		false,
	)
}

func (s *Store) revertCheckpointAdd(ctx context.Context, decision storedDecision) (bool, error) {
	if strings.TrimSpace(decision.TargetFilename) != CheckpointSummaryFilename {
		return false, nil
	}
	content, err := s.Read(decisionScope(decision.Decision), decision.TargetFilename)
	if err != nil {
		return false, err
	}
	state, err := parseCheckpointSummaryState(content)
	if err != nil {
		return false, fmt.Errorf("memory: preserve checkpoint coverage while reverting add: %w", err)
	}
	if len(state.coverage.Ranges) == 0 {
		return false, nil
	}
	state.body = ""
	document, err := renderCheckpointSummaryState(state)
	if err != nil {
		return false, err
	}
	if err := s.writeRaw(
		ctx,
		decisionScope(decision.Decision),
		decision.TargetFilename,
		document,
		false,
	); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) checkpointUpdateRevertContent(decision storedDecision) ([]byte, error) {
	prior := []byte(decision.PriorContent)
	if strings.TrimSpace(decision.TargetFilename) != CheckpointSummaryFilename {
		return prior, nil
	}
	content, err := s.Read(decisionScope(decision.Decision), decision.TargetFilename)
	if err != nil {
		return nil, err
	}
	current, err := parseCheckpointSummaryState(content)
	if err != nil {
		return nil, fmt.Errorf("memory: parse current checkpoint during revert: %w", err)
	}
	if len(current.coverage.Ranges) == 0 {
		return prior, nil
	}
	reverted, err := parseCheckpointSummaryState(prior)
	if err != nil {
		return nil, fmt.Errorf("memory: parse prior checkpoint during revert: %w", err)
	}
	reverted.coverage = current.coverage
	return renderCheckpointSummaryState(reverted)
}
