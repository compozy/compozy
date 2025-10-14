package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/knowledge/uc"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingIngestExecutor struct {
	store    resources.ResourceStore
	contexts []context.Context
	calls    []*uc.IngestInput
	err      error
}

func (e *recordingIngestExecutor) Execute(ctx context.Context, in *uc.IngestInput) (*uc.IngestOutput, error) {
	e.contexts = append(e.contexts, ctx)
	e.calls = append(e.calls, in)
	if e.err != nil {
		return nil, e.err
	}
	return &uc.IngestOutput{}, nil
}

func TestIngestKnowledgeBasesOnStart_TriggersOnStartBases(t *testing.T) {
	t.Run("Should trigger ingest for startup-scoped bases", func(t *testing.T) {
		origFactory := newStartupIngestExecutor
		defer func() { newStartupIngestExecutor = origFactory }()
		rec := &recordingIngestExecutor{}
		newStartupIngestExecutor = func(store resources.ResourceStore) startupIngestExecutor {
			rec.store = store
			return rec
		}
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider(), appconfig.NewEnvProvider())
		require.NoError(t, err)
		ctx := logger.ContextWithLogger(
			appconfig.ContextWithManager(context.Background(), manager),
			logger.NewForTests(),
		)
		state := &appstate.State{}
		store := resources.NewMemoryResourceStore()
		state.SetResourceStore(store)
		projectConfig := &project.Config{Name: "demo"}
		require.NoError(t, projectConfig.SetCWD(t.TempDir()))
		projectConfig.KnowledgeBases = []knowledge.BaseConfig{
			{
				ID:       "manual_kb",
				Embedder: "embedder",
				VectorDB: "vector",
				Ingest:   knowledge.IngestManual,
			},
			{
				ID:       "startup_kb",
				Embedder: "embedder",
				VectorDB: "vector",
				Ingest:   knowledge.IngestOnStart,
			},
		}
		err = ingestKnowledgeBasesOnStart(ctx, state, projectConfig, nil)
		require.NoError(t, err)
		require.Len(t, rec.calls, 1)
		assert.Equal(t, store, rec.store)
		call := rec.calls[0]
		assert.Equal(t, "demo", call.Project)
		assert.Equal(t, "startup_kb", call.ID)
		assert.Equal(t, knowledge.IngestOnStart, projectConfig.KnowledgeBases[1].Ingest)
		assert.Equal(t, rec.calls[0].Strategy, ingest.StrategyReplace)
		assert.NotNil(t, call.CWD)
		assert.Equal(t, projectConfig.GetCWD(), call.CWD)
	})
}

func TestIngestKnowledgeBasesOnStart_PropagatesErrors(t *testing.T) {
	t.Run("Should propagate execution errors", func(t *testing.T) {
		origFactory := newStartupIngestExecutor
		defer func() { newStartupIngestExecutor = origFactory }()
		rec := &recordingIngestExecutor{err: errors.New("ingest failure")}
		newStartupIngestExecutor = func(store resources.ResourceStore) startupIngestExecutor {
			rec.store = store
			return rec
		}
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider(), appconfig.NewEnvProvider())
		require.NoError(t, err)
		ctx := logger.ContextWithLogger(
			appconfig.ContextWithManager(context.Background(), manager),
			logger.NewForTests(),
		)
		state := &appstate.State{}
		state.SetResourceStore(resources.NewMemoryResourceStore())
		projectConfig := &project.Config{Name: "demo"}
		require.NoError(t, projectConfig.SetCWD(t.TempDir()))
		projectConfig.KnowledgeBases = []knowledge.BaseConfig{
			{
				ID:       "startup_kb",
				Embedder: "embedder",
				VectorDB: "vector",
				Ingest:   knowledge.IngestOnStart,
			},
		}
		err = ingestKnowledgeBasesOnStart(ctx, state, projectConfig, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "startup ingest for \"startup_kb\" failed")
		require.Len(t, rec.calls, 1)
	})
}

