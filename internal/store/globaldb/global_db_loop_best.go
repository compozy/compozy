package globaldb

import (
	"database/sql"
	"fmt"
	"math"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func applyLoopRunBest(run *looppkg.Run, generation sql.NullInt64, score sql.NullFloat64) error {
	bestGeneration, bestScore, err := loopBestValues(generation, score)
	if err != nil {
		return err
	}
	if bestGeneration == nil {
		return nil
	}
	if *bestGeneration > int64(run.Generation) {
		return fmt.Errorf("store: loop run best generation is outside run history: %w", looppkg.ErrValidation)
	}
	run.BestGeneration = bestGeneration
	run.BestScore = bestScore
	return nil
}

func loopBestValues(generation sql.NullInt64, score sql.NullFloat64) (*int64, *float64, error) {
	if generation.Valid != score.Valid {
		return nil, nil, fmt.Errorf(
			"store: loop run best generation and score must be set together: %w",
			looppkg.ErrValidation,
		)
	}
	if !generation.Valid {
		return nil, nil, nil
	}
	if generation.Int64 < 1 {
		return nil, nil, fmt.Errorf("store: loop run best generation must be positive: %w", looppkg.ErrValidation)
	}
	if math.IsNaN(score.Float64) || math.IsInf(score.Float64, 0) {
		return nil, nil, fmt.Errorf("store: loop run best score must be finite: %w", looppkg.ErrValidation)
	}
	return new(generation.Int64), new(score.Float64), nil
}
