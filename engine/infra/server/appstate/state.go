package appstate

import (
	"context"
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	stateKey contextKey = "app_state"
)

// ExtensionKey is a distinct type for keys stored in State.Extensions to avoid
// accidental collisions and stringly-typed access across the codebase.
// Prefer using helper methods that rely on these typed keys.
type ExtensionKey string

const (
	extensionScheduleManagerKey ExtensionKey = "scheduleManager"
	extensionWebhookRegistryKey ExtensionKey = "webhook.registry"
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
	mu         sync.RWMutex
	Extensions map[ExtensionKey]any
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
		Extensions: make(map[ExtensionKey]any),
	}, nil
}

func WithState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, stateKey, state)
}

func GetState(ctx context.Context) (*State, error) {
	state, ok := ctx.Value(stateKey).(*State)
	if !ok {
		return nil, fmt.Errorf("app state not found in context")
	}
	return state, nil
}

// SetWebhookRegistry stores the webhook registry in extensions with type safety
func (s *State) SetWebhookRegistry(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Extensions == nil {
		s.Extensions = make(map[ExtensionKey]any)
	}
	s.Extensions[extensionWebhookRegistryKey] = v
}

// WebhookRegistry retrieves the webhook registry from extensions with type safety
func (s *State) WebhookRegistry() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Extensions[extensionWebhookRegistryKey]
	return v, ok
}

// SetScheduleManager stores the schedule manager in extensions with type safety
func (s *State) SetScheduleManager(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Extensions == nil {
		s.Extensions = make(map[ExtensionKey]any)
	}
	s.Extensions[extensionScheduleManagerKey] = v
}

// ScheduleManager retrieves the schedule manager from extensions with type safety
func (s *State) ScheduleManager() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Extensions[extensionScheduleManagerKey]
	return v, ok
}

func StateMiddleware(state *State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := WithState(c.Request.Context(), state)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
