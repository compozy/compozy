package task

import (
	"context"

	"github.com/compozy/agh/internal/network/participation"
)

// CreateTaskDefinitionMutation commits a task definition, optional execution
// profile intent, and every causal event as one persistence command.
type CreateTaskDefinitionMutation struct {
	Task    Task
	Profile *ExecutionProfile
	Events  []Event
}

// UpdateTaskDefinitionMutation commits a task-row update, optional execution
// profile intent, and every causal event as one persistence command.
type UpdateTaskDefinitionMutation struct {
	Task                      Task
	UpdateTaskRow             bool
	PatchNetworkParticipation bool
	NetworkParticipation      *participation.Request
	Actor                     ActorContext
	Events                    []Event
}

// ExecutionProfileMutation commits one profile replacement and its event.
type ExecutionProfileMutation struct {
	Profile ExecutionProfile
	Event   Event
}

// ExecutionProfileDeleteMutation commits one profile deletion and its event.
type ExecutionProfileDeleteMutation struct {
	TaskID string
	Event  Event
}

// DefinitionStore owns atomic task-definition and profile commands.
type DefinitionStore interface {
	CreateTaskDefinition(ctx context.Context, mutation CreateTaskDefinitionMutation) error
	UpdateTaskDefinition(ctx context.Context, mutation *UpdateTaskDefinitionMutation) (ExecutionProfile, error)
	SetTaskExecutionProfile(ctx context.Context, mutation *ExecutionProfileMutation) (ExecutionProfile, error)
	DeleteTaskExecutionProfile(ctx context.Context, mutation ExecutionProfileDeleteMutation) error
}
