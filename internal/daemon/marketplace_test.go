package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/marketplace"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestBootMarketplaceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should omit an unavailable marketplace from settings dependencies", func(t *testing.T) {
		t.Parallel()

		if dependency := settingsMarketplaceCatalogDependency(nil); dependency != nil {
			t.Fatalf("settings marketplace dependency = %#v, want nil", dependency)
		}
	})

	t.Run("Should boot, emit refresh events, reconcile config live, and shut down cleanly", func(t *testing.T) {
		t.Parallel()

		firstServer := newMarketplaceFeedServer(t, "first")
		secondServer := newMarketplaceFeedServer(t, "second")
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Marketplace.Catalog.BaseURL = firstServer.URL
		cfg.Marketplace.Catalog.TTL = "1h"
		cfg.Marketplace.Catalog.Timeout = "1s"
		registry := openDaemonTestGlobalDB(t)

		daemonInstance := newTestDaemon(t, homePaths, &cfg)
		state := &bootState{cfg: cfg, logger: discardLogger(), registry: registry}
		if err := daemonInstance.bootMarketplace(testutil.Context(t), state, &bootCleanup{}); err != nil {
			t.Fatalf("bootMarketplace() error = %v", err)
		}
		if state.marketplace == nil {
			t.Fatal("bootMarketplace() runtime = nil")
		}
		if state.marketplaceNotifier == nil {
			t.Fatal("bootMarketplace() notifier = nil")
		}

		if _, err := state.marketplace.Refresh(testutil.Context(t), marketplace.KindSkill); err != nil {
			t.Fatalf("Refresh(first) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, state.marketplace, "first")
		assertMarketplaceRefreshEvent(t, registry)
		if err := state.marketplaceNotifier.NotifyInstall(testutil.Context(t), marketplace.InstallOutcome{
			Kind:       marketplace.KindMCP,
			EntryID:    "github",
			Outcome:    marketplace.InstallOutcomeSucceeded,
			PolicyGate: marketplace.InstallPolicyGatePassed,
		}); err != nil {
			t.Fatalf("NotifyInstall() error = %v", err)
		}
		assertMarketplaceInstallEvent(t, registry)

		next := cfg
		next.Marketplace.Catalog.BaseURL = secondServer.URL
		if err := state.marketplace.ReconcileConfig(testutil.Context(t), &next); err != nil {
			t.Fatalf("ReconcileConfig() error = %v", err)
		}
		if _, err := state.marketplace.Refresh(testutil.Context(t), marketplace.KindSkill); err != nil {
			t.Fatalf("Refresh(second) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, state.marketplace, "second")

		if err := state.marketplace.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(first) error = %v", err)
		}
		if err := state.marketplace.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(second) error = %v", err)
		}
	})

	t.Run("Should bound detached refresh event writes", func(t *testing.T) {
		t.Parallel()

		writeErrors := make(chan error, 1)
		notifier := &daemonMarketplaceNotifier{
			writer:       blockingMarketplaceEventWriter{writeErrors: writeErrors},
			writeTimeout: 20 * time.Millisecond,
		}
		parent, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		started := time.Now()
		err := notifier.NotifyCatalogRefresh(parent, marketplace.RefreshOutcome{
			Kind:    marketplace.KindSkill,
			Outcome: marketplace.RefreshOutcomeSucceeded,
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("NotifyCatalogRefresh() error = %v, want context deadline exceeded", err)
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("NotifyCatalogRefresh() elapsed = %s, want bounded event write", elapsed)
		}
		select {
		case err := <-writeErrors:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("WriteEventSummary() context error = %v, want deadline exceeded", err)
			}
		default:
			t.Fatal("WriteEventSummary() did not observe its bounded deadline")
		}
	})

	t.Run("Should retire a blocked catalog generation before publishing its replacement", func(t *testing.T) {
		t.Parallel()

		requestStarted := make(chan struct{})
		requestCanceled := make(chan struct{})
		oldServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/skills.json" {
				if _, err := writer.Write([]byte(
					`{"manifest_version":1,"generated_at":"2026-07-13T12:00:00Z","entries":[]}`,
				)); err != nil {
					t.Errorf("write empty feed: %v", err)
				}
				return
			}
			close(requestStarted)
			<-request.Context().Done()
			close(requestCanceled)
		}))
		t.Cleanup(oldServer.Close)
		newServer := newMarketplaceFeedServer(t, "replacement")
		registry := openDaemonTestGlobalDB(t)
		marketplaceStore, err := marketplace.NewSQLiteStore(registry)
		if err != nil {
			t.Fatalf("NewSQLiteStore() error = %v", err)
		}
		cfg := testConfig(t, testHomePaths(t))
		cfg.Marketplace.Catalog.BaseURL = oldServer.URL
		cfg.Marketplace.Catalog.TTL = "1h"
		cfg.Marketplace.Catalog.Timeout = "1m"
		runtime, err := newMarketplaceRuntime(marketplaceStore, nil, cfg.Marketplace.Catalog, time.Now)
		if err != nil {
			t.Fatalf("newMarketplaceRuntime() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		oldRefresh := make(chan error, 1)
		go func() {
			_, refreshErr := runtime.Refresh(t.Context(), marketplace.KindSkill)
			oldRefresh <- refreshErr
		}()
		<-requestStarted
		next := cfg
		next.Marketplace.Catalog.BaseURL = newServer.URL
		if err := runtime.ReconcileConfig(testutil.Context(t), &next); err != nil {
			t.Fatalf("ReconcileConfig() error = %v", err)
		}
		<-requestCanceled
		if err := <-oldRefresh; !errors.Is(err, marketplace.ErrServiceClosed) {
			t.Fatalf("old Refresh() error = %v, want ErrServiceClosed", err)
		}
		if _, err := runtime.Refresh(testutil.Context(t), marketplace.KindSkill); err != nil {
			t.Fatalf("Refresh(replacement) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, runtime, "replacement")
	})
}

type blockingMarketplaceEventWriter struct {
	writeErrors chan<- error
}

func (w blockingMarketplaceEventWriter) WriteEventSummary(ctx context.Context, _ store.EventSummary) error {
	<-ctx.Done()
	w.writeErrors <- ctx.Err()
	return ctx.Err()
}

func newMarketplaceFeedServer(t *testing.T, skillID string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		body := `{"manifest_version":1,"generated_at":"2026-07-13T12:00:00Z","entries":[]}`
		if request.URL.Path == "/skills.json" {
			body = `{"manifest_version":1,"generated_at":"2026-07-13T12:00:00Z","entries":[{` +
				`"entry_id":"` + skillID + `","name":"` + skillID + `","description":"Daemon fixture",` +
				`"install_slug":"compozy/` + skillID + `"}]}`
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			t.Errorf("write feed response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func assertMarketplaceRuntimeEntry(t *testing.T, runtime *marketplaceRuntime, wantEntryID string) {
	t.Helper()
	result, err := runtime.Browse(testutil.Context(t), marketplace.KindSkill, "", 0, 10)
	if err != nil {
		t.Fatalf("Browse() error = %v", err)
	}
	if got, want := len(result.Entries), 1; got != want || result.Entries[0].EntryID != wantEntryID {
		t.Fatalf("Browse() = %#v, want only %q", result.Entries, wantEntryID)
	}
}

func assertMarketplaceRefreshEvent(t *testing.T, registry *globaldb.GlobalDB) {
	t.Helper()
	summaries, err := registry.ListEventSummaries(testutil.Context(t), store.EventSummaryQuery{
		Type:  eventspkg.MarketplaceCatalogRefresh,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	if got, want := len(summaries), 1; got != want {
		t.Fatalf("ListEventSummaries() count = %d, want %d", got, want)
	}
	if summaries[0].Outcome != string(eventspkg.OutcomeSuccess) ||
		summaries[0].Timestamp.IsZero() || time.Since(summaries[0].Timestamp) > time.Minute {
		t.Fatalf("refresh summary = %#v, want recent success", summaries[0])
	}
}

func assertMarketplaceInstallEvent(t *testing.T, registry *globaldb.GlobalDB) {
	t.Helper()
	summaries, err := registry.ListEventSummaries(testutil.Context(t), store.EventSummaryQuery{
		Type:  eventspkg.MarketplaceInstall,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(marketplace.install) error = %v", err)
	}
	if got, want := len(summaries), 1; got != want {
		t.Fatalf("marketplace.install summary count = %d, want %d", got, want)
	}
	var outcome marketplace.InstallOutcome
	if err := json.Unmarshal(summaries[0].Content, &outcome); err != nil {
		t.Fatalf("json.Unmarshal(marketplace.install) error = %v", err)
	}
	if summaries[0].Outcome != string(eventspkg.OutcomeSuccess) ||
		outcome.Kind != marketplace.KindMCP ||
		outcome.EntryID != "github" ||
		outcome.Outcome != marketplace.InstallOutcomeSucceeded ||
		outcome.PolicyGate != marketplace.InstallPolicyGatePassed {
		t.Fatalf("marketplace.install summary = %#v, outcome = %#v", summaries[0], outcome)
	}
}
