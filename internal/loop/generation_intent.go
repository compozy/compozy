package loop

import (
	"context"
	"fmt"
	"time"
)

// GenerationOrigin identifies why a loop generation was created.
type GenerationOrigin string

const (
	OriginInitial            GenerationOrigin = "initial"
	OriginStopWhen           GenerationOrigin = "stop_when"
	OriginReattempt          GenerationOrigin = "reattempt"
	OriginGateRevise         GenerationOrigin = "gate_revise"
	OriginGateNextGeneration GenerationOrigin = "gate_next_generation"
	OriginDoDRetry           GenerationOrigin = "dod_retry"
	OriginRatchetRestore     GenerationOrigin = "ratchet_restore"
)

// GenerationIntent records immutable provenance for a newly created generation.
type GenerationIntent struct {
	Generation       int64            `json:"generation"`
	ParentGeneration int64            `json:"parent_generation"`
	Origin           GenerationOrigin `json:"origin"`
}

// LoopGeneration is the durable provenance record for one loop generation.
//
//nolint:revive // The accepted TechSpec names this cross-package contract LoopGeneration.
type LoopGeneration struct {
	RunID            string
	Generation       int64
	ParentGeneration int64
	Origin           GenerationOrigin
	CreatedAt        time.Time
}

// GenerationLineageReader provides workspace-scoped immutable generation history.
type GenerationLineageReader interface {
	ListGenerations(ctx context.Context, workspaceID string, runID string) ([]LoopGeneration, error)
}

// Validate rejects generation provenance that cannot satisfy the persistent invariant.
func (i GenerationIntent) Validate() error {
	if i.Generation < 1 {
		return fmt.Errorf("%w: generation intent generation must be positive", ErrValidation)
	}
	if i.ParentGeneration < 0 || i.ParentGeneration >= i.Generation {
		return fmt.Errorf(
			"%w: generation intent parent_generation must be non-negative and less than generation",
			ErrValidation,
		)
	}
	switch i.Origin {
	case OriginInitial,
		OriginStopWhen,
		OriginReattempt,
		OriginGateRevise,
		OriginGateNextGeneration,
		OriginDoDRetry,
		OriginRatchetRestore:
		return nil
	default:
		return fmt.Errorf("%w: generation intent origin is invalid: %q", ErrValidation, i.Origin)
	}
}
