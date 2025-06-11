package services

import (
	"context"

	"github.com/compozy/compozy/engine/task"
)

// ConfigStore provides persistent storage for task configurations
// keyed by their TaskExecID. This allows workers to avoid shipping
// large config objects through Temporal history and enables
// retrieval of generated child configs for collection/parallel tasks.
type ConfigStore interface {
	// Save persists a task configuration with the given taskExecID as key
	Save(ctx context.Context, taskExecID string, config *task.Config) error

	// Get retrieves a task configuration by taskExecID
	Get(ctx context.Context, taskExecID string) (*task.Config, error)

	// Delete removes a task configuration by taskExecID
	// This can be called when a task reaches terminal status to save space
	Delete(ctx context.Context, taskExecID string) error

	// Close closes the underlying storage and releases resources
	Close() error
}
