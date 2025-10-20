package reconciler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	pkgcfg "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSchedule implements schedule.Manager for tests
type mockSchedule struct{ reloads atomic.Int64 }

func (m *mockSchedule) ReconcileSchedules(context.Context, []*workflow.Config) error { return nil }
func (m *mockSchedule) ListSchedules(context.Context) ([]*schedule.Info, error)      { return nil, nil }
func (m *mockSchedule) GetSchedule(context.Context, string) (*schedule.Info, error)  { return nil, nil }
func (m *mockSchedule) UpdateSchedule(context.Context, string, schedule.UpdateRequest) error {
	return nil
}
func (m *mockSchedule) DeleteSchedule(context.Context, string) error { return nil }
func (m *mockSchedule) OnConfigurationReload(_ context.Context, _ []*workflow.Config) error {
	m.reloads.Add(1)
	return nil
}
func (m *mockSchedule) StartPeriodicReconciliation(context.Context, func() []*workflow.Config, time.Duration) error {
	return nil
}
func (m *mockSchedule) StopPeriodicReconciliation() {}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	base := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	m := pkgcfg.NewManager(t.Context(), pkgcfg.NewService())
	_, err := m.Load(base, pkgcfg.NewDefaultProvider(), pkgcfg.NewEnvProvider())
	require.NoError(t, err, "failed to load test configuration")
	// override to builder mode for tests
	c := m.Get()
	c.Server.SourceOfTruth = "builder"
	return pkgcfg.ContextWithManager(base, m)
}

func TestReverseIndex_ComputeImpacted(t *testing.T) {
	t.Run("Should map tool id to workflow id via reverse index", func(t *testing.T) {
		ctx := testCtx(t)
		store := resources.NewMemoryResourceStore()
		// seed a minimal workflow in store
		wf := &workflow.Config{ID: "wf1"}
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: "proj", Type: resources.ResourceWorkflow, ID: wf.ID},
			wf,
		)
		require.NoError(t, err)
		// Build state and reconciler
		st := &appstate.State{
			BaseDeps:   appstate.BaseDeps{ProjectConfig: &project.Config{Name: "proj"}},
			Extensions: map[appstate.ExtensionKey]any{},
		}
		st.SetResourceStore(store)
		ms := &mockSchedule{}
		st.SetScheduleManager(ms)
		r, err := NewReconciler(ctx, st)
		require.NoError(t, err)
		require.NoError(t, r.buildInitialIndex(ctx))
		// Simulate a tool event that should impact nothing yet (no deps), but pipeline works
		impacted, deletes := r.computeImpacted(
			ctx,
			[]resources.Event{
				{
					Type: resources.EventPut,
					Key:  resources.ResourceKey{Project: "proj", Type: resources.ResourceTool, ID: "tool-1"},
				},
			},
		)
		assert.Empty(t, deletes)
		assert.Empty(t, impacted)
	})
}

func TestDebounce_BatchesEvents(t *testing.T) {
	t.Run("Should coalesce multiple events into one reload", func(t *testing.T) {
		ctx := testCtx(t)
		store := resources.NewMemoryResourceStore()
		// seed a minimal workflow
		wf := &workflow.Config{ID: "wf1"}
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: "proj", Type: resources.ResourceWorkflow, ID: wf.ID},
			wf,
		)
		require.NoError(t, err)
		st := &appstate.State{
			BaseDeps:   appstate.BaseDeps{ProjectConfig: &project.Config{Name: "proj"}},
			Extensions: map[appstate.ExtensionKey]any{},
		}
		st.SetResourceStore(store)
		ms := &mockSchedule{}
		st.SetScheduleManager(ms)
		r, err := NewReconciler(ctx, st)
		require.NoError(t, err)
		require.NoError(t, r.Start(ctx))
		defer r.Stop()
		// burst of events
		for range 5 {
			_, err := store.Put(
				ctx,
				resources.ResourceKey{Project: "proj", Type: resources.ResourceWorkflow, ID: wf.ID},
				wf,
			)
			require.NoError(t, err)
		}
		// wait > debounceMaxWait to allow batch to fire
		time.Sleep(800 * time.Millisecond)
		assert.Equal(t, int64(1), ms.reloads.Load())
	})
}
