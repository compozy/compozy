//go:build integration

package modelcatalog_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	_ "modernc.org/sqlite"
)

func TestCatalogServiceGlobalDBIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh and list rows by provider with global DB store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, _ := openCatalogGlobalDB(t)
		source := modelcatalog.NewConfigSource(map[string]aghconfig.ProviderConfig{
			"codex": {
				Models: aghconfig.ProviderModelsConfig{
					Default: "manual-model",
					Curated: []aghconfig.ProviderModelConfig{
						{ID: "gpt-5.4", DisplayName: "GPT-5.4"},
					},
				},
			},
			"claude": {
				Models: aghconfig.ProviderModelsConfig{
					Default: "claude-sonnet-4-6",
				},
			},
		})
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if _, err := service.Refresh(
			ctx,
			modelcatalog.RefreshOptions{Force: true, Now: integrationTime(0)},
		); err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "codex", Now: integrationTime(1)})
		if err != nil {
			t.Fatalf("ListModels(codex) error = %v", err)
		}
		if got, want := len(models), 1; got != want {
			t.Fatalf("len(curated models) = %d, want %d: %#v", got, want, models)
		}
		if models[0].ProviderID != "codex" || models[0].ModelID != "gpt-5.4" ||
			!models[0].ExplicitlyCurated || !models[0].Curated {
			t.Fatalf("curated models = %#v, want explicit codex/gpt-5.4", models)
		}
		allModels, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID: "codex",
			View:       modelcatalog.CatalogViewAll,
			Now:        integrationTime(1),
		})
		if err != nil {
			t.Fatalf("ListModels(codex all) error = %v", err)
		}
		if got, want := integrationModelIDs(allModels), []string{"gpt-5.4", "manual-model"}; !slices.Equal(got, want) {
			t.Fatalf("all model ids = %#v, want %#v", got, want)
		}
		if allModels[1].ExplicitlyCurated || allModels[1].Curated {
			t.Fatalf("manual default curation = %#v, want default-only outside curated view", allModels[1])
		}
		claudeModels, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID: "claude",
			Now:        integrationTime(1),
		})
		if err != nil {
			t.Fatalf("ListModels(claude) error = %v", err)
		}
		if len(claudeModels) != 1 || claudeModels[0].ModelID != "claude-sonnet-4-6" ||
			!claudeModels[0].Curated || claudeModels[0].ExplicitlyCurated {
			t.Fatalf("claude fallback models = %#v, want visible default-only fallback", claudeModels)
		}
	})

	t.Run("Should not persist raw models dev upstream payload", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, path := openCatalogGlobalDB(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprint(w, `{
				"openai": {
					"models": {
						"gpt-5.4": {
							"name": "GPT-5.4",
							"unused_raw_marker": "raw_secret_marker_should_not_persist"
						}
					}
				}
			}`)
			if err != nil {
				t.Errorf("Fprint(response) error = %v", err)
			}
		}))
		t.Cleanup(server.Close)
		enabled := true
		source, err := modelcatalog.NewModelsDevSource(nil, aghconfig.ModelsDevSourceConfig{
			Enabled:  &enabled,
			Endpoint: server.URL,
			TTL:      "1h",
			Timeout:  "1s",
		})
		if err != nil {
			t.Fatalf("NewModelsDevSource() error = %v", err)
		}
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if _, err := service.Refresh(
			ctx,
			modelcatalog.RefreshOptions{ProviderID: "codex", Force: true, Now: integrationTime(0)},
		); err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}

		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := db.Close(); closeErr != nil {
				t.Errorf("db.Close() error = %v", closeErr)
			}
		})
		var matches int
		if err := db.QueryRowContext(
			ctx,
			`SELECT
				(SELECT COUNT(*) FROM model_catalog_rows
				 WHERE source_id LIKE ? OR provider_id LIKE ? OR model_id LIKE ? OR display_name LIKE ? OR last_error LIKE ?)
				+
				(SELECT COUNT(*) FROM model_catalog_sources
				 WHERE source_id LIKE ? OR provider_id LIKE ? OR last_error LIKE ?)`,
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
		).Scan(&matches); err != nil {
			t.Fatalf("QueryRowContext(raw marker) error = %v", err)
		}
		if matches != 0 {
			t.Fatalf("raw marker persisted in %d catalog fields, want 0", matches)
		}
	})

	t.Run("Should coalesce same provider refreshes without SQLite busy failures", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, _ := openCatalogGlobalDB(t)
		source := newIntegrationBlockingSource(map[string][]modelcatalog.ModelRow{
			"codex": {
				integrationRow("codex", "gpt-5.4", integrationTime(20)),
			},
		})
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		results := make(chan error, 2)
		for range 2 {
			go func() {
				_, refreshErr := service.Refresh(ctx, modelcatalog.RefreshOptions{
					ProviderID: "codex",
					SourceID:   source.ID(),
					Force:      true,
					Now:        integrationTime(20),
				})
				results <- refreshErr
			}()
		}
		source.waitForCalls(t, 1)
		source.requireCallCountStable(t, 1, 25*time.Millisecond)
		source.release()

		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "codex", Now: integrationTime(21)})
		if err != nil {
			t.Fatalf("ListModels(codex) error = %v", err)
		}
		if got, want := integrationModelKeys(models), []string{"codex/gpt-5.4"}; !slices.Equal(got, want) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
	})

	t.Run("Should persist concurrent cross provider refreshes without SQLite busy failures", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, _ := openCatalogGlobalDB(t)
		source := newIntegrationBlockingSource(map[string][]modelcatalog.ModelRow{
			"claude": {
				integrationRow("claude", "claude-sonnet-4-6", integrationTime(30)),
			},
			"codex": {
				integrationRow("codex", "gpt-5.4", integrationTime(30)),
			},
		})
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		results := make(chan error, 2)
		for _, providerID := range []string{"codex", "claude"} {
			go func(providerID string) {
				_, refreshErr := service.Refresh(ctx, modelcatalog.RefreshOptions{
					ProviderID: providerID,
					SourceID:   source.ID(),
					Force:      true,
					Now:        integrationTime(30),
				})
				results <- refreshErr
			}(providerID)
		}
		source.waitForCalls(t, 2)
		source.release()

		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{Now: integrationTime(31)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := integrationModelKeys(
			models,
		), []string{
			"claude/claude-sonnet-4-6",
			"codex/gpt-5.4",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
	})
}

