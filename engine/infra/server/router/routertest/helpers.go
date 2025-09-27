package routertest

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/testutil"
	"github.com/compozy/compozy/pkg/config"
)

// StubTaskRepo wraps the in-memory repo with error injection for repository lookups.
type StubTaskRepo struct {
	*testutil.InMemoryRepo
	err error
}

// NewStubTaskRepo creates a stub repository ready for handler tests.
func NewStubTaskRepo() *StubTaskRepo {
	return &StubTaskRepo{InMemoryRepo: testutil.NewInMemoryRepo()}
}

// SetError configures the repository to return the provided error on GetState.
func (s *StubTaskRepo) SetError(err error) {
	s.err = err
}

// GetState returns a stored task state or mimics not-found semantics.
func (s *StubTaskRepo) GetState(ctx context.Context, id core.ID) (*task.State, error) {
	if s.err != nil {
		return nil, s.err
	}
	state, err := s.InMemoryRepo.GetState(ctx, id)
	if err != nil {
		return nil, store.ErrTaskNotFound
	}
	return state, nil
}

// NewTestAppState builds an app state with isolated project configuration.
func NewTestAppState(t *testing.T) *appstate.State {
	t.Helper()
	proj := &project.Config{Name: "test"}
	requireNoError(t, proj.SetCWD(t.TempDir()))
	deps := appstate.NewBaseDeps(proj, nil, nil, nil)
	state, err := appstate.NewState(deps, nil)
	requireNoError(t, err)
	return state
}

// WithConfig injects a configuration manager into the request context.
func WithConfig(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(context.Background(), config.NewDefaultProvider())
	requireNoError(t, err)
	ctx := config.ContextWithManager(req.Context(), manager)
	return req.WithContext(ctx)
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// NewResourceStore registers an in-memory resource store on the provided state.
func NewResourceStore(state *appstate.State) resources.ResourceStore {
	store := resources.NewMemoryResourceStore()
	state.SetResourceStore(store)
	return store
}

// ComposeLocation builds the expected Location header for async responses.
func ComposeLocation(component string, execID core.ID) string {
	return routes.Executions() + "/" + component + "/" + execID.String()
}
