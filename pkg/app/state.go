package app

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	StateKey contextKey = "app_state"
)

type State struct {
	CWD           *common.CWD
	ProjectConfig *project.Config
	Workflows     []*workflow.Config
	NatsServer    *nats.Server
	Orchestrator  *orchestrator.Orchestrator
}

func NewState(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	natsServer *nats.Server,
) (*State, error) {
	if projectConfig == nil {
		return nil, fmt.Errorf("project config is required")
	}

	cwd := projectConfig.GetCWD()
	if cwd == nil {
		return nil, fmt.Errorf("project config must have a valid CWD")
	}

	return &State{
		CWD:           cwd,
		ProjectConfig: projectConfig,
		Workflows:     workflows,
		NatsServer:    natsServer,
	}, nil
}

func WithState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, StateKey, state)
}

func GetState(ctx context.Context) (*State, error) {
	state, ok := ctx.Value(StateKey).(*State)
	if !ok {
		return nil, fmt.Errorf("app state not found in context")
	}
	return state, nil
}

func StateMiddleware(state *State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := WithState(c.Request.Context(), state)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
