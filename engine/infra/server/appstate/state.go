package appstate

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	StateKey           contextKey = "app_state"
	ScheduleManagerKey string     = "scheduleManager"
	WebhookRegistryKey string     = "webhook.registry"
)

type BaseDeps struct {
	Store         *store.Store
	ProjectConfig *project.Config
	Workflows     []*workflow.Config
	ClientConfig  *worker.TemporalConfig
}

func NewBaseDeps(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	store *store.Store,
	clientConfig *worker.TemporalConfig,
) BaseDeps {
	return BaseDeps{
		ProjectConfig: projectConfig,
		Workflows:     workflows,
		Store:         store,
		ClientConfig:  clientConfig,
	}
}

type State struct {
	BaseDeps
	CWD        *core.PathCWD
	Worker     *worker.Worker
	Extensions map[string]any
}

func NewState(deps BaseDeps, worker *worker.Worker) (*State, error) {
	if deps.ProjectConfig == nil {
		return nil, fmt.Errorf("project config is required")
	}
	cwd := deps.ProjectConfig.GetCWD()
	if cwd == nil {
		return nil, fmt.Errorf("project config must have a valid CWD")
	}
	return &State{
		CWD:        cwd,
		BaseDeps:   deps,
		Worker:     worker,
		Extensions: make(map[string]any),
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
