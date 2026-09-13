package marketplace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"time"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"

	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestNewCatalogServiceValidation(t *testing.T) {
	t.Parallel()

	store := openMarketplaceTestStore(t)
	validSource := &recordingSource{fetch: func(context.Context) (*Document, error) {
		return testDocument(time.Now().UTC(), testEntry("extension", "Skill", "Valid source")), nil
	}}
	tests := []struct {
		name    string
		store   Store
		source  Source
		ttl     time.Duration
		timeout time.Duration
		wantErr string
	}{
		{
			name:    "Should reject a missing store",
			source:  validSource,
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "store is required",
		},
		{
			name:    "Should reject a non-positive TTL",
			store:   store,
			source:  validSource,
			timeout: time.Minute,
			wantErr: "TTL must be positive",
		},
		{
			name:    "Should reject a non-positive refresh timeout",
			store:   store,
			source:  validSource,
			ttl:     time.Hour,
			wantErr: "refresh timeout must be positive",
		},
		{
			name:    "Should reject a missing source",
			store:   store,
			source:  nil,
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "source is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewService(t.Context(), tc.store, marketplaceTestBindings(t, tc.source), tc.ttl, tc.timeout)
			if err == nil {
				t.Fatal("NewService() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewService() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestCatalogServiceDetailAndStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	newService := func(t *testing.T) *CatalogService {
		t.Helper()
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(now, testEntry("extension", "Extension", "Detail fixture")), nil
		}}
		service, err := NewService(
			t.Context(), openMarketplaceTestStore(t),
			marketplaceTestBindings(t, source),
			time.Hour,
			time.Minute,
			WithNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		return service
	}

	t.Run("Should return source status and resolve detail by immutable id", func(t *testing.T) {
		t.Parallel()

		service := newService(t)
		ctx := testutil.Context(t)
		states, err := service.Status(ctx)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got, want := len(states), 1; got != want || states[0].Source != CompozyCatalogSource {
			t.Fatalf("Status() = %#v, want the curated source before its first refresh", states)
		}
		if _, err := service.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		entry, err := service.Detail(ctx, CompozyCatalogSource, "extension")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if entry.EntryID != "extension" {
			t.Fatalf("Detail().EntryID = %q, want extension", entry.EntryID)
		}
		if _, err := service.Detail(ctx, CompozyCatalogSource, "missing"); !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("Detail(missing) error = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("Should resolve extension install by slug and fail for a missing slug", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		extensionSource := &recordingSource{fetch: func(context.Context) (*Document, error) {
			entry := testEntry("telemetry", "Telemetry", "Curated extension")
			entry.Version = "2.1.0"
			return testDocument(now, entry), nil
		}}
		extensionService := newMarketplaceTestService(t, openMarketplaceTestStore(t), extensionSource, now, nil)
		if _, err := extensionService.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		resolved, err := extensionService.ResolveExtensionInstall(ctx, "compozy/telemetry", "2.1.0")
		if err != nil {
			t.Fatalf("ResolveExtensionInstall() error = %v", err)
		}
		if resolved.EntryID != "telemetry" {
			t.Fatalf("ResolveExtensionInstall().EntryID = %q, want telemetry", resolved.EntryID)
		}
		if _, err := extensionService.ResolveExtensionInstall(ctx, "compozy/missing", ""); !errors.Is(
			err,
			ErrEntryNotFound,
		) {
			t.Fatalf("ResolveExtensionInstall(missing) error = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("Should resolve a missing installation without contacting an unavailable source", func(t *testing.T) {
		t.Parallel()
		source := &recordingSource{
			fetch: func(context.Context) (*Document, error) { return nil, errors.New("unavailable") },
		}
		service := newMarketplaceTestService(t, openMarketplaceTestStore(t), source, now, nil)
		if _, err := service.ResolveExtensionInstall(
			t.Context(),
			"compozy/missing",
			"1.0.0",
		); !errors.Is(
			err,
			ErrEntryNotFound,
		) {
			t.Fatalf("missing install = %v", err)
		}
		if source.calls.Load() != 0 {
			t.Fatal("install resolution contacted the source")
		}
	})

	t.Run("Should reject nil contexts and service receivers with specific diagnostics", func(t *testing.T) {
		t.Parallel()

		service := newService(t)
		ctx := testutil.Context(t)
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err := service.Browse(nil, "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "service context is required") {
			t.Fatalf("Browse(nil context) error = %v, want service-context validation", err)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err = service.Detail(nil, CompozyCatalogSource, "extension")
		if err == nil || !strings.Contains(err.Error(), "service context is required") {
			t.Fatalf("Detail(nil context) error = %v, want service-context validation", err)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err = service.Refresh(nil)
		if err == nil || !strings.Contains(err.Error(), "service context is required") {
			t.Fatalf("Refresh(nil context) error = %v, want refresh-context validation", err)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err = service.Status(nil)
		if err == nil || !strings.Contains(err.Error(), "service context is required") {
			t.Fatalf("Status(nil context) error = %v, want status-context validation", err)
		}
		var unavailable *CatalogService
		_, err = unavailable.Browse(ctx, "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "service is required") {
			t.Fatalf("Browse(nil service) error = %v, want service validation", err)
		}
		_, err = unavailable.Refresh(ctx)
		if err == nil || !strings.Contains(err.Error(), "service is required") {
			t.Fatalf("Refresh(nil service) error = %v, want source validation", err)
		}
	})
}

func TestCatalogServiceRefreshErrorClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fetchErr  error
		wantClass string
	}{
		{name: "Should classify cancellation", fetchErr: context.Canceled, wantClass: errorClassCanceled},
		{
			name:      "Should classify plugin rate limits",
			fetchErr:  &pluginsource.SourceError{Reason: "rate_limited"},
			wantClass: "rate_limited",
		},
		{
			name:      "Should classify unreachable plugin sources",
			fetchErr:  pluginsource.ErrSourceUnreachable,
			wantClass: "source_unreachable",
		},
		{
			name:      "Should classify oversized plugin documents",
			fetchErr:  pluginsource.ErrDocumentTooLarge,
			wantClass: "marketplace_document_too_large",
		},
		{
			name:      "Should classify invalid plugin documents",
			fetchErr:  pluginsource.ErrNotMarketplace,
			wantClass: "marketplace_not_a_marketplace",
		},
		{
			name:      "Should classify the plugin refresh budget",
			fetchErr:  errors.Join(ErrRefreshBudgetExhausted, context.DeadlineExceeded),
			wantClass: budgetExhausted,
		},
		{name: "Should classify timeout", fetchErr: context.DeadlineExceeded, wantClass: "timeout"},
		{name: "Should classify oversized payload", fetchErr: ErrResponseTooLarge, wantClass: "payload_too_large"},
		{
			name:      "Should classify unsupported manifest",
			fetchErr:  &UnsupportedManifestVersionError{Version: 2},
			wantClass: "manifest_version",
		},
		{name: "Should classify HTTP status", fetchErr: &httpStatusError{status: 503}, wantClass: "http_status"},
		{
			name:      "Should classify JSON decode",
			fetchErr:  fmt.Errorf("fixture decode failure: %w", ErrCatalogDecode),
			wantClass: "decode",
		},
		{
			name:      "Should classify validation",
			fetchErr:  fmt.Errorf("fixture validation failure: %w", ErrCatalogValidation),
			wantClass: "validation",
		},
		{
			name:      "Should classify URL transport failure",
			fetchErr:  &url.Error{Op: "Get", URL: "https://catalog.example.test", Err: errors.New("offline")},
			wantClass: errorClassNetwork,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := openMarketplaceTestStore(t)
			source := &recordingSource{fetch: func(context.Context) (*Document, error) {
				return nil, tc.fetchErr
			}}
			service := newMarketplaceTestService(t, store, source, time.Now().UTC(), nil)
			report, err := service.Refresh(testutil.Context(t))
			if err == nil {
				t.Fatal("Refresh() error = nil, want source failure")
			}
			if got := report.Outcomes[0].ErrorClass; got != tc.wantClass {
				t.Fatalf("Refresh().ErrorClass = %q, want %q", got, tc.wantClass)
			}
		})
	}
}

func TestCatalogServiceRefreshLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should serve a fresh projection without fetching", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		fetchedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)

		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return nil, errors.New("unexpected fetch")
		}}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(30*time.Minute), nil)
		if err := store.ReplaceSource(
			ctx,
			CompozyCatalogSource,
			testSourceGeneration(t, store, CompozyCatalogSource),
			testDocument(
				fetchedAt,
				testEntry("fresh", "Fresh skill", "Already projected"),
			),
		); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}

		result, err := service.Browse(ctx, "", 0, 10)
		if err != nil {
			t.Fatalf("Browse() error = %v", err)
		}
		if got, want := len(result.Entries), 1; got != want {
			t.Fatalf("Browse() entries = %d, want %d", got, want)
		}
		if got := source.calls.Load(); got != 0 {
			t.Fatalf("source calls = %d, want 0 while fresh", got)
		}
	})

	t.Run("Should replace a fresh remote projection through an explicit checkout refresh", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		fetchedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
		if err := store.ReplaceSource(
			ctx,
			CompozyCatalogSource,
			testSourceGeneration(t, store, CompozyCatalogSource),
			testDocument(
				fetchedAt,
				testEntry("remote", "Remote skill", "Fresh remote projection"),
			),
		); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}
		directory := t.TempDir()
		if err := os.MkdirAll(filepath.Join(directory, "v3"), 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, "v3", "extensions.json")
		if err := os.WriteFile(
			path,
			[]byte(validExtensionDocumentJSON()),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
		source, err := NewDirectorySource(
			(&url.URL{Scheme: "file", Path: directory}).String(),
		)
		if err != nil {
			t.Fatalf("NewDirectorySource() error = %v", err)
		}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(30*time.Minute), nil)

		if _, err := service.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		result, err := service.Browse(ctx, "", 0, 10)
		if err != nil {
			t.Fatalf("Browse() error = %v", err)
		}
		if got, want := result.Entries[0].EntryID, "bridge-github"; got != want {
			t.Fatalf("Browse() entry id = %q, want checkout entry %q", got, want)
		}
	})

	t.Run("Should return cached pages while concurrent reads coalesce a stale refresh", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		catalog := openMarketplaceTestStore(t)
		at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		if err := catalog.ReplaceSource(
			ctx,
			CompozyCatalogSource,
			testSourceGeneration(t, catalog, CompozyCatalogSource),
			testDocument(at, testEntry("old", "Old", "cached")),
		); err != nil {
			t.Fatal(err)
		}
		started, release := make(chan struct{}), make(chan struct{})
		source := &recordingSource{fetch: func(ctx context.Context) (*Document, error) {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return testDocument(at.Add(2*time.Hour), testEntry("new", "New", "refreshed")), nil
		}}
		service := newMarketplaceTestService(t, catalog, source, at.Add(2*time.Hour), nil)
		var wg sync.WaitGroup
		for range 12 {
			wg.Go(func() {
				page, err := service.Browse(ctx, "", 0, 10)
				if err != nil || len(page.Entries) != 1 || page.Entries[0].EntryID != "old" || !page.Stale {
					t.Errorf("cached page = %+v, %v", page, err)
				}
			})
		}
		wg.Wait()
		<-started
		if source.calls.Load() != 1 {
			t.Fatal("reads started more than one refresh")
		}
		service.flightMu.Lock()
		flight := service.byName[CompozyCatalogSource].flight
		service.flightMu.Unlock()
		close(release)
		if _, err := awaitRefreshFlight(ctx, flight); err != nil {
			t.Fatal(err)
		}
		page, err := service.Browse(ctx, "", 0, 10)
		if err != nil || page.Stale || len(page.Entries) != 1 || page.Entries[0].EntryID != "new" {
			t.Fatalf("refreshed page = %+v, %v", page, err)
		}
	})

	t.Run("Should keep a shared refresh alive when its first caller cancels", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		started := make(chan struct{})
		release := make(chan struct{})
		now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			close(started)
			<-release
			return testDocument(now, testEntry("shared", "Shared", "Detached refresh")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, nil)

		leaderCtx, cancelLeader := context.WithCancel(t.Context())
		leaderResult := make(chan error, 1)
		go func() {
			_, refreshErr := service.Refresh(leaderCtx)
			leaderResult <- refreshErr
		}()
		<-started
		cancelLeader()

		select {
		case err := <-leaderResult:
			if !errors.Is(err, context.Canceled) {
				close(release)
				t.Fatalf("Refresh(canceled leader) error = %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			close(release)
			<-leaderResult
			t.Fatal("Refresh(canceled leader) did not return while the shared refresh remained active")
		}

		service.flightMu.Lock()
		flight := service.byName[CompozyCatalogSource].flight
		service.flightMu.Unlock()
		if flight == nil {
			close(release)
			t.Fatal("shared refresh was removed when its first caller canceled")
		}

		close(release)
		select {
		case <-flight.done:
		case <-time.After(time.Second):
			t.Fatal("shared refresh did not finish after its source was released")
		}
		if got, want := source.calls.Load(), int32(1); got != want {
			t.Fatalf("source calls = %d, want one shared refresh", got)
		}
		if _, err := store.GetEntry(t.Context(), CompozyCatalogSource, "shared"); err != nil {
			t.Fatalf("GetEntry(shared) error = %v, want the detached refresh to persist its result", err)
		}
	})

	t.Run("Should cancel and join an owned refresh before rejecting later work", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		started := make(chan struct{})
		canceled := make(chan struct{})
		release := make(chan struct{})
		now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{fetch: func(ctx context.Context) (*Document, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
			return testDocument(now, testEntry("obsolete", "Obsolete", "Closed generation")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, nil)
		refreshResult := make(chan error, 1)
		go func() {
			_, err := service.Refresh(t.Context())
			refreshResult <- err
		}()
		<-started

		closeResult := make(chan error, 1)
		go func() { closeResult <- service.Close(t.Context()) }()
		<-canceled
		select {
		case err := <-closeResult:
			close(release)
			t.Fatalf("Close() returned before the flight joined: %v", err)
		default:
		}
		close(release)
		if err := <-closeResult; err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if err := <-refreshResult; !errors.Is(err, ErrServiceClosed) {
			t.Fatalf("Refresh() error = %v, want ErrServiceClosed", err)
		}
		if _, err := store.GetEntry(t.Context(), CompozyCatalogSource, "obsolete"); !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("GetEntry(obsolete) error = %v, want no stale-generation commit", err)
		}
		if err := service.Close(t.Context()); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
		if _, err := service.Refresh(t.Context()); !errors.Is(err, ErrServiceClosed) {
			t.Fatalf("Refresh(after close) error = %v, want ErrServiceClosed", err)
		}
	})

	t.Run("Should bound and join refresh notification during close", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
		notifier := newBlockingRefreshNotifier()
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(now, testEntry("notified", "Notified", "Joined notification")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, notifier)
		t.Cleanup(func() {
			notifier.releaseNotification()
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
			defer cancel()
			if err := service.Close(cleanupCtx); err != nil {
				t.Errorf("cleanup Close() error = %v", err)
			}
		})

		refreshResult := make(chan error, 1)
		go func() {
			_, err := service.Refresh(t.Context())
			refreshResult <- err
		}()

		if hasDeadline := <-notifier.started; !hasDeadline {
			t.Fatal("NotifyCatalogRefresh() context has no deadline")
		}
		closeCtx, cancelClose := context.WithTimeout(t.Context(), time.Second)
		defer cancelClose()
		if err := service.Close(closeCtx); err != nil {
			t.Fatalf("Close() error = %v, want joined notification", err)
		}
		select {
		case <-notifier.stopped:
		default:
			t.Fatal("Close() returned before the refresh notification stopped")
		}
		if err := <-refreshResult; !errors.Is(err, ErrServiceClosed) || !errors.Is(err, context.Canceled) {
			t.Fatalf("Refresh() error = %v, want ErrServiceClosed and context.Canceled", err)
		}
	})

	t.Run("Should persist stale state after the service refresh deadline expires", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		source := &recordingSource{fetch: func(ctx context.Context) (*Document, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		service, err := NewService(
			t.Context(),
			store,
			marketplaceTestBindings(t, source),
			time.Hour,
			30*time.Millisecond,
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		_, err = service.Refresh(t.Context())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Refresh() error = %v, want context.DeadlineExceeded", err)
		}
		state, err := store.SourceState(t.Context(), CompozyCatalogSource)
		if err != nil {
			t.Fatalf("SourceState() error = %v, want persisted timeout state", err)
		}
		if !state.Stale || state.ErrorClass != "timeout" || !strings.Contains(state.LastError, "deadline exceeded") {
			t.Fatalf("SourceState() = %#v, want durable timeout failure", state)
		}
	})

	t.Run("Should propagate canonical refresh event persistence failures", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		notifyErr := errors.New("event store unavailable")
		notifier := &recordingRefreshNotifier{err: notifyErr}
		now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(now, testEntry("persisted", "Persisted", "Committed projection")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, notifier)

		_, err := service.Refresh(t.Context())
		if !errors.Is(err, notifyErr) {
			t.Fatalf("Refresh() error = %v, want event persistence failure", err)
		}
		if _, err := store.GetEntry(t.Context(), CompozyCatalogSource, "persisted"); err != nil {
			t.Fatalf("GetEntry(persisted) error = %v, want committed projection", err)
		}
	})

	t.Run("Should force refresh a fresh source and prune a pulled entry", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		fetchedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
		if err := store.ReplaceSource(
			ctx,
			CompozyCatalogSource,
			testSourceGeneration(t, store, CompozyCatalogSource),
			testDocument(
				fetchedAt,
				testEntry("keep", "Keep", "Still curated"),
				testEntry("pulled", "Pulled", "Kill switch target"),
			),
		); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(
				fetchedAt.Add(10*time.Minute),
				testEntry("keep", "Keep", "Still curated"),
			), nil
		}}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(10*time.Minute), nil)

		report, err := service.Refresh(ctx)
		if err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}
		if got, want := report.Outcomes[0].EntryCount, 1; got != want {
			t.Fatalf("Refresh() entry count = %d, want %d", got, want)
		}
		page, err := store.BrowseSource(ctx, CompozyCatalogSource, "", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want || page.Entries[0].EntryID != "keep" {
			t.Fatalf("BrowseSource() = %#v, want only keep after force refresh", page)
		}
	})
}