func TestIngestKnowledgeBasesOnStart_AppliesKnowledgeTimeout(t *testing.T) {
	t.Run("Should apply configured ingest timeout", func(t *testing.T) {
		t.Setenv("SERVER_KNOWLEDGE_INGEST_TIMEOUT", "2s")
		origFactory := newStartupIngestExecutor
		defer func() { newStartupIngestExecutor = origFactory }()
		rec := &recordingIngestExecutor{}
		newStartupIngestExecutor = func(store resources.ResourceStore) startupIngestExecutor {
			rec.store = store
			return rec
		}
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider(), appconfig.NewEnvProvider())
		require.NoError(t, err)
		ctx := logger.ContextWithLogger(
			appconfig.ContextWithManager(context.Background(), manager),
			logger.NewForTests(),
		)
		state := &appstate.State{}
		state.SetResourceStore(resources.NewMemoryResourceStore())
		projectConfig := &project.Config{Name: "demo"}
		require.NoError(t, projectConfig.SetCWD(t.TempDir()))
		projectConfig.KnowledgeBases = []knowledge.BaseConfig{
			{
				ID:       "startup_kb",
				Embedder: "embedder",
				VectorDB: "vector",
				Ingest:   knowledge.IngestOnStart,
			},
		}
		start := time.Now()
		err = ingestKnowledgeBasesOnStart(ctx, state, projectConfig, nil)
		require.NoError(t, err)
		require.Len(t, rec.contexts, 1)
		deadline, ok := rec.contexts[0].Deadline()
		require.True(t, ok, "expected context with deadline")
		elapsed := deadline.Sub(start)
		assert.InDelta(t, 2*time.Second, elapsed, float64(200*time.Millisecond))
	})
}

func TestIngestKnowledgeBasesOnStart_IncludesWorkflowBases(t *testing.T) {
	t.Run("Should include workflow startup knowledge bases", func(t *testing.T) {
		origFactory := newStartupIngestExecutor
		defer func() { newStartupIngestExecutor = origFactory }()
		rec := &recordingIngestExecutor{}
		newStartupIngestExecutor = func(store resources.ResourceStore) startupIngestExecutor {
			rec.store = store
			return rec
		}
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider(), appconfig.NewEnvProvider())
		require.NoError(t, err)
		ctx := logger.ContextWithLogger(
			appconfig.ContextWithManager(context.Background(), manager),
			logger.NewForTests(),
		)
		state := &appstate.State{}
		store := resources.NewMemoryResourceStore()
		state.SetResourceStore(store)
		projectConfig := &project.Config{Name: "demo"}
		require.NoError(t, projectConfig.SetCWD(t.TempDir()))
		wf := &workflow.Config{
			ID: "ticket-escalation",
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "wf_on_start",
					Embedder: "embedder",
					VectorDB: "vector",
					Ingest:   knowledge.IngestOnStart,
				},
			},
		}
		err = ingestKnowledgeBasesOnStart(ctx, state, projectConfig, []*workflow.Config{wf})
		require.NoError(t, err)
		require.Len(t, rec.calls, 1)
		assert.Equal(t, "wf_on_start", rec.calls[0].ID)
	})
}

func TestIngestKnowledgeBasesOnStart_DetectsDuplicateIDs(t *testing.T) {
	t.Run("Should reject duplicate startup knowledge base IDs", func(t *testing.T) {
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider(), appconfig.NewEnvProvider())
		require.NoError(t, err)
		ctx := logger.ContextWithLogger(
			appconfig.ContextWithManager(context.Background(), manager),
			logger.NewForTests(),
		)
		state := &appstate.State{}
		state.SetResourceStore(resources.NewMemoryResourceStore())
		projectConfig := &project.Config{Name: "demo"}
		require.NoError(t, projectConfig.SetCWD(t.TempDir()))
		projectConfig.KnowledgeBases = []knowledge.BaseConfig{
			{
				ID:       "duplicate",
				Embedder: "embedder",
				VectorDB: "vector",
				Ingest:   knowledge.IngestOnStart,
			},
		}
		wf := &workflow.Config{
			ID: "wf-dup",
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "duplicate",
					Embedder: "embedder",
					VectorDB: "vector",
					Ingest:   knowledge.IngestOnStart,
				},
			},
		}
		err = ingestKnowledgeBasesOnStart(ctx, state, projectConfig, []*workflow.Config{wf})
		require.Error(t, err)
		assert.ErrorContains(t, err, "knowledge_base \"duplicate\" configured for startup ingestion")
	})
}
