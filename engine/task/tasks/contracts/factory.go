package contracts

import (
	"context"

	"github.com/compozy/compozy/engine/task"
)

// NormalizerFactory defines the contract for creating task normalizers.
// This interface is implemented by the main factory in the tasks package
// and used by components that need to create normalizers dynamically.
type NormalizerFactory interface {
	// CreateNormalizer creates a normalizer for the given task type.
	// Returns an error if the task type is not supported.
	CreateNormalizer(ctx context.Context, taskType task.Type) (TaskNormalizer, error)
}
