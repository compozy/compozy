package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestBootMarketplaceLifecycle(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"add", "disable", "remove"} {
		t.Run("Should restore persisted and live sources after a rejected "+operation, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "marketplace.json"), []byte(`{"plugins":[]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			ref := (&url.URL{Scheme: "file", Path: root}).String()
			feed := newMarketplaceFeedServer(t, "feed")
			home := testHomePaths(t)
			cfg := testConfig(t, home)
			cfg.Marketplace.Catalog.BaseURL = feed.URL
			cfg.Marketplace.PluginSources = []compozyconfig.MarketplacePluginSourceConfig{{Name: "team", Source: ref}}
			original := fmt.Sprintf("# preserved config comment\n[marketplace.catalog]\nbase_url = %q\n[[marketplace.plugin_sources]]\nname = \"team\"\nsource = %q\nenabled = true\n", feed.URL, ref)
			if err := os.WriteFile(home.ConfigFile, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			db := openDaemonTestGlobalDB(t)
			catalog, err := marketplace.NewSQLiteStore(db)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := newMarketplaceRuntime(t.Context(), catalog, nil, cfg.Marketplace, home, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := runtime.Shutdown(context.WithoutCancel(t.Context())); err != nil {
					t.Error(err)
				}
			})
			if _, err := runtime.Refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			before, err := runtime.Browse(t.Context(), "", 0, 100)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().ExecContext(t.Context(), `CREATE TRIGGER reject_source_configuration BEFORE UPDATE ON marketplace_catalog_config
				BEGIN SELECT RAISE(ABORT, 'injected source configuration failure'); END`); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "add":
				otherRoot := t.TempDir()
				if err := os.WriteFile(filepath.Join(otherRoot, "marketplace.json"), []byte(`{"plugins":[]}`), 0o600); err != nil {
					t.Fatal(err)
				}
				_, err = runtime.AddSource(t.Context(), (&url.URL{Scheme: "file", Path: otherRoot}).String(), "other", false)
			case "disable":
				_, err = runtime.UpdateSource(t.Context(), "team", false)
			case "remove":
				err = runtime.RemoveSource(t.Context(), "team")
			}
			if err == nil || !strings.Contains(err.Error(), "injected source configuration failure") {
				t.Fatalf("mutation error = %v, want actual SQL failure", err)
			}
			content, err := os.ReadFile(home.ConfigFile)
			if err != nil || string(content) != original {
				t.Fatalf("failed mutation changed overlay: %q, %v", content, err)
			}
			after, err := runtime.Browse(t.Context(), "", 0, 100)
			if err != nil || after.Revision != before.Revision || after.Total != before.Total {
				t.Fatalf("failed mutation changed live projection: before=%#v after=%#v err=%v", before, after, err)
			}
			if len(runtime.config.PluginSources) != 1 || runtime.config.PluginSources[0].Name != "team" {
				t.Fatalf("runtime source config changed: %#v", runtime.config.PluginSources)
			}
			state, err := runtime.sourceStatus(t.Context(), "team")
			if err != nil || !state.Enabled {
				t.Fatalf("source enabled state changed: %#v, %v", state, err)
			}
		})
	}

	// Invariant: feed presets are data, defaults overlay by origin, and the last valid set survives restart.
	// Owner: daemon source composition and derived preset cache; canonical lifecycle suite.
	t.Run(
		"Should refresh newly discovered presets and retain disabled choices after a rename and restart",
		func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(
				filepath.Join(root, "marketplace.json"),
				[]byte(`{"plugins":[]}`),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			ref := (&url.URL{Scheme: "file", Path: root}).String()
			var presetName atomic.Value
			presetName.Store("original")
			feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var entries any = []any{}
				if r.URL.Path == "/v3/marketplaces.json" {
					entries = []marketplace.Preset{
						{Name: presetName.Load().(string), Source: ref, Description: "Local preset", Default: "on"},
					}
				}
				if err := json.NewEncoder(w).Encode(map[string]any{
					"manifest_version": 3, "generated_at": "2026-09-13T00:00:00Z", "entries": entries,
				}); err != nil {
					t.Errorf("write feed: %v", err)
				}
			}))
			t.Cleanup(feed.Close)
			home := testHomePaths(t)
			cfg := testConfig(t, home)
			cfg.Marketplace.Catalog.BaseURL = feed.URL
			cfg.Marketplace.Catalog.Timeout = "1s"
			repository := openDaemonTestGlobalDB(t)
			store, err := marketplace.NewSQLiteStore(repository)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := newMarketplaceRuntime(t.Context(), store, nil, cfg.Marketplace, home, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := runtime.Shutdown(testutil.Context(t)); err != nil {
					t.Error(err)
				}
			})
			report, err := runtime.Refresh(t.Context())
			if err != nil || len(report.Outcomes) != 2 || report.Outcomes[1].Source != "original" ||
				report.Outcomes[1].Outcome != marketplace.RefreshOutcomeSucceeded {
				t.Fatalf("new preset was not refreshed: %+v, %v", report, err)
			}
			cfg.Marketplace.PluginSources = []compozyconfig.MarketplacePluginSourceConfig{
				{Name: "original", Source: ref, Enabled: new(false)},
			}
			if err := runtime.ReconcileConfig(t.Context(), &cfg); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(
				filepath.Join(root, "marketplace.json"),
				filepath.Join(root, "disabled.json"),
			); err != nil {
				t.Fatal(err)
			}
			presetName.Store("renamed")
			report, err = runtime.Refresh(t.Context())
			if err != nil || len(report.Outcomes) != 1 {
				t.Fatalf("disabled preset was contacted: %+v, %v", report, err)
			}
			if err := runtime.Shutdown(t.Context()); err != nil {
				t.Fatal(err)
			}
			feed.Close()
			reopened, err := newMarketplaceRuntime(t.Context(), store, nil, cfg.Marketplace, home, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reopened.Shutdown(testutil.Context(t)); err != nil {
					t.Error(err)
				}
			})
			states, err := reopened.Status(t.Context())
			if err != nil || len(states) != 2 || states[1].Source != "renamed" || states[1].Enabled ||
				states[1].SourceRef != ref || states[1].Kind != marketplace.SourceKindPreset {
				t.Fatalf("preset choice after offline restart=%+v err=%v", states, err)
			}
		},
	)

	// Invariant: daemon config composes the real plugin loader/cache, excludes disabled sources,
	// and retains last-good plugin rows after acquisition failure. Owner: marketplace runtime.
	t.Run(
		"Should project configured plugins through the install loader and retain cached bytes while degraded",
		func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.CopyFS(
				filepath.Join(root, "plugin"),
				os.DirFS(filepath.Join("..", "extension", "testdata", "client-plugins", "open-design")),
			); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(root, "marketplace.json"),
				[]byte(`{"plugins":[{"name":"design","source":"./plugin"}]}`),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			var disabledRequests atomic.Int64
			disabled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				disabledRequests.Add(1)
				http.NotFound(w, r)
			}))
			t.Cleanup(disabled.Close)
			feed := newMarketplaceFeedServer(t, "feed")
			home := testHomePaths(t)
			cfg := testConfig(t, home)
			cfg.Marketplace.Catalog.BaseURL = feed.URL
			cfg.Marketplace.Catalog.Timeout = "1s"
			cfg.Marketplace.PluginSources = []compozyconfig.MarketplacePluginSourceConfig{
				{Name: "team", Source: (&url.URL{Scheme: "file", Path: root}).String()},
				{Name: "disabled", Source: "github:team/disabled", Enabled: new(false)},
			}
			repository := openDaemonTestGlobalDB(t)
			store, err := marketplace.NewSQLiteStore(repository)
			if err != nil {
				t.Fatal(err)
			}
			installedBytes := bytes.Repeat([]byte("p"), 32<<10)
			unusedBytes := bytes.Repeat([]byte("u"), 32<<10)
			installedDigest, unusedDigest := fmt.Sprintf(
				"%x",
				sha256.Sum256(installedBytes),
			), fmt.Sprintf(
				"%x",
				sha256.Sum256(unusedBytes),
			)
			runtime, err := newMarketplaceRuntime(t.Context(), store, nil, cfg.Marketplace, home, time.Now,
				marketplace.WithInstalledPackages(func(context.Context) ([]marketplace.InstalledPackage, error) {
					return []marketplace.InstalledPackage{{DigestSHA256: installedDigest}}, nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			runtime.resolver.Cache.MaxBytes = 64 << 10
			for digest, data := range map[string][]byte{installedDigest: installedBytes, unusedDigest: unusedBytes} {
				if err := runtime.resolver.Cache.Put(t.Context(), digest, bytes.NewReader(data)); err != nil {
					t.Fatal(err)
				}
			}
			runtime.resolver.Sources.GitHubOptions = []pluginsource.GitHubOption{
				pluginsource.WithGitHubBaseURL(disabled.URL),
			}
			t.Cleanup(func() {
				if err := runtime.Shutdown(testutil.Context(t)); err != nil {
					t.Error(err)
				}
			})
			report, err := runtime.Refresh(t.Context())
			if err != nil || len(report.Outcomes) != 2 || disabledRequests.Load() != 0 {
				t.Fatalf("refresh=%+v err=%v disabled requests=%d", report, err, disabledRequests.Load())
			}
			kept, err := runtime.resolver.Cache.Open(t.Context(), installedDigest)
			if err != nil {
				t.Fatal(err)
			}
			if err := kept.Close(); err != nil {
				t.Fatal(err)
			}
			if reader, err := runtime.resolver.Cache.Open(
				t.Context(),
				unusedDigest,
			); reader != nil ||
				!errors.Is(err, pluginsource.ErrPackageUnavailable) {
				t.Fatalf("unreferenced cache blob survived refresh: %v, %v", reader, err)
			}
			page, err := runtime.Browse(t.Context(), "", 0, 20)
			if err != nil || page.Total != 2 || len(page.Sources) != 3 ||
				page.Entries[0].SourceName != marketplace.CompozyCatalogSource {
				t.Fatalf("merged listing=%+v err=%v", page, err)
			}
			entry := page.Entries[1]
			projected, err := marketplace.ProjectEntry(entry)
			if err != nil || !entry.Installable || entry.SourceName != "team" ||
				projected.Extension.Contents.MCPServers != 1 ||
				projected.Extension.Acquisition == nil ||
				projected.SourceRef != cfg.Marketplace.PluginSources[0].Source {
				t.Fatalf("projected plugin=%+v entry=%+v err=%v", projected, entry, err)
			}
			blob := filepath.Join(runtime.resolver.Cache.Root, entry.DigestSHA256+".tar")
			for _, corrupt := range []bool{false, true} {
				if corrupt {
					if err := os.WriteFile(blob, []byte("corrupt cached package"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Remove(blob); err != nil {
					t.Fatal(err)
				}
				unavailable, err := runtime.Browse(t.Context(), "", 0, 20)
				if err != nil || unavailable.Total != 2 || unavailable.Entries[1].Installable ||
					unavailable.Entries[1].InstallBlocker != "package_unavailable" {
					t.Fatalf("cache availability = %+v, %v", unavailable, err)
				}
				detail, err := runtime.Detail(t.Context(), "team", "design")
				if err != nil || detail.InstallBlocker != "package_unavailable" {
					t.Fatalf("unavailable detail = %+v, %v", detail, err)
				}
				repaired, err := runtime.resolver.Acquire(
					t.Context(),
					*projected.Extension.Acquisition,
					entry.DigestSHA256,
				)
				if err != nil {
					t.Fatal(err)
				}
				if err := repaired.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(
				filepath.Join(root, "marketplace.json"),
				filepath.Join(root, "unreachable.json"),
			); err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.Refresh(t.Context(), "team"); err == nil {
				t.Fatal("unreachable source refresh succeeded")
			}
			retained, err := runtime.Detail(t.Context(), "team", "design")
			if err != nil || retained.DigestSHA256 != entry.DigestSHA256 {
				t.Fatalf("last-good entry=%+v err=%v", retained, err)
			}
			reader, err := runtime.resolver.Acquire(t.Context(), *projected.Extension.Acquisition, entry.DigestSHA256)
			if err != nil {
				t.Fatal(err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			cfg.Marketplace.PluginSources[0].Enabled = new(false)
			if err := runtime.ReconcileConfig(t.Context(), &cfg); err != nil {
				t.Fatal(err)
			}
			page, err = runtime.Browse(t.Context(), "", 0, 20)
			if err != nil || page.Total != 1 || page.Sources[1].Enabled || disabledRequests.Load() != 0 {
				t.Fatalf(
					"disabled source visible or fetched: %+v err=%v requests=%d",
					page,
					err,
					disabledRequests.Load(),
				)
			}
		},
	)

	t.Run("Should browse a checkout catalog through a file source", func(t *testing.T) {
		t.Parallel()

		catalogDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(catalogDir, "v3"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(catalogDir, "v3", "marketplaces.json"),
			[]byte(`{"manifest_version":3,"generated_at":"2026-07-13T12:00:00Z","entries":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		catalogPath := filepath.Join(catalogDir, "v3", "extensions.json")
		catalogDocument := `{"manifest_version":3,"generated_at":"2026-07-13T12:00:00Z","entries":[{` +
			`"entry_id":"checkout","name":"checkout","description":"Local checkout fixture",` +
			`"tier":"official","version":"1.0.0","install_slug":"compozy/checkout","artifact_url":"https://example.test/checkout.tgz","digest_sha256":"` + strings.Repeat("a", 64) + `"}]}`
		if err := os.WriteFile(catalogPath, []byte(catalogDocument), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", catalogPath, err)
		}

		registry := openDaemonTestGlobalDB(t)
		marketplaceStore, err := marketplace.NewSQLiteStore(registry)
		if err != nil {
			t.Fatalf("NewSQLiteStore() error = %v", err)
		}
		seedRemoteMarketplaceProjection(t, marketplaceStore, time.Now().UTC())
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Marketplace.Catalog.BaseURL = (&url.URL{Scheme: "file", Path: catalogDir}).String()
		cfg.Marketplace.Catalog.TTL = "1h"
		cfg.Marketplace.Catalog.Timeout = "1s"
		runtime, err := newMarketplaceRuntime(t.Context(), marketplaceStore, nil, cfg.Marketplace, homePaths, time.Now)
		if err != nil {
			t.Fatalf("newMarketplaceRuntime() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		if _, err := runtime.Refresh(testutil.Context(t)); err != nil {
			t.Fatalf("Browse(checkout) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, runtime, "checkout")

		catalogDocument = `{"manifest_version":3,"generated_at":"2026-07-13T12:01:00Z","entries":[{` +
			`"entry_id":"edited-checkout","name":"edited-checkout","description":"Edited fixture",` +
			`"tier":"official","version":"1.0.0","install_slug":"compozy/edited-checkout","artifact_url":"https://example.test/edited.tgz","digest_sha256":"` + strings.Repeat("a", 64) + `"}]}`
		if err := os.WriteFile(catalogPath, []byte(catalogDocument), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", catalogPath, err)
		}
		if _, err := runtime.Refresh(testutil.Context(t)); err != nil {
			t.Fatalf("Browse(edited checkout) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, runtime, "edited-checkout")
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

		if _, err := state.marketplace.Refresh(testutil.Context(t)); err != nil {
			t.Fatalf("Refresh(first) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, state.marketplace, "first")
		assertMarketplaceRefreshEvent(t, registry)
		if err := state.marketplaceNotifier.NotifyInstall(testutil.Context(t), marketplace.InstallOutcome{
			Origin:     &marketplace.Origin{SourceRef: marketplace.CompozyCatalogRef, EntryID: "github"},
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
		if _, err := state.marketplace.Refresh(testutil.Context(t)); err != nil {
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
		var missingContext context.Context
		if err := notifier.NotifyCatalogRefresh(
			missingContext,
			marketplace.RefreshOutcome{Source: marketplace.CompozyCatalogSource,
				Outcome: marketplace.RefreshOutcomeSucceeded,
			},
		); err == nil ||
			!strings.Contains(err.Error(), "context is required") {
			t.Fatalf("NotifyCatalogRefresh(nil context) error = %v", err)
		}
		parent, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		started := time.Now()
		err := notifier.NotifyCatalogRefresh(
			parent,
			marketplace.RefreshOutcome{Source: marketplace.CompozyCatalogSource,
				Outcome: marketplace.RefreshOutcomeSucceeded,
			},
		)
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
			if request.URL.Path != "/v3/extensions.json" {
				if _, err := writer.Write([]byte(
					`{"manifest_version":3,"generated_at":"2026-07-13T12:00:00Z","entries":[]}`,
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
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Marketplace.Catalog.BaseURL = oldServer.URL
		cfg.Marketplace.Catalog.TTL = "1h"
		cfg.Marketplace.Catalog.Timeout = "1m"
		runtime, err := newMarketplaceRuntime(t.Context(), marketplaceStore, nil, cfg.Marketplace, homePaths, time.Now)
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
			_, refreshErr := runtime.Refresh(t.Context())
			oldRefresh <- refreshErr
		}()
		<-requestStarted
		next := cfg
		next.Marketplace.Catalog.BaseURL = newServer.URL
		if err := runtime.ReconcileConfig(testutil.Context(t), &next); err != nil {
			t.Fatalf("ReconcileConfig() error = %v", err)
		}
		<-requestCanceled
		if err := <-oldRefresh; !errors.Is(err, store.ErrMarketplaceCatalogGenerationStale) {
			t.Fatalf("old Refresh() error = %v, want obsolete source generation", err)
		}
		if _, err := runtime.Refresh(testutil.Context(t)); err != nil {
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

func newMarketplaceFeedServer(t *testing.T, entryID string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		body := `{"manifest_version":3,"generated_at":"2026-07-13T12:00:00Z","entries":[]}`
		if request.URL.Path == "/v3/extensions.json" {
			body = `{"manifest_version":3,"generated_at":"2026-07-13T12:00:00Z","entries":[{` +
				`"entry_id":"` + entryID + `","name":"` + entryID + `","description":"Daemon fixture",` +
				`"tier":"official","version":"1.0.0","install_slug":"compozy/` + entryID + `","artifact_url":"https://example.test/package.tgz","digest_sha256":"` + strings.Repeat("a", 64) + `"}]}`
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
	result, err := runtime.Browse(testutil.Context(t), "", 0, 10)
	if err != nil {
		t.Fatalf("Browse() error = %v", err)
	}
	if got, want := len(result.Entries), 1; got != want || result.Entries[0].EntryID != wantEntryID {
		t.Fatalf("Browse() = %#v, want only %q", result.Entries, wantEntryID)
	}
}

func seedRemoteMarketplaceProjection(t *testing.T, catalogStore marketplace.Store, fetchedAt time.Time) {
	t.Helper()
	generation, err := catalogStore.ConfigureSources(
		t.Context(),
		[]marketplace.ResolvedSource{
			{
				Name:    marketplace.CompozyCatalogSource,
				Ref:     marketplace.CompozyCatalogRef,
				Kind:    marketplace.SourceKindFeed,
				Enabled: true,
			},
		},
		"fixture",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogStore.ReplaceSource(
		t.Context(),
		marketplace.CompozyCatalogSource,
		generation.Generation,
		&marketplace.Document{
			ManifestVersion: marketplace.ManifestVersion,
			GeneratedAt:     fetchedAt.Add(-time.Minute),
			FetchedAt:       fetchedAt,
			Entries: []marketplace.Entry{{
				EntryID:      "remote",
				Version:      "1.0.0",
				DigestSHA256: strings.Repeat("a", 64),
				Name:         "remote",
				Description:  "Fresh remote projection",
				InstallSlug:  "compozy/remote",
				Payload:      []byte(`{"entry_id":"remote","name":"remote"}`),
				FetchedAt:    fetchedAt,
			}},
		},
	); err != nil {
		t.Fatalf("ReplaceSource(remote projection) error = %v", err)
	}
}

func assertMarketplaceRefreshEvent(t *testing.T, registry *globaldb.GlobalDB) {
	t.Helper()
	summaries, err := registry.ListEventSummaries(
		testutil.Context(t),
		store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			Type:  eventspkg.MarketplaceCatalogRefresh,
			Limit: 10,
		},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	if got, want := len(summaries), 1; got != want {
		t.Fatalf("ListEventSummaries() count = %d, want %d", got, want)
	}
	var outcome marketplace.RefreshOutcome
	if err := json.Unmarshal(summaries[0].Content, &outcome); err != nil {
		t.Fatal(err)
	}
	if summaries[0].Outcome != string(eventspkg.OutcomeSuccess) ||
		outcome.Source != marketplace.CompozyCatalogSource ||
		summaries[0].Timestamp.IsZero() || time.Since(summaries[0].Timestamp) > time.Minute {
		t.Fatalf("refresh summary = %#v, want recent success", summaries[0])
	}
}

func assertMarketplaceInstallEvent(t *testing.T, registry *globaldb.GlobalDB) {
	t.Helper()
	summaries, err := registry.ListEventSummaries(
		testutil.Context(t),
		store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			Type:  eventspkg.MarketplaceInstall,
			Limit: 10,
		},
	)
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
		outcome.Origin == nil || outcome.Origin.SourceRef != marketplace.CompozyCatalogRef || outcome.Origin.EntryID != "github" ||
		outcome.EntryID != "github" ||
		outcome.Outcome != marketplace.InstallOutcomeSucceeded ||
		outcome.PolicyGate != marketplace.InstallPolicyGatePassed {
		t.Fatalf("marketplace.install summary = %#v, outcome = %#v", summaries[0], outcome)
	}
}

// Invariant: canonical install observations carry persisted origin/ref and no distribution credentials.
// Owner: daemon marketplace event persistence; canonical suite: marketplace_test.go (UT-069).
func TestMarketplaceExtensionInstallEvent(t *testing.T) {
	t.Parallel()
	t.Run("Should persist one classified install observation without inferring sideload origin", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		registry := openDaemonTestGlobalDB(t)
		service := &daemonExtensionService{eventWriter: registry, logger: discardLogger()}
		item := contract.ExtensionPayload{Name: "custom-name", Origin: &contract.MarketplaceOriginPayload{
			Source: marketplace.CompozyCatalogSource, SourceRef: marketplace.CompozyCatalogRef, EntryID: "published-id",
		}, Provenance: &contract.ExtensionProvenancePayload{ResolvedRef: strings.Repeat("a", 40), SourceURL: "https://user:password@example.test/private"}}
		if err := service.notifyMarketplaceExtensionInstalled(ctx, &item); err != nil {
			t.Fatal(err)
		}
		if err := service.notifyMarketplaceExtensionInstalled(
			ctx, &contract.ExtensionPayload{
				Name:       "local",
				Provenance: &contract.ExtensionProvenancePayload{CatalogEntryID: "published-id"},
			},
		); err != nil {
			t.Fatal(err)
		}
		rows, err := registry.ListEventSummaries(ctx, store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.MarketplaceInstall, Limit: 10,
		})
		if err != nil || len(rows) != 1 {
			t.Fatalf("events=%#v error=%v", rows, err)
		}
		var outcome marketplace.InstallOutcome
		if err := json.Unmarshal(rows[0].Content, &outcome); err != nil {
			t.Fatal(err)
		}
		want := marketplace.Origin{SourceRef: marketplace.CompozyCatalogRef, EntryID: "published-id"}
		if outcome.Origin == nil || *outcome.Origin != want || outcome.ResolvedRef != item.Provenance.ResolvedRef ||
			outcome.Outcome != marketplace.InstallOutcomeSucceeded {
			t.Fatalf("install observation=%#v", outcome)
		}
		if strings.Contains(string(rows[0].Content), "password") ||
			strings.Contains(string(rows[0].Content), "SourceURL") {
			t.Fatal("install event leaked acquisition credentials")
		}
	})
}