func TestCatalogServiceStaleFallbackAndNotifications(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve stale rows, redact errors, and emit a classified failure", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		fetchedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
		if err := store.ReplaceSource(
			ctx,
			CompozyCatalogSource,
			testSourceGeneration(t, store, CompozyCatalogSource),
			testDocument(
				fetchedAt,
				testEntry("offline", "Offline server", "Survives feed outage"),
			),
		); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return nil, errors.New("dial feed: token=super-secret-value")
		}}
		notifier := &recordingRefreshNotifier{}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(2*time.Hour), notifier)

		if _, err := service.Refresh(ctx); err == nil {
			t.Fatal("expected refresh failure")
		}
		result, err := service.Browse(ctx, "", 0, 10)
		if err != nil {
			t.Fatalf("Browse() stale fallback error = %v", err)
		}
		if got, want := len(result.Entries), 1; got != want {
			t.Fatalf("Browse() stale entries = %d, want %d", got, want)
		}
		if !result.Stale || result.ErrorClass != errorClassNetwork {
			t.Fatalf("Browse() state = %#v, want stale network state", result.Sources)
		}
		if strings.Contains(result.LastError, "super-secret-value") ||
			!strings.Contains(result.LastError, "[REDACTED]") {
			t.Fatalf("Browse() LastError = %q, want redacted detail", result.LastError)
		}
		outcomes := notifier.snapshot()
		if got, want := len(outcomes), 1; got != want {
			t.Fatalf("notifier outcomes = %d, want %d", got, want)
		}
		if outcome := outcomes[0]; outcome.Outcome != RefreshOutcomeFailed || !outcome.Stale ||
			outcome.ErrorClass != errorClassNetwork || outcome.EntryCount != 1 {
			t.Fatalf("notifier outcome = %#v, want classified stale failure", outcome)
		}
	})

	t.Run("Should return a source refresh error without fabricating a projection", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return nil, errors.New("feed unavailable")
		}}
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		service := newMarketplaceTestService(t, store, source, now, nil)

		_, err := service.Refresh(testutil.Context(t))
		if err == nil {
			t.Fatal("Browse() error = nil, want failure without stale projection")
		}
		if !strings.Contains(err.Error(), "feed unavailable") {
			t.Fatalf("Browse() error = %v, want source failure", err)
		}
		if !errors.Is(err, ErrSourceUnavailable) {
			t.Fatalf("Browse() error = %v, want ErrSourceUnavailable", err)
		}
	})
}

