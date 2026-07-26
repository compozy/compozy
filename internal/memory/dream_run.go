package memory

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

// Run acquires the consolidation lock when needed and invokes the spawner with
// the embedded prompt and a normalized workspace ID when provided.
func (s *Service) Run(ctx context.Context, spawn SessionSpawner, workspaceRef string) error {
	if err := s.validate(); err != nil {
		return err
	}
	if ctx == nil {
		return errors.New("memory: context is required")
	}
	if spawn == nil {
		return errors.New("memory: session spawner is required")
	}

	s.runMu.Lock()
	defer s.runMu.Unlock()

	priorMtime, err := s.ensureLock()
	if err != nil {
		return err
	}

	workspace, err := s.prepareWorkspace(ctx, workspaceRef)
	if err != nil {
		return s.failBeforeDreamStart("prepare workspace", workspaceRef, priorMtime, err)
	}
	gate, err := s.evaluateDreamSignalGate(ctx, workspace)
	if err != nil {
		return s.failBeforeDreamStart("evaluate dream signal gate", workspace.id, priorMtime, err)
	}
	if err := s.handleDreamGateResult(ctx, workspace, gate, priorMtime); err != nil {
		return err
	}

	s.logger.Debug("memory: starting consolidation run", "goal", s.goal, "workspace_id", workspace.id)

	if err := spawn(ctx, s.goal, s.prompt, workspace.id, priorMtime); errors.Is(err, ErrDreamRoleDisabled) {
		return s.skipDreamRun(ctx, workspace, gate, priorMtime)
	} else if err != nil {
		return s.failDreamRun(ctx, workspace, gate, priorMtime, err, "spawn consolidation session")
	}
	if !gate.active {
		s.logger.Debug(
			"memory: consolidation run completed; releasing lock",
			"goal",
			s.goal,
			"workspace_id",
			workspace.id,
		)
		return s.completeRun(true, priorMtime)
	}

	if err := s.promoteDreamRun(ctx, workspace, gate, priorMtime); err != nil {
		return err
	}
	s.logger.Debug("memory: consolidation run completed; releasing lock", "goal", s.goal, "workspace_id", workspace.id)
	return s.completeRun(true, priorMtime)
}

func (s *Service) skipDreamRun(
	ctx context.Context,
	workspace dreamRunWorkspace,
	run dreamSignalGateResult,
	priorMtime time.Time,
) error {
	s.logger.Debug(
		"memory: dream consolidation skipped because the workspace role is disabled",
		"workspace_id", workspace.id,
	)
	var cleanupErrs []error
	if run.active && workspace.store != nil {
		if err := workspace.store.deleteDreamRun(ctx, run.runID); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	if err := s.completeRun(false, priorMtime); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
		return fmt.Errorf("memory: clean up disabled dream run: %w", cleanupErr)
	}
	return ErrDreamRoleDisabled
}

func (s *Service) handleDreamGateResult(
	ctx context.Context,
	workspace dreamRunWorkspace,
	gate dreamSignalGateResult,
	priorMtime time.Time,
) error {
	if gate.active && len(gate.candidates) < s.dreamGate.MinCandidates {
		s.logger.Debug(
			"memory: dream signal gate blocked consolidation",
			"workspace_id",
			workspace.id,
			"candidate_count",
			len(gate.candidates),
			"min_candidates",
			s.dreamGate.MinCandidates,
			"reason",
			gate.reason,
		)
		return errors.Join(ErrDreamGateNotSatisfied, s.completeRun(true, priorMtime))
	}
	if !gate.active {
		return nil
	}
	if err := workspace.store.startDreamRun(ctx, gate, workspace, s.now().UTC()); err != nil {
		s.logger.Debug("memory: dream run start failed; rolling back lock", "workspace_id", workspace.id, "error", err)
		return errors.Join(fmt.Errorf("memory: start dream run: %w", err), s.completeRun(false, priorMtime))
	}
	return nil
}

