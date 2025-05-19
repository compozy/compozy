package server

import (
	"context"

	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

// ComponentManager handles the initialization and lifecycle of all server components
type ComponentManager struct {
	state            *AppState
	workflowExecutor *workflow.Executor
}

// NewComponentManager creates a new component manager that initializes
// and manages the lifecycle of all server components
func NewComponentManager(state *AppState) (*ComponentManager, error) {
	// Initialize workflow executor with workflows from the app state
	executor, err := workflow.NewExecutor(state.NatsServer, state.Workflows, nil)
	if err != nil {
		return nil, err
	}

	return &ComponentManager{
		state:            state,
		workflowExecutor: executor,
	}, nil
}

// Start initializes all components
func (cm *ComponentManager) Start(ctx context.Context) error {
	logger.Info("Starting server components")

	// Start workflow executor
	if err := cm.workflowExecutor.Start(ctx); err != nil {
		return err
	}

	logger.Info("All server components started")
	return nil
}

// Stop gracefully shuts down all components
func (cm *ComponentManager) Stop() error {
	logger.Info("Stopping server components")

	// Stop workflow executor
	if err := cm.workflowExecutor.Stop(); err != nil {
		logger.Error("Error stopping workflow executor", "error", err)
	}

	logger.Info("All server components stopped")
	return nil
}