type recordingSource struct {
	fetch func(context.Context) (*Document, error)
	calls atomic.Int32
}

func (s *recordingSource) Fetch(ctx context.Context) (*Document, error) {
	s.calls.Add(1)
	return s.fetch(ctx)
}

type recordingRefreshNotifier struct {
	mu       sync.Mutex
	outcomes []RefreshOutcome
	err      error
}

type blockingRefreshNotifier struct {
	started     chan bool
	stopped     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

func newBlockingRefreshNotifier() *blockingRefreshNotifier {
	return &blockingRefreshNotifier{
		started: make(chan bool, 1),
		stopped: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (n *blockingRefreshNotifier) NotifyCatalogRefresh(ctx context.Context, _ RefreshOutcome) error {
	_, hasDeadline := ctx.Deadline()
	n.started <- hasDeadline
	defer close(n.stopped)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-n.release:
		return nil
	}
}

func (n *blockingRefreshNotifier) NotifyInstall(context.Context, InstallOutcome) error { return nil }

func (n *blockingRefreshNotifier) releaseNotification() {
	n.releaseOnce.Do(func() {
		close(n.release)
	})
}

func (n *recordingRefreshNotifier) NotifyCatalogRefresh(_ context.Context, outcome RefreshOutcome) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.outcomes = append(n.outcomes, outcome)
	return n.err
}

func (n *recordingRefreshNotifier) NotifyInstall(context.Context, InstallOutcome) error { return n.err }

func (n *recordingRefreshNotifier) snapshot() []RefreshOutcome {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]RefreshOutcome(nil), n.outcomes...)
}

