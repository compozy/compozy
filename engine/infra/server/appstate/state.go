package appstate

import (
	"context"
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/pubsub"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/webhook"
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
	extensionScheduleManagerKey     ExtensionKey = "scheduleManager"
	extensionWebhookRegistryKey     ExtensionKey = "webhook.registry"
	extensionResourceStoreKey       ExtensionKey = "resource.store"
	extensionConfigRegistryKey      ExtensionKey = "config.registry"
	extensionAPIIdempotencyKey      ExtensionKey = "api.idempotency"
	extensionMonitoringServiceKey   ExtensionKey = "monitoring.service"
	extensionWorkflowQueryClientKey ExtensionKey = "workflow.query.client"
	extensionPubSubProviderKey      ExtensionKey = "pubsub.provider"
)

type BaseDeps struct {
	Store         *repo.Provider
	ProjectConfig *project.Config
	Workflows     []*workflow.Config
	ClientConfig  *worker.TemporalConfig
}

func NewBaseDeps(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	store *repo.Provider,
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

// SetExtension stores an arbitrary value in the extensions map using a typed key.
// Passing a nil value removes the extension to keep the map stable for tests and runtime code.
func (s *State) SetExtension(key ExtensionKey, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Extensions == nil {
		s.Extensions = make(map[ExtensionKey]any)
	}
	if value == nil {
		delete(s.Extensions, key)
		return
	}
	s.Extensions[key] = value
}

// Extension retrieves an arbitrary value stored for the provided key.
func (s *State) Extension(key ExtensionKey) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Extensions == nil {
		return nil, false
	}
	v, ok := s.Extensions[key]
	if !ok || v == nil {
		return nil, false
	}
	return v, true
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

// SetWorkflowQueryClient stores the workflow query client adapter in extensions.
func (s *State) SetWorkflowQueryClient(v any) {
	s.SetExtension(extensionWorkflowQueryClientKey, v)
}

// WorkflowQueryClient retrieves the stored workflow query client adapter if present.
func (s *State) WorkflowQueryClient() (any, bool) {
	return s.Extension(extensionWorkflowQueryClientKey)
}

// SetResourceStore stores the resources.ResourceStore in extensions with type safety
func (s *State) SetResourceStore(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Extensions == nil {
		s.Extensions = make(map[ExtensionKey]any)
	}
	s.Extensions[extensionResourceStoreKey] = v
}

// ResourceStore retrieves the resources.ResourceStore from extensions with type safety
func (s *State) ResourceStore() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Extensions[extensionResourceStoreKey]
	return v, ok
}

// SetConfigRegistry stores the autoload.ConfigRegistry in extensions
func (s *State) SetConfigRegistry(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Extensions == nil {
		s.Extensions = make(map[ExtensionKey]any)
	}
	s.Extensions[extensionConfigRegistryKey] = v
}

// ConfigRegistry retrieves the autoload.ConfigRegistry from extensions
func (s *State) ConfigRegistry() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Extensions[extensionConfigRegistryKey]
	return v, ok
}

// SetAPIIdempotencyService stores the webhook.Service used for API idempotency checks
func (s *State) SetAPIIdempotencyService(service webhook.Service) {
	s.SetExtension(extensionAPIIdempotencyKey, service)
}

// APIIdempotencyService retrieves the webhook.Service configured for API idempotency checks
func (s *State) APIIdempotencyService() (webhook.Service, bool) {
	v, ok := s.Extension(extensionAPIIdempotencyKey)
	if !ok {
		return nil, false
	}
	service, ok := v.(webhook.Service)
	if !ok {
		return nil, false
	}
	return service, true
}

// SetMonitoringService stores the monitoring service in extensions for reuse across handlers.
func (s *State) SetMonitoringService(service *monitoring.Service) {
	s.SetExtension(extensionMonitoringServiceKey, service)
}

// MonitoringService retrieves the monitoring service if it was configured during startup.
func (s *State) MonitoringService() (*monitoring.Service, bool) {
	v, ok := s.Extension(extensionMonitoringServiceKey)
	if !ok {
		return nil, false
	}
	service, ok := v.(*monitoring.Service)
	if !ok {
		return nil, false
	}
	return service, true
}

// ReplaceWorkflows swaps the compiled workflow set atomically under RW lock
func (s *State) ReplaceWorkflows(workflows []*workflow.Config) {
	s.mu.Lock()
	s.Workflows = workflows
	s.mu.Unlock()
}

// GetWorkflows returns the current compiled workflow set under read lock
func (s *State) GetWorkflows() []*workflow.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*workflow.Config, len(s.Workflows))
	copy(out, s.Workflows)
	return out
}

// SetPubSubProvider registers the pub/sub provider used by API handlers.
func (s *State) SetPubSubProvider(provider pubsub.Provider) {
	if provider == nil {
		s.SetExtension(extensionPubSubProviderKey, nil)
		return
	}
	s.SetExtension(extensionPubSubProviderKey, provider)
}

// PubSubProvider retrieves the configured pub/sub provider when available.
func (s *State) PubSubProvider() (pubsub.Provider, bool) {
	v, ok := s.Extension(extensionPubSubProviderKey)
	if !ok || v == nil {
		return nil, false
	}
	provider, ok := v.(pubsub.Provider)
	if !ok {
		return nil, false
	}
	return provider, true
}

func StateMiddleware(state *State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := WithState(c.Request.Context(), state)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
