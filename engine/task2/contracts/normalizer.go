// Package contracts defines the core interfaces for the task2 normalization system.
// These interfaces are designed to have zero dependencies on other task2 packages
// to avoid circular import issues.
package contracts

import (
	"context"

	"github.com/compozy/compozy/engine/task"
)

// TaskNormalizer defines the contract for task-specific normalization.
// Each task type (basic, parallel, composite, etc.) implements this interface
// to provide its own normalization logic.
type TaskNormalizer interface {
	// Normalize applies task-specific normalization rules to the given configuration.
	// The context parameter must implement NormalizationContext interface.
	// In practice, this will be *shared.NormalizationContext.
	Normalize(ctx context.Context, config *task.Config, normCtx NormalizationContext) error
	// Type returns the task type this normalizer handles
	Type() task.Type
}
