package appstate

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/infra/temporal"
	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	StateKey contextKey = "app_state"
)

type BaseDeps struct {
	TemporalClient *temporal.Client
	Store          *store.Store
	ProjectConfig  *project.Config
	Workflows      []*workflow.Config
}

func NewBaseDeps(
	tc *temporal.Client,
	store *store.Store,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) BaseDeps {
	return BaseDeps{
		TemporalClient: tc,
		Store:          store,
		ProjectConfig:  projectConfig,
		Workflows:      workflows,
	}
}

type State struct {
	BaseDeps
	CWD                        *core.CWD
	Orchestrator               *orchestrator.Orchestrator
	TemporalOrchestratorConfig *orchestrator.Config
}

func NewState(deps BaseDeps, orch *orchestrator.Orchestrator) (*State, error) {
	if deps.ProjectConfig == nil {
		return nil, fmt.Errorf("project config is required")
	}
	cwd := deps.ProjectConfig.GetCWD()
	if cwd == nil {
		return nil, fmt.Errorf("project config must have a valid CWD")
	}
	return &State{
		CWD:                        cwd,
		BaseDeps:                   deps,
		Orchestrator:               orch,
		TemporalOrchestratorConfig: orch.Config(),
	}, nil
}

func WithState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, StateKey, state)
}

func GetState(ctx context.Context) (*State, error) {
	state, ok := ctx.Value(StateKey).(*State)
	if !ok {
		return nil, fmt.Errorf("temporal app state not found in context")
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
