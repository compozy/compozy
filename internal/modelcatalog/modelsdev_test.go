package modelcatalog

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
)

func TestModelsDevSource(t *testing.T) {
	t.Parallel()

	t.Run("Should parse current models dev fields", func(t *testing.T) {
		t.Parallel()

		server := modelsDevServer(t, http.StatusOK, `{
			"openai": {
				"id": "openai",
				"models": {
					"gpt-5.4": {
						"id": "gpt-5.4",
						"name": "GPT-5.4",
						"release_date": "2026-03-05",
						"status": "deprecated",
						"reasoning": true,
						"tool_call": true,
						"limit": {"context": 256000, "input": 200000, "output": 32000},
						"cost": {"input": 1.25, "output": 10.5, "cache_read": 0.125, "cache_write": 2.5, "reasoning": 12}
					}
				}
			}
		}`)
		source := newModelsDevTestSource(t, server.URL, "1h", "1s", true)

		rows, err := source.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(0)},
		)
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		row := requireSingleRow(t, rows)
		assertModelsDevCurrentRow(t, row)
		*row.SupportsReasoning = false
		*row.SupportsTools = false
		*row.ContextWindow = 1
		*row.MaxInputTokens = 1
		*row.MaxOutputTokens = 1
		*row.CostInputPerMillion = 0
		*row.CostOutputPerMillion = 0
		*row.CostCacheReadPerMillion = 0
		*row.CostCacheWritePerMillion = 0
		*row.CostReasoningPerMillion = 0
		*row.ReleaseDate = "mutated"
		*row.Deprecated = false

		cached, err := source.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Now: testTime(1)},
		)
		if err != nil {
			t.Fatalf("ListModels(cached) error = %v", err)
		}
		assertModelsDevCurrentRow(t, requireSingleRow(t, cached))
	})

	t.Run("Should parse legacy models dev aliases", func(t *testing.T) {
		t.Parallel()

		server := modelsDevServer(t, http.StatusOK, `{
			"anthropic": {
				"models": {
					"claude-legacy": {
						"name": "Claude Legacy",
						"supportsReasoning": true,
						"supports_tools": true,
						"contextWindow": 200000,
						"maxInputTokens": 150000,
						"maxOutputTokens": 24000,
						"pricing": {"input": 3, "output": 15}
					}
				}
			}
		}`)
		source := newModelsDevTestSource(t, server.URL, "1h", "1s", true)

		rows, err := source.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "claude", Refresh: true, Now: testTime(0)},
		)
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		row := requireSingleRow(t, rows)
		if row.ProviderID != "claude" || row.ModelID != "claude-legacy" {
			t.Fatalf("row identity = %s/%s, want claude/claude-legacy", row.ProviderID, row.ModelID)
		}
		if row.SupportsReasoning == nil || !*row.SupportsReasoning {
			t.Fatalf("SupportsReasoning = %v, want true", row.SupportsReasoning)
		}
		if row.SupportsTools == nil || !*row.SupportsTools {
			t.Fatalf("SupportsTools = %v, want true", row.SupportsTools)
		}
		if row.ContextWindow == nil || *row.ContextWindow != 200000 {
			t.Fatalf("ContextWindow = %v, want 200000", row.ContextWindow)
		}
		if row.MaxInputTokens == nil || *row.MaxInputTokens != 150000 {
			t.Fatalf("MaxInputTokens = %v, want 150000", row.MaxInputTokens)
		}
		if row.MaxOutputTokens == nil || *row.MaxOutputTokens != 24000 {
			t.Fatalf("MaxOutputTokens = %v, want 24000", row.MaxOutputTokens)
		}
		if row.CostInputPerMillion == nil || *row.CostInputPerMillion != 3 {
			t.Fatalf("CostInputPerMillion = %v, want 3", row.CostInputPerMillion)
		}
		if row.CostOutputPerMillion == nil || *row.CostOutputPerMillion != 15 {
			t.Fatalf("CostOutputPerMillion = %v, want 15", row.CostOutputPerMillion)
		}
		if row.ReleaseDate != nil {
			t.Fatalf("ReleaseDate = %v, want nil when upstream omits it", row.ReleaseDate)
		}
		if row.Deprecated != nil {
			t.Fatalf("Deprecated = %v, want nil when upstream omits status", row.Deprecated)
		}
	})

	t.Run("Should record disabled status without outbound request", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)
		source := newModelsDevTestSource(t, server.URL, "1h", "1s", false)
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		statuses, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "codex", Force: true, Now: testTime(0)},
		)
		if err != nil {
			t.Fatalf("Refresh(disabled) error = %v", err)
		}
		status := requireStatus(t, statuses, SourceIDModelsDev)
		if status.RefreshState != RefreshStateDisabled {
			t.Fatalf("RefreshState = %q, want disabled", status.RefreshState)
		}
		if got := requests.Load(); got != 0 {
			t.Fatalf("requests = %d, want 0", got)
		}
	})

	t.Run("Should apply overridden endpoint ttl and timeout", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			_, err := fmt.Fprint(w, `{"openai":{"models":{"gpt-5.4":{"name":"GPT-5.4"}}}}`)
			if err != nil {
				t.Errorf("Fprint(response) error = %v", err)
			}
		}))
		t.Cleanup(server.Close)
		source := newModelsDevTestSource(t, server.URL, "1h", "250ms", true)
		if source.Timeout() != 250*time.Millisecond {
			t.Fatalf("Timeout() = %s, want 250ms", source.Timeout())
		}
		if source.TTL() != time.Hour {
			t.Fatalf("TTL() = %s, want 1h", source.TTL())
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		if _, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "codex", Now: testTime(0)},
		); err != nil {
			t.Fatalf("Refresh(first) error = %v", err)
		}
		if _, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "codex", Now: testTime(1)},
		); err != nil {
			t.Fatalf("Refresh(second) error = %v", err)
		}
		if got := requests.Load(); got != 1 {
			t.Fatalf("requests = %d, want 1 due TTL skip", got)
		}
	})

	t.Run("Should apply explicit HTTP timeout", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)
		source := newModelsDevTestSource(t, server.URL, "1h", "20ms", true)

		started := time.Now()
		_, err := source.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(0)},
		)
		elapsed := time.Since(started)
		if err == nil {
			t.Fatal("ListModels(timeout) error = nil, want timeout error")
		}
		var timeoutErr net.Error
		if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
			t.Fatalf("ListModels(timeout) error = %v, want timeout error", err)
		}
		if elapsed >= 150*time.Millisecond {
			t.Fatalf("elapsed = %s, want timeout before server sleep completes", elapsed)
		}
	})

	t.Run("Should reject injected HTTP clients without timeouts", func(t *testing.T) {
		t.Parallel()

		enabled := true
		_, err := NewModelsDevSource(
			nil,
			aghconfig.ModelsDevSourceConfig{
				Enabled:  &enabled,
				Endpoint: "https://models.example.test/api.json",
				TTL:      "1h",
				Timeout:  "1s",
			},
			WithModelsDevHTTPClient(&http.Client{}),
		)
		if err == nil {
			t.Fatal("NewModelsDevSource(zero-timeout client) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "client timeout must be positive") {
			t.Fatalf("NewModelsDevSource(zero-timeout client) error = %v, want timeout validation", err)
		}
	})

	t.Run("Should return stale cached rows when refresh fails after prior success", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requests.Add(1)
			if count == 1 {
				_, err := fmt.Fprint(w, `{"openai":{"models":{"gpt-5.4":{"name":"GPT-5.4"}}}}`)
				if err != nil {
					t.Errorf("Fprint(first response) error = %v", err)
				}
				return
			}
			http.Error(w, "raw upstream secret sk-should-not-leak", http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		source := newModelsDevTestSource(t, server.URL, "1h", "1s", true)
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		if _, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(0)},
		); err != nil {
			t.Fatalf("ListModels(first) error = %v", err)
		}
		models, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(1)},
		)
		if err != nil {
			t.Fatalf("ListModels(exclude stale fallback) error = %v", err)
		}
		if len(models) != 0 {
			t.Fatalf("ListModels(exclude stale fallback) = %#v, want empty projection", models)
		}

		models, err = service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, IncludeStale: true, Now: testTime(1)},
		)
		if err != nil {
			t.Fatalf("ListModels(include stale fallback) error = %v", err)
		}
		model := requireSingleModel(t, models)
		if !model.Stale {
			t.Fatal("Model.Stale = false, want true")
		}
		statuses, err := service.ListSourceStatus(testutil.Context(t), "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		status := requireStatus(t, statuses, SourceIDModelsDev)
		if status.RefreshState != RefreshStateFailed || !status.Stale {
			t.Fatalf("status = %#v, want failed stale status", status)
		}
		if !status.LastSuccess.Equal(testTime(0)) {
			t.Fatalf("LastSuccess = %s, want first refresh time %s", status.LastSuccess, testTime(0))
		}
		if strings.Contains(status.LastError, "sk-should-not-leak") {
			t.Fatalf("LastError = %q, want redacted/no raw upstream body", status.LastError)
		}
	})

	t.Run("Should reject all source failure without cache", func(t *testing.T) {
		t.Parallel()

		server := modelsDevServer(t, http.StatusInternalServerError, `secret sk-raw-value`)
		source := newModelsDevTestSource(t, server.URL, "1h", "1s", true)

		_, err := source.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(0)},
		)
		if err == nil {
			t.Fatal("ListModels(no cache) error = nil, want upstream error")
		}
		if _, ok := errors.AsType[*StaleFallbackError](err); ok {
			t.Fatalf("ListModels(no cache) error = %v, want no stale fallback", err)
		}
	})
}

func modelsDevServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, err := fmt.Fprint(w, body)
		if err != nil {
			t.Errorf("Fprint(response) error = %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newModelsDevTestSource(
	t *testing.T,
	endpoint string,
	ttl string,
	timeout string,
	enabled bool,
) *ModelsDevSource {
	t.Helper()

	source, err := NewModelsDevSource(nil, aghconfig.ModelsDevSourceConfig{
		Enabled:  &enabled,
		Endpoint: endpoint,
		TTL:      ttl,
		Timeout:  timeout,
	})
	if err != nil {
		t.Fatalf("NewModelsDevSource() error = %v", err)
	}
	return source
}

func requireSingleRow(t *testing.T, rows []ModelRow) ModelRow {
	t.Helper()

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1: %#v", len(rows), rows)
	}
	return rows[0]
}

func assertModelsDevCurrentRow(t *testing.T, row ModelRow) {
	t.Helper()

	if row.ProviderID != "codex" || row.ModelID != "gpt-5.4" {
		t.Fatalf("row identity = %s/%s, want codex/gpt-5.4", row.ProviderID, row.ModelID)
	}
	if row.DisplayName != "GPT-5.4" {
		t.Fatalf("DisplayName = %q, want GPT-5.4", row.DisplayName)
	}
	if row.SupportsReasoning == nil || !*row.SupportsReasoning {
		t.Fatalf("SupportsReasoning = %v, want true", row.SupportsReasoning)
	}
	if row.SupportsTools == nil || !*row.SupportsTools {
		t.Fatalf("SupportsTools = %v, want true", row.SupportsTools)
	}
	if row.ContextWindow == nil || *row.ContextWindow != 256000 {
		t.Fatalf("ContextWindow = %v, want 256000", row.ContextWindow)
	}
	if row.MaxInputTokens == nil || *row.MaxInputTokens != 200000 {
		t.Fatalf("MaxInputTokens = %v, want 200000", row.MaxInputTokens)
	}
	if row.MaxOutputTokens == nil || *row.MaxOutputTokens != 32000 {
		t.Fatalf("MaxOutputTokens = %v, want 32000", row.MaxOutputTokens)
	}
	if row.CostInputPerMillion == nil || *row.CostInputPerMillion != 1.25 {
		t.Fatalf("CostInputPerMillion = %v, want 1.25", row.CostInputPerMillion)
	}
	if row.CostOutputPerMillion == nil || *row.CostOutputPerMillion != 10.5 {
		t.Fatalf("CostOutputPerMillion = %v, want 10.5", row.CostOutputPerMillion)
	}
	if row.CostCacheReadPerMillion == nil || *row.CostCacheReadPerMillion != 0.125 {
		t.Fatalf("CostCacheReadPerMillion = %v, want 0.125", row.CostCacheReadPerMillion)
	}
	if row.CostCacheWritePerMillion == nil || *row.CostCacheWritePerMillion != 2.5 {
		t.Fatalf("CostCacheWritePerMillion = %v, want 2.5", row.CostCacheWritePerMillion)
	}
	if row.CostReasoningPerMillion == nil || *row.CostReasoningPerMillion != 12 {
		t.Fatalf("CostReasoningPerMillion = %v, want 12", row.CostReasoningPerMillion)
	}
	if row.ReleaseDate == nil || *row.ReleaseDate != "2026-03-05" {
		t.Fatalf("ReleaseDate = %v, want 2026-03-05", row.ReleaseDate)
	}
	if row.Deprecated == nil || !*row.Deprecated {
		t.Fatal("Deprecated = false, want true")
	}
}