func marketplaceTestBindings(t *testing.T, source Source) []SourceBinding {
	t.Helper()
	return []SourceBinding{
		{
			Config: ResolvedSource{
				Name:    CompozyCatalogSource,
				Ref:     CompozyCatalogRef,
				Kind:    SourceKindFeed,
				Enabled: true,
			},
			Fetcher: source,
		},
	}
}

func newMarketplaceTestService(
	t *testing.T,
	store Store,
	source Source,
	now time.Time,
	notifier Notifier,
) *CatalogService {
	t.Helper()
	options := []ServiceOption{WithNow(func() time.Time { return now })}
	if notifier != nil {
		options = append(options, WithNotifier(notifier))
	}
	service, err := NewService(
		t.Context(),
		store,
		marketplaceTestBindings(t, source),
		time.Hour,
		time.Minute,
		options...)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(testutil.Context(t)); err != nil {
			t.Error(err)
		}
	})
	return service
}

// Invariant: a fetch captures generation before I/O; obsolete success and failure cannot mutate or notify current state.
// Owner: catalog refresh lifecycle; canonical suite: service_test.go (UT-008, UT-069).
func TestCatalogServiceSourceGeneration(t *testing.T) {
	t.Parallel()
	for _, fetchFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should discard an obsolete fetch with failure=%t", fetchFails), func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			catalog := openMarketplaceTestStore(t)
			at := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
			started, release := make(chan struct{}), make(chan struct{})
			source := &recordingSource{fetch: func(ctx context.Context) (*Document, error) {
				close(started)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
				}
				if fetchFails {
					return nil, errors.New("old source unreachable")
				}
				return testDocument(at, testEntry("old", "Old", "Obsolete fetch")), nil
			}}
			notifier := &recordingRefreshNotifier{}
			service := newMarketplaceTestService(t, catalog, source, at, notifier)
			finished := make(chan error, 1)
			go func() { _, err := service.Refresh(ctx); finished <- err }()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			configuration, err := catalog.ConfigureSources(ctx, []ResolvedSource{{
				Name: CompozyCatalogSource, Ref: CompozyCatalogRef, Kind: SourceKindFeed,
				Enabled: true, Revision: "replacement-acquisition",
			}}, "replacement-configuration")
			if err != nil {
				t.Fatal(err)
			}
			generation := configuration.SourceGenerations[CompozyCatalogSource]
			current := testDocument(at, testEntry("current", "Current", "New source configuration"))
			if err := catalog.ReplaceSource(ctx, CompozyCatalogSource, generation, current); err != nil {
				t.Fatal(err)
			}
			close(release)
			select {
			case err := <-finished:
				if !errors.Is(err, storepkg.ErrMarketplaceCatalogGenerationStale) {
					t.Fatalf("refresh error=%v", err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			page, err := catalog.BrowseSource(ctx, CompozyCatalogSource, "", 0, 100)
			if err != nil || len(page.Entries) != 1 || page.Entries[0].EntryID != "current" || page.State.Stale ||
				page.State.Generation != generation {
				t.Fatalf("current page=%#v error=%v", page, err)
			}
			if events := notifier.snapshot(); len(events) != 0 {
				t.Fatalf("obsolete fetch emitted events: %#v", events)
			}
		})
	}
	t.Run("Should include the captured source generation in a successful refresh event", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		catalog := openMarketplaceTestStore(t)
		at := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
		notifier := &recordingRefreshNotifier{}
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(at, testEntry("current", "Current", "Published")), nil
		}}
		service := newMarketplaceTestService(t, catalog, source, at, notifier)
		generation := testSourceGeneration(t, catalog, CompozyCatalogSource)
		if _, err := service.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		events := notifier.snapshot()
		if len(events) != 1 || events[0].Source != CompozyCatalogSource || events[0].Generation != generation ||
			events[0].Outcome != RefreshOutcomeSucceeded {
			t.Fatalf("refresh events=%#v", events)
		}
	})
}

