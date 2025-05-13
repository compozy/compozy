package server

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/nats"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/parser/workflow"
)

type contextKey string

const (
	AppStateKey contextKey = "app_state"
)

type AppState struct {
	CWD           *common.CWD
	ProjectConfig *project.Config
	Workflows     []*workflow.Config
	NatsServer    *nats.Server
}

func NewAppState(projectConfig *project.Config, workflows []*workflow.Config, natsServer *nats.Server) (*AppState, error) {
	// ProjectConfig must be provided and have a valid CWD
	if projectConfig == nil {
		return nil, fmt.Errorf("project config is required")
	}

	cwd := projectConfig.GetCWD()
	if cwd == nil {
		return nil, fmt.Errorf("project config must have a valid CWD")
	}

	return &AppState{
		CWD:           cwd,
		ProjectConfig: projectConfig,
		Workflows:     workflows,
		NatsServer:    natsServer,
	}, nil
}

func WithAppState(ctx context.Context, state *AppState) context.Context {
	return context.WithValue(ctx, AppStateKey, state)
}

func GetAppState(ctx context.Context) (*AppState, error) {
	state, ok := ctx.Value(AppStateKey).(*AppState)
	if !ok {
		return nil, NewServerError(ErrInternalCode, "App state not found in context")
	}
	return state, nil
}