func (s *Service) promoteDreamRun(
	ctx context.Context,
	workspace dreamRunWorkspace,
	gate dreamSignalGateResult,
	priorMtime time.Time,
) error {
	artifactPath, err := workspace.store.writeDreamArtifact(ctx, workspace, gate, s.now().UTC())
	if err != nil {
		return s.failDreamRun(ctx, workspace, gate, priorMtime, err, "write dream artifact")
	}
	decision, err := workspace.store.ProposeCandidate(
		ctx,
		dreamPromotionCandidate(gate, workspace, artifactPath, s.now().UTC()),
	)
	if err != nil {
		return s.failDreamRun(ctx, workspace, gate, priorMtime, err, "propose dream promotion")
	}
	promoted := 0
	if decision.Op == memcontract.OpAdd || decision.Op == memcontract.OpUpdate {
		promoted, err = workspace.store.markDreamPromoted(ctx, gate.candidates, gate.runID, s.now().UTC())
		if err != nil {
			return s.failDreamRun(ctx, workspace, gate, priorMtime, err, "mark dream promoted")
		}
	}
	if err := workspace.store.completeDreamRun(ctx, gate, workspace, promoted, s.now().UTC()); err != nil {
		s.logger.Debug(
			"memory: dream run completion failed; rolling back lock",
			"workspace_id",
			workspace.id,
			"error",
			err,
		)
		rollbackErr := s.completeRun(false, priorMtime)
		return errors.Join(fmt.Errorf("memory: complete dream run: %w", err), rollbackErr)
	}
	return nil
}

func (s *Service) failBeforeDreamStart(operation string, target string, priorMtime time.Time, cause error) error {
	s.logger.Debug(
		"memory: consolidation run failed before spawn; rolling back lock",
		"operation",
		operation,
		"target",
		strings.TrimSpace(target),
		"error",
		cause,
	)
	return errors.Join(
		fmt.Errorf("memory: %s %q: %w", operation, strings.TrimSpace(target), cause),
		s.completeRun(false, priorMtime),
	)
}

func (s *Service) evaluateDreamSignalGate(
	ctx context.Context,
	workspace dreamRunWorkspace,
) (dreamSignalGateResult, error) {
	run := dreamSignalGateResult{runID: storepkg.NewID("dream")}
	if workspace.store == nil || workspace.store.catalog == nil {
		run.reason = "catalog disabled"
		return run, nil
	}
	run.active = true
	candidates, err := workspace.store.dreamCandidates(ctx, workspace.id, s.dreamGate, s.now().UTC())
	if err != nil {
		return dreamSignalGateResult{}, err
	}
	run.candidates = candidates
	if len(candidates) < s.dreamGate.MinCandidates {
		run.reason = fmt.Sprintf(
			"eligible_candidates=%d min_candidates=%d",
			len(candidates),
			s.dreamGate.MinCandidates,
		)
	}
	return run, nil
}

func (s *Service) failDreamRun(
	ctx context.Context,
	workspace dreamRunWorkspace,
	run dreamSignalGateResult,
	priorMtime time.Time,
	cause error,
	operation string,
) error {
	s.logger.Debug(
		"memory: consolidation run failed; rolling back lock",
		"workspace_id",
		workspace.id,
		"operation",
		operation,
		"error",
		cause,
	)
	var cleanupErrs []error
	if workspace.store != nil {
		if _, err := workspace.store.writeDreamFailure(ctx, workspace, run, cause, s.now().UTC()); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
		if err := workspace.store.failDreamRun(ctx, run, workspace, cause, s.now().UTC()); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	rollbackErr := s.completeRun(false, priorMtime)
	errs := []error{fmt.Errorf("memory: %s: %w", operation, cause)}
	errs = append(errs, cleanupErrs...)
	errs = append(errs, rollbackErr)
	return errors.Join(errs...)
}