func openCatalogGlobalDB(t *testing.T) (*globaldb.GlobalDB, string) {
	t.Helper()

	ctx := testutil.Context(t)
	path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	store, err := globaldb.OpenGlobalDB(ctx, path)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(testutil.Context(t)); closeErr != nil {
			t.Errorf("GlobalDB.Close() error = %v", closeErr)
		}
	})
	return store, path
}

func integrationTime(offset int) time.Time {
	return time.Date(2026, 5, 7, 13, offset, 0, 0, time.UTC)
}

type integrationBlockingSource struct {
	mu             sync.Mutex
	rowsByProvider map[string][]modelcatalog.ModelRow
	calls          int
	callsCh        chan int
	releaseCh      chan struct{}
	releaseOnce    sync.Once
}

func newIntegrationBlockingSource(rowsByProvider map[string][]modelcatalog.ModelRow) *integrationBlockingSource {
	return &integrationBlockingSource{
		rowsByProvider: rowsByProvider,
		callsCh:        make(chan int, 16),
		releaseCh:      make(chan struct{}),
	}
}

func (s *integrationBlockingSource) ID() string {
	return "provider_live:integration"
}

func (s *integrationBlockingSource) Kind() modelcatalog.SourceKind {
	return modelcatalog.SourceKindProviderLive
}

func (s *integrationBlockingSource) Priority() int {
	return modelcatalog.PriorityProviderLive
}

func (s *integrationBlockingSource) ProviderIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers := make([]string, 0, len(s.rowsByProvider))
	for providerID := range s.rowsByProvider {
		providers = append(providers, providerID)
	}
	slices.Sort(providers)
	return providers
}

func (s *integrationBlockingSource) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.ModelRow, error) {
	s.mu.Lock()
	s.calls++
	calls := s.calls
	s.mu.Unlock()
	select {
	case s.callsCh <- calls:
	default:
	}

	select {
	case <-s.releaseCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	s.mu.Lock()
	rows := cloneIntegrationRows(s.rowsByProvider[opts.ProviderID])
	s.mu.Unlock()
	return rows, nil
}

func (s *integrationBlockingSource) waitForCalls(t *testing.T, want int) {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		if s.callCount() >= want {
			return
		}
		select {
		case <-s.callsCh:
		case <-deadline:
			t.Fatalf("source calls = %d, want at least %d", s.callCount(), want)
		}
	}
}

func (s *integrationBlockingSource) requireCallCountStable(
	t *testing.T,
	want int,
	duration time.Duration,
) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case <-s.callsCh:
			if got := s.callCount(); got > want {
				t.Fatalf("source calls = %d while first refresh was blocked, want at most %d", got, want)
			}
		case <-timer.C:
			return
		}
	}
}

func (s *integrationBlockingSource) release() {
	s.releaseOnce.Do(func() {
		close(s.releaseCh)
	})
}

func (s *integrationBlockingSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func integrationRow(providerID string, modelID string, refreshedAt time.Time) modelcatalog.ModelRow {
	return modelcatalog.ModelRow{
		SourceID:    "provider_live:integration",
		SourceKind:  modelcatalog.SourceKindProviderLive,
		Priority:    modelcatalog.PriorityProviderLive,
		ProviderID:  providerID,
		ModelID:     modelID,
		RefreshedAt: refreshedAt,
	}
}

func cloneIntegrationRows(rows []modelcatalog.ModelRow) []modelcatalog.ModelRow {
	return append([]modelcatalog.ModelRow(nil), rows...)
}

func integrationModelKeys(models []modelcatalog.Model) []string {
	keys := make([]string, 0, len(models))
	for _, model := range models {
		keys = append(keys, model.ProviderID+"/"+model.ModelID)
	}
	return keys
}

func integrationModelIDs(models []modelcatalog.Model) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ModelID)
	}
	return ids
}
