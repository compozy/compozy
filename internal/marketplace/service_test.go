package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestNewCatalogServiceValidation(t *testing.T) {
	t.Parallel()

	store := openMarketplaceTestStore(t)
	validSource := &recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
		return testDocument(time.Now().UTC(), testEntry(KindSkill, "skill", "Skill", "Valid source")), nil
	}}
	tests := []struct {
		name    string
		store   Store
		sources []Source
		ttl     time.Duration
		timeout time.Duration
		wantErr string
	}{
		{
			name:    "Should reject a missing store",
			sources: []Source{validSource},
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "store is required",
		},
		{
			name:    "Should reject a non-positive TTL",
			store:   store,
			sources: []Source{validSource},
			timeout: time.Minute,
			wantErr: "TTL must be positive",
		},
		{
			name:    "Should reject a non-positive refresh timeout",
			store:   store,
			sources: []Source{validSource},
			ttl:     time.Hour,
			wantErr: "refresh timeout must be positive",
		},
		{
			name:    "Should reject a missing source",
			store:   store,
			sources: []Source{nil},
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "source is required",
		},
		{
			name:    "Should reject an empty source set",
			store:   store,
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "at least one source",
		},
		{
			name:    "Should reject an unsupported source kind",
			store:   store,
			sources: []Source{&recordingSource{kind: Kind("bundle")}},
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "unsupported kind",
		},
		{
			name:    "Should reject duplicate kind sources",
			store:   store,
			sources: []Source{validSource, validSource},
			ttl:     time.Hour,
			timeout: time.Minute,
			wantErr: "registered more than once",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewService(tc.store, tc.sources, tc.ttl, tc.timeout)
			if err == nil {
				t.Fatal("NewService() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewService() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestCatalogServiceDetailStatusAndSelection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	newService := func(t *testing.T) *CatalogService {
		t.Helper()
		sources := []Source{
			&recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
				return testDocument(now, testEntry(KindSkill, "skill", "Skill", "Detail fixture")), nil
			}},
			&recordingSource{kind: KindMCP, fetch: func(context.Context) (*Document, error) {
				return testDocument(now, testEntry(KindMCP, "mcp", "MCP", "Status fixture")), nil
			}},
		}
		service, err := NewService(
			openMarketplaceTestStore(t),
			sources,
			time.Hour,
			time.Minute,
			WithNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		return service
	}

	t.Run("Should return status in kind order and resolve detail by immutable id", func(t *testing.T) {
		t.Parallel()

		service := newService(t)
		ctx := testutil.Context(t)
		states, err := service.Status(ctx)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got, want := len(states), 2; got != want || states[0].Kind != KindMCP || states[1].Kind != KindSkill {
			t.Fatalf("Status() = %#v, want missing states in deterministic kind order", states)
		}
		entry, err := service.Detail(ctx, KindSkill, "skill")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if entry.EntryID != "skill" {
			t.Fatalf("Detail().EntryID = %q, want skill", entry.EntryID)
		}
		if _, err := service.Detail(ctx, KindSkill, "missing"); !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("Detail(missing) error = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("Should resolve extension install by slug and fail for a missing slug", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		extensionSource := &recordingSource{kind: KindExtension, fetch: func(context.Context) (*Document, error) {
			entry := testEntry(KindExtension, "telemetry", "Telemetry", "Curated extension")
			entry.Version = "2.1.0"
			return testDocument(now, entry), nil
		}}
		extensionService := newMarketplaceTestService(t, openMarketplaceTestStore(t), extensionSource, now, nil)
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

	t.Run("Should preserve refresh and missing-entry failures together", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		refreshErr := errors.New("catalog unavailable")
		extensionSource := &recordingSource{kind: KindExtension, fetch: func(context.Context) (*Document, error) {
			return nil, refreshErr
		}}
		extensionService := newMarketplaceTestService(t, openMarketplaceTestStore(t), extensionSource, now, nil)
		_, err := extensionService.ResolveExtensionInstall(ctx, "compozy/missing", "1.0.0")
		if !errors.Is(err, refreshErr) || !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("ResolveExtensionInstall() error = %v, want refresh and missing-entry identities", err)
		}
		var resolutionErr *ExtensionInstallResolutionError
		if !errors.As(err, &resolutionErr) || resolutionErr.RefreshErr == nil || resolutionErr.LookupErr == nil {
			t.Fatalf("ResolveExtensionInstall() error = %#v, want combined resolution error", err)
		}
	})

	t.Run("Should deduplicate refresh kinds and reject invalid selections", func(t *testing.T) {
		t.Parallel()

		service := newService(t)
		ctx := testutil.Context(t)
		report, err := service.Refresh(ctx, KindSkill, KindSkill)
		if err != nil {
			t.Fatalf("Refresh(duplicate kinds) error = %v", err)
		}
		if got, want := len(report.Outcomes), 1; got != want {
			t.Fatalf("Refresh(duplicate kinds) outcomes = %d, want %d", got, want)
		}
		_, err = service.Refresh(ctx, KindExtension)
		if err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Fatalf("Refresh(unconfigured kind) error = %v, want source diagnostic", err)
		}
		_, err = service.Refresh(ctx, Kind("bundle"))
		if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
			t.Fatalf("Refresh(unsupported kind) error = %v, want kind diagnostic", err)
		}
	})

	t.Run("Should reject nil contexts and service receivers with specific diagnostics", func(t *testing.T) {
		t.Parallel()

		service := newService(t)
		ctx := testutil.Context(t)
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err := service.Browse(nil, KindSkill, "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "service context is required") {
			t.Fatalf("Browse(nil context) error = %v, want service-context validation", err)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err = service.Detail(nil, KindSkill, "skill")
		if err == nil || !strings.Contains(err.Error(), "service context is required") {
			t.Fatalf("Detail(nil context) error = %v, want service-context validation", err)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err = service.Refresh(nil)
		if err == nil || !strings.Contains(err.Error(), "refresh context is required") {
			t.Fatalf("Refresh(nil context) error = %v, want refresh-context validation", err)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err = service.Status(nil)
		if err == nil || !strings.Contains(err.Error(), "status context is required") {
			t.Fatalf("Status(nil context) error = %v, want status-context validation", err)
		}
		var unavailable *CatalogService
		_, err = unavailable.Browse(ctx, KindSkill, "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "service is required") {
			t.Fatalf("Browse(nil service) error = %v, want service validation", err)
		}
		_, err = unavailable.Refresh(ctx)
		if err == nil || !strings.Contains(err.Error(), "service sources are required") {
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
		{name: "Should classify timeout", fetchErr: context.DeadlineExceeded, wantClass: "timeout"},
		{name: "Should classify oversized payload", fetchErr: ErrResponseTooLarge, wantClass: "payload_too_large"},
		{
			name:      "Should classify unsupported manifest",
			fetchErr:  &UnsupportedManifestVersionError{Kind: KindSkill, Version: 2},
			wantClass: "manifest_version",
		},
		{name: "Should classify HTTP status", fetchErr: &httpStatusError{status: 503}, wantClass: "http_status"},
		{name: "Should classify JSON decode", fetchErr: &json.SyntaxError{Offset: 1}, wantClass: "decode"},
		{name: "Should classify validation", fetchErr: errors.New("name is required"), wantClass: "validation"},
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
			source := &recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
				return nil, tc.fetchErr
			}}
			service := newMarketplaceTestService(t, store, source, time.Now().UTC(), nil)
			report, err := service.Refresh(testutil.Context(t), KindSkill)
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
		if err := store.ReplaceKind(ctx, KindSkill, testDocument(
			fetchedAt,
			testEntry(KindSkill, "fresh", "Fresh skill", "Already projected"),
		)); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
		}
		source := &recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
			return nil, errors.New("unexpected fetch")
		}}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(30*time.Minute), nil)

		result, err := service.Browse(ctx, KindSkill, "", 0, 10)
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

	t.Run("Should collapse concurrent stale browse refreshes into one fetch", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		fetchedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
		if err := store.ReplaceKind(ctx, KindMCP, testDocument(
			fetchedAt,
			testEntry(KindMCP, "old", "Old server", "Removed by refresh"),
		)); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
		}
		refreshAt := fetchedAt.Add(2 * time.Hour)
		source := &recordingSource{kind: KindMCP, fetch: func(context.Context) (*Document, error) {
			time.Sleep(20 * time.Millisecond)
			return testDocument(
				refreshAt,
				testEntry(KindMCP, "new", "New server", "Projected once"),
			), nil
		}}
		service := newMarketplaceTestService(t, store, source, refreshAt, nil)

		const callers = 12
		errorsCh := make(chan error, callers)
		var wait sync.WaitGroup
		wait.Add(callers)
		for range callers {
			go func() {
				defer wait.Done()
				result, err := service.Browse(ctx, KindMCP, "", 0, 10)
				if err != nil {
					errorsCh <- err
					return
				}
				if got, want := result.Entries[0].EntryID, "new"; got != want {
					errorsCh <- errors.New("Browse() did not return refreshed entry")
				}
			}()
		}
		wait.Wait()
		close(errorsCh)
		for err := range errorsCh {
			t.Errorf("concurrent Browse() error = %v", err)
		}
		if got, want := source.calls.Load(), int32(1); got != want {
			t.Fatalf("source calls = %d, want singleflight %d", got, want)
		}
	})

	t.Run("Should keep a shared refresh alive when its first caller cancels", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		started := make(chan struct{})
		release := make(chan struct{})
		now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
			close(started)
			<-release
			return testDocument(now, testEntry(KindSkill, "shared", "Shared", "Detached refresh")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, nil)

		leaderCtx, cancelLeader := context.WithCancel(t.Context())
		leaderResult := make(chan error, 1)
		go func() {
			_, refreshErr := service.Refresh(leaderCtx, KindSkill)
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
		flight := service.flights[KindSkill]
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
		if _, err := store.GetEntry(t.Context(), KindSkill, "shared"); err != nil {
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
		source := &recordingSource{kind: KindSkill, fetch: func(ctx context.Context) (*Document, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
			return testDocument(now, testEntry(KindSkill, "obsolete", "Obsolete", "Closed generation")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, nil)
		refreshResult := make(chan error, 1)
		go func() {
			_, err := service.Refresh(t.Context(), KindSkill)
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
		if _, err := store.GetEntry(t.Context(), KindSkill, "obsolete"); !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("GetEntry(obsolete) error = %v, want no stale-generation commit", err)
		}
		if err := service.Close(t.Context()); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
		if _, err := service.Refresh(t.Context(), KindSkill); !errors.Is(err, ErrServiceClosed) {
			t.Fatalf("Refresh(after close) error = %v, want ErrServiceClosed", err)
		}
	})

	t.Run("Should persist stale state after the service refresh deadline expires", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		source := &recordingSource{kind: KindSkill, fetch: func(ctx context.Context) (*Document, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		service, err := NewService(store, []Source{source}, time.Hour, 30*time.Millisecond)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		_, err = service.Refresh(t.Context(), KindSkill)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Refresh() error = %v, want context.DeadlineExceeded", err)
		}
		state, err := store.KindState(t.Context(), KindSkill)
		if err != nil {
			t.Fatalf("KindState() error = %v, want persisted timeout state", err)
		}
		if !state.Stale || state.ErrorClass != "timeout" || !strings.Contains(state.LastError, "deadline exceeded") {
			t.Fatalf("KindState() = %#v, want durable timeout failure", state)
		}
	})

	t.Run("Should propagate canonical refresh event persistence failures", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		notifyErr := errors.New("event store unavailable")
		notifier := &recordingRefreshNotifier{err: notifyErr}
		now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
		source := &recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
			return testDocument(now, testEntry(KindSkill, "persisted", "Persisted", "Committed projection")), nil
		}}
		service := newMarketplaceTestService(t, store, source, now, notifier)

		_, err := service.Refresh(t.Context(), KindSkill)
		if !errors.Is(err, notifyErr) {
			t.Fatalf("Refresh() error = %v, want event persistence failure", err)
		}
		if _, err := store.GetEntry(t.Context(), KindSkill, "persisted"); err != nil {
			t.Fatalf("GetEntry(persisted) error = %v, want committed projection", err)
		}
	})

	t.Run("Should force refresh a fresh kind and prune a pulled entry", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		fetchedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
		if err := store.ReplaceKind(ctx, KindExtension, testDocument(
			fetchedAt,
			testEntry(KindExtension, "keep", "Keep", "Still curated"),
			testEntry(KindExtension, "pulled", "Pulled", "Kill switch target"),
		)); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
		}
		source := &recordingSource{kind: KindExtension, fetch: func(context.Context) (*Document, error) {
			return testDocument(
				fetchedAt.Add(10*time.Minute),
				testEntry(KindExtension, "keep", "Keep", "Still curated"),
			), nil
		}}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(10*time.Minute), nil)

		report, err := service.Refresh(ctx, KindExtension)
		if err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}
		if got, want := report.Outcomes[0].EntryCount, 1; got != want {
			t.Fatalf("Refresh() entry count = %d, want %d", got, want)
		}
		page, err := store.ListKind(ctx, KindExtension, "", 0, 10)
		if err != nil {
			t.Fatalf("ListKind() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want || page.Entries[0].EntryID != "keep" {
			t.Fatalf("ListKind() = %#v, want only keep after force refresh", page)
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
		if err := store.ReplaceKind(ctx, KindMCP, testDocument(
			fetchedAt,
			testEntry(KindMCP, "offline", "Offline server", "Survives feed outage"),
		)); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
		}
		source := &recordingSource{kind: KindMCP, fetch: func(context.Context) (*Document, error) {
			return nil, errors.New("dial feed: token=super-secret-value")
		}}
		notifier := &recordingRefreshNotifier{}
		service := newMarketplaceTestService(t, store, source, fetchedAt.Add(2*time.Hour), notifier)

		result, err := service.Browse(ctx, KindMCP, "", 0, 10)
		if err != nil {
			t.Fatalf("Browse() stale fallback error = %v", err)
		}
		if got, want := len(result.Entries), 1; got != want {
			t.Fatalf("Browse() stale entries = %d, want %d", got, want)
		}
		if !result.State.Stale || result.State.ErrorClass != errorClassNetwork {
			t.Fatalf("Browse() state = %#v, want stale network state", result.State)
		}
		if strings.Contains(result.State.LastError, "super-secret-value") ||
			!strings.Contains(result.State.LastError, "[REDACTED]") {
			t.Fatalf("Browse() LastError = %q, want redacted detail", result.State.LastError)
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

	t.Run("Should return the refresh error when no projection exists", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		source := &recordingSource{kind: KindSkill, fetch: func(context.Context) (*Document, error) {
			return nil, errors.New("feed unavailable")
		}}
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		service := newMarketplaceTestService(t, store, source, now, nil)

		_, err := service.Browse(testutil.Context(t), KindSkill, "", 0, 10)
		if err == nil {
			t.Fatal("Browse() error = nil, want failure without stale projection")
		}
		if !strings.Contains(err.Error(), "feed unavailable") {
			t.Fatalf("Browse() error = %v, want source failure", err)
		}
	})
}

type recordingSource struct {
	kind  Kind
	fetch func(context.Context) (*Document, error)
	calls atomic.Int32
}

func (s *recordingSource) Kind() Kind {
	return s.kind
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
	service, err := NewService(store, []Source{source}, time.Hour, time.Minute, options...)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
