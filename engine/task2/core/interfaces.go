package core

// Package core provides foundational components for task normalization.
// It defines minimal interfaces to avoid circular dependencies with parent task2 package.
// This file is intentionally left minimal to maintain clean package boundaries.

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// TaskNormalizer defines the minimal interface for task normalization needed by core
type TaskNormalizer interface {
	// Normalize applies task-specific normalization rules
	Normalize(config *task.Config, ctx *shared.NormalizationContext) error
}

// NormalizerFactory defines the minimal factory interface needed by core
type NormalizerFactory interface {
	// CreateNormalizer creates a normalizer for the given task type
	CreateNormalizer(taskType task.Type) (TaskNormalizer, error)
}
