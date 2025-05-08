package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/nats"
	"github.com/compozy/compozy/internal/parser/workflow"
)

// Context keys
type contextKey string

const (
	AppStateKey contextKey = "app_state"
)

// AppState contains the state shared across the server
type AppState struct {
	CWD        string
	Workflows  []*workflow.WorkflowConfig
	NatsServer *nats.NatsServer
}

// NewAppState creates a new AppState
func NewAppState(cwd string, workflows []*workflow.WorkflowConfig, natsServer *nats.NatsServer) (*AppState, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, NewServerError(ErrInternalCode, "Failed to get current working directory")
		}
	}

	if !filepath.IsAbs(cwd) {
		absPath, err := filepath.Abs(cwd)
		if err != nil {
			return nil, NewServerError(ErrInternalCode, "Failed to resolve absolute path")
		}
		cwd = absPath
	}

	if workflows == nil {
		workflows = []*workflow.WorkflowConfig{}
	}

	return &AppState{
		CWD:        cwd,
		Workflows:  workflows,
		NatsServer: natsServer,
	}, nil
}

// WithAppState adds the app state to the context
func WithAppState(ctx context.Context, state *AppState) context.Context {
	return context.WithValue(ctx, AppStateKey, state)
}

// GetAppState retrieves the app state from the context
func GetAppState(ctx context.Context) (*AppState, error) {
	state, ok := ctx.Value(AppStateKey).(*AppState)
	if !ok {
		return nil, NewServerError(ErrInternalCode, "App state not found in context")
	}
	return state, nil
}
