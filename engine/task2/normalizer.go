// Package task2 provides a modular, task-type-specific normalization architecture
// that replaces the monolithic normalizer package.
package task2

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
)

// TaskNormalizer defines the contract for task-specific normalization
type TaskNormalizer interface {
	// Normalize applies task-specific normalization rules
	Normalize(config *task.Config, ctx *shared.NormalizationContext) error

	// Type returns the task type this normalizer handles
	Type() task.Type
}

// NormalizerFactory creates appropriate normalizers
type NormalizerFactory interface {
	CreateNormalizer(taskType task.Type) (TaskNormalizer, error)

	// Component normalizers
	CreateAgentNormalizer() *core.AgentNormalizer
	CreateToolNormalizer() *core.ToolNormalizer
	CreateSuccessTransitionNormalizer() *core.SuccessTransitionNormalizer
	CreateErrorTransitionNormalizer() *core.ErrorTransitionNormalizer
	CreateOutputTransformer() *core.OutputTransformer
}