// Invariant: pages use one ordered source snapshot, while lifecycle fences reject obsolete remote work.
// Owner: catalog aggregation and refresh lifecycle; canonical suite: service_test.go (UT-002/003/067/073).
func TestCatalogServiceSources(t *testing.T) {
	t.Parallel()
	t.Run("Should reject retained source names without changing the active projection", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		fetcher := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(at, testEntry("tool", "Tool", "Retained origin")), nil
		}}
		service := newMarketplaceTestService(t, openMarketplaceTestStore(t), fetcher, at, nil)
		service.installedPackages = func(context.Context) ([]InstalledPackage, error) {
			return []InstalledPackage{
				{Name: "second", SourceName: "team", SourceRef: "github:team/plugins"},
				{Name: "first", SourceName: "team", SourceRef: "github:team/plugins"},
			}, nil
		}
		feed := marketplaceTestBindings(t, fetcher)
		team := SourceBinding{Config: ResolvedSource{
			Name: "team", Ref: "github:team/plugins", Kind: SourceKindCustom, Enabled: true,
		}, Fetcher: fetcher}
		if err := service.SetSources(ctx, append(slices.Clone(feed), team)); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		before, err := service.Browse(ctx, "", 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		replacement := team
		replacement.Config.Ref = "github:other/plugins"
		err = service.SetSources(ctx, append(slices.Clone(feed), replacement))
		var retained *SourceNameRetainedError
		if !errors.Is(err, ErrSourceNameRetained) || !errors.As(err, &retained) ||
			retained.Name != "team" || !slices.Equal(retained.RetainedBy, []string{"first", "second"}) {
			t.Fatalf("retained name = %v", err)
		}
		after, err := service.Browse(ctx, "", 0, 10)
		if err != nil || after.Revision != before.Revision || after.Total != before.Total {
			t.Fatalf("rejected configuration changed projection: %+v, %v", after, err)
		}
		if err := service.SetSources(ctx, feed); err != nil {
			t.Fatal(err)
		}
		if err := service.SetSources(
			ctx,
			append(slices.Clone(feed), replacement),
		); !errors.Is(
			err,
			ErrSourceNameRetained,
		) {
			t.Fatalf("removed name lost retention: %v", err)
		}
		team.Config.Name = "renamed"
		if err := service.SetSources(ctx, append(slices.Clone(feed), team)); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Refresh(ctx, "renamed"); err != nil {
			t.Fatal(err)
		}
		entry, err := service.Entry(ctx, Origin{SourceRef: team.Config.Ref, EntryID: "tool"})
		if err != nil || entry.SourceName != "renamed" {
			t.Fatalf("renamed origin = %+v, %v", entry, err)
		}
	})
	t.Run("Should keep curated acquisition refs distinct from plugin source slugs", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			entry := testEntry("tool", "Tool", "Same slug in separate acquisition namespaces")
			entry.InstallSlug = "team/tool"
			return testDocument(at, entry), nil
		}}
		service := newMarketplaceTestService(t, openMarketplaceTestStore(t), source, at, nil)
		bindings := append(marketplaceTestBindings(t, source), SourceBinding{
			Config:  ResolvedSource{Name: "team", Ref: "github:team/plugins", Kind: SourceKindCustom, Enabled: true},
			Fetcher: source,
		})
		if err := service.SetSources(ctx, bindings); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		curated, err := service.ResolveExtensionInstall(ctx, "team/tool", "1.0.0")
		if err != nil || curated.SourceName != CompozyCatalogSource {
			t.Fatalf("curated acquisition = %+v, %v", curated, err)
		}
		plugin, err := service.Detail(ctx, "team", "tool")
		if err != nil || plugin.SourceName != "team" {
			t.Fatalf("plugin acquisition = %+v, %v", plugin, err)
		}
	})
	t.Run(
		"Should page across enabled sources and join renamed origins without fetching during lookup",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
			binding := func(name, ref, kind string, enabled bool) SourceBinding {
				return SourceBinding{
					Config: ResolvedSource{Name: name, Ref: ref, Kind: kind, Enabled: enabled},
					Fetcher: &recordingSource{fetch: func(context.Context) (*Document, error) {
						if !enabled {
							return nil, errors.New("disabled source was fetched")
						}
						first, second := testEntry("same", "beta", name), testEntry("alpha", "Alpha", name)
						if kind != SourceKindFeed {
							first.InstallSlug, second.InstallSlug = name+"/same", name+"/alpha"
						}
						return testDocument(at, first, second), nil
					}},
				}
			}
			feed := binding(CompozyCatalogSource, CompozyCatalogRef, SourceKindFeed, true)
			preset := binding("preset", "github:team/preset", SourceKindPreset, true)
			custom := binding("team", "github:team/plugins", SourceKindCustom, true)
			disabled := binding("off", "github:team/off", SourceKindPreset, false)
			catalog := openMarketplaceTestStore(t)
			service, err := NewService(
				ctx,
				catalog,
				[]SourceBinding{custom, disabled, preset, feed},
				time.Hour,
				time.Minute,
				WithNow(func() time.Time { return at }),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Close(testutil.Context(t)); err != nil {
					t.Error(err)
				}
			})
			if _, err := service.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			page, err := service.Browse(ctx, "", 1, 4)
			if err != nil || page.Total != 6 || len(page.Entries) != 4 || len(page.Sources) != 4 || page.Stale {
				t.Fatalf("merged page = %+v, %v", page, err)
			}
			wantSources := []string{CompozyCatalogSource, "preset", "preset", "team"}
			wantIDs := []string{"same", "alpha", "same", "alpha"}
			for i, entry := range page.Entries {
				if entry.SourceName != wantSources[i] || entry.EntryID != wantIDs[i] {
					t.Fatalf("row %d = %+v", i, entry)
				}
			}
			if _, err := service.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			identical, err := service.Browse(ctx, "", 1, 4)
			if err != nil || identical.Revision != page.Revision {
				t.Fatalf("unchanged refresh invalidated cursor: %+v, %v", identical, err)
			}
			filtered, err := service.Browse(ctx, "ALPHA", 1, 1)
			if err != nil || filtered.Total != 3 || len(filtered.Entries) != 1 ||
				filtered.Entries[0].SourceName != "preset" {
				t.Fatalf("filtered merged page = %+v, %v", filtered, err)
			}
			if _, err := service.Detail(ctx, "off", "same"); !errors.Is(err, ErrEntryNotFound) {
				t.Fatalf("disabled detail = %v", err)
			}
			entry, err := service.Detail(ctx, "team", "same")
			if err != nil || entry.SourceName != "team" {
				t.Fatalf("install lookup = %+v, %v", entry, err)
			}
			if disabled.Fetcher.(*recordingSource).calls.Load() != 0 {
				t.Fatal("disabled source was contacted")
			}
			renamed := binding("renamed", custom.Config.Ref, SourceKindCustom, true)
			if err := service.SetSources(ctx, []SourceBinding{feed, preset, disabled, renamed}); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Refresh(ctx, "renamed"); err != nil {
				t.Fatal(err)
			}
			joined, err := service.Entry(ctx, Origin{SourceRef: custom.Config.Ref, EntryID: "same"})
			if err != nil || joined.SourceName != "renamed" {
				t.Fatalf("origin after rename = %+v, %v", joined, err)
			}
			changed, err := service.Browse(ctx, "", 0, 10)
			if err != nil || changed.Revision == page.Revision || changed.Total != 6 {
				t.Fatalf("renamed page = %+v, %v", changed, err)
			}
			if _, err := catalog.SourceState(ctx, "team"); !errors.Is(err, ErrSourceStateMissing) {
				t.Fatalf("removed source survived = %v", err)
			}
		},
	)
	t.Run("Should join a cached origin through an alias when its first source has no projection", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{fetch: func(context.Context) (*Document, error) {
			return testDocument(at, testEntry("shared", "Shared", "One immutable origin")), nil
		}}
		bindings := marketplaceTestBindings(t, source)
		for _, name := range []string{"first", "second"} {
			bindings = append(bindings, SourceBinding{
				Config:  ResolvedSource{Name: name, Ref: "github:team/plugins", Kind: SourceKindCustom, Enabled: true},
				Fetcher: source,
			})
		}
		service, err := NewService(ctx, openMarketplaceTestStore(t), bindings, time.Hour, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Close(testutil.Context(t)); err != nil {
				t.Error(err)
			}
		})
		if _, err := service.Refresh(ctx, "second"); err != nil {
			t.Fatal(err)
		}
		entry, err := service.Entry(ctx, Origin{SourceRef: "github:team/plugins", EntryID: "shared"})
		if err != nil || entry.SourceName != "second" || source.calls.Load() != 1 {
			t.Fatalf("origin lookup=%+v, err=%v, source calls=%d", entry, err, source.calls.Load())
		}
	})
	t.Run("Should preserve an unchanged source flight when another source is added", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		started, release := make(chan struct{}), make(chan struct{})
		releaseFetch := sync.OnceFunc(func() { close(release) })
		source := &recordingSource{fetch: func(ctx context.Context) (*Document, error) {
			close(started)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-release:
				return testDocument(at, testEntry("retained", "Retained", "Source unchanged")), nil
			}
		}}
		catalog := openMarketplaceTestStore(t)
		service := newMarketplaceTestService(t, catalog, source, at, nil)
		t.Cleanup(releaseFetch)
		originalGeneration := testSourceGeneration(t, catalog, CompozyCatalogSource)
		finished := make(chan error, 1)
		go func() { _, err := service.Refresh(ctx); finished <- err }()
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		bindings := marketplaceTestBindings(t, source)
		bindings = append(bindings, SourceBinding{
			Config: ResolvedSource{
				Name:    "disabled",
				Ref:     "github:team/disabled",
				Kind:    SourceKindCustom,
				Enabled: false,
			},
			Fetcher: &recordingSource{fetch: func(context.Context) (*Document, error) {
				return nil, errors.New("disabled source must not be contacted")
			}},
		})
		if err := service.SetSources(ctx, bindings); err != nil {
			t.Fatal(err)
		}
		if got := testSourceGeneration(t, catalog, CompozyCatalogSource); got != originalGeneration {
			t.Fatalf("unchanged source generation = %d, want %d", got, originalGeneration)
		}
		releaseFetch()
		select {
		case err := <-finished:
			if err != nil {
				t.Fatalf("unchanged refresh was interrupted: %v", err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		entry, err := service.Detail(ctx, CompozyCatalogSource, "retained")
		if err != nil || entry.EntryID != "retained" || source.calls.Load() != 1 {
			t.Fatalf("entry=%+v, err=%v, calls=%d", entry, err, source.calls.Load())
		}
	})
	for _, operation := range []string{"remove", "disable", "re-add"} {
		for _, fail := range []bool{false, true} {
			t.Run(
				fmt.Sprintf("Should reject late success or failure after %s with failure=%t", operation, fail),
				func(t *testing.T) {
					t.Parallel()
					ctx := t.Context()
					at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
					feed := marketplaceTestBindings(t, &recordingSource{fetch: func(context.Context) (*Document, error) { return testDocument(at), nil }})[0]
					started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
					releaseOld := sync.OnceFunc(func() { close(release) })
					old := SourceBinding{
						Config: ResolvedSource{
							Name:    "team",
							Ref:     "github:team/plugins",
							Kind:    SourceKindCustom,
							Enabled: true,
						},
						Fetcher: &recordingSource{fetch: func(ctx context.Context) (*Document, error) {
							close(started)
							<-ctx.Done()
							close(canceled)
							<-release
							if fail {
								return nil, errors.New("late source failure")
							}
							return testDocument(at, testEntry("old", "Old", "obsolete")), nil
						}},
					}
					catalog := openMarketplaceTestStore(t)
					notifier := &recordingRefreshNotifier{}
					service, err := NewService(
						ctx,
						catalog,
						[]SourceBinding{feed, old},
						time.Hour,
						time.Minute,
						WithNotifier(notifier),
					)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := service.Close(testutil.Context(t)); err != nil {
							t.Error(err)
						}
					})
					t.Cleanup(releaseOld)
					finished := make(chan error, 1)
					go func() { _, err := service.Refresh(ctx, "team"); finished <- err }()
					<-started
					next := []SourceBinding{feed}
					if operation == "disable" {
						disabled := old
						disabled.Config.Enabled = false
						next = append(next, disabled)
					}
					if err := service.SetSources(ctx, next); err != nil {
						t.Fatal(err)
					}
					<-canceled
					if operation == "re-add" {
						current := old
						current.Fetcher = &recordingSource{fetch: func(context.Context) (*Document, error) {
							return testDocument(at, testEntry("current", "Current", "new generation")), nil
						}}
						if err := service.SetSources(ctx, []SourceBinding{feed, current}); err != nil {
							t.Fatal(err)
						}
						if _, err := service.Refresh(ctx, "team"); err != nil {
							t.Fatal(err)
						}
					}
					releaseOld()
					if err := <-finished; !errors.Is(err, storepkg.ErrMarketplaceCatalogGenerationStale) {
						t.Fatalf("late refresh = %v", err)
					}
					if _, err := catalog.GetEntry(ctx, "team", "old"); !errors.Is(err, ErrEntryNotFound) {
						t.Fatalf("obsolete row published: %v", err)
					}
					events := notifier.snapshot()
					if operation == "re-add" {
						current, err := service.Detail(ctx, "team", "current")
						if err != nil || current.EntryID != "current" || len(events) != 1 {
							t.Fatalf("current entry=%+v, events=%+v, %v", current, events, err)
						}
					} else if len(events) != 0 {
						t.Fatalf("obsolete refresh notified: %+v", events)
					}
				},
			)
		}
	}
}
