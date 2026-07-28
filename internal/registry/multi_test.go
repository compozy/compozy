package registry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type stubRegistrySource struct {
	name         string
	caps         SourceCaps
	searchFunc   func(context.Context, string, SearchOpts) ([]Listing, error)
	infoFunc     func(context.Context, string) (*Detail, error)
	downloadFunc func(context.Context, string, DownloadOpts) (*DownloadResult, error)
	closeFunc    func() error

	searchCalls   atomic.Int32
	infoCalls     atomic.Int32
	downloadCalls atomic.Int32
	closeCalls    atomic.Int32
}

var _ Source = (*stubRegistrySource)(nil)

func (s *stubRegistrySource) Name() string { return s.name }

func (s *stubRegistrySource) Capabilities() SourceCaps { return s.caps }

func (s *stubRegistrySource) Search(ctx context.Context, query string, opts SearchOpts) ([]Listing, error) {
	s.searchCalls.Add(1)
	if s.searchFunc == nil {
		return nil, nil
	}
	return s.searchFunc(ctx, query, opts)
}

func (s *stubRegistrySource) Info(ctx context.Context, slug string) (*Detail, error) {
	s.infoCalls.Add(1)
	if s.infoFunc == nil {
		return nil, nil
	}
	return s.infoFunc(ctx, slug)
}

func (s *stubRegistrySource) Download(ctx context.Context, slug string, opts DownloadOpts) (*DownloadResult, error) {
	s.downloadCalls.Add(1)
	if s.downloadFunc == nil {
		return nil, nil
	}
	return s.downloadFunc(ctx, slug, opts)
}

func (s *stubRegistrySource) Close() error {
	s.closeCalls.Add(1)
	if s.closeFunc == nil {
		return nil
	}
	return s.closeFunc()
}

func TestMultiRegistrySearchQueriesSourcesConcurrently(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	release := make(chan struct{})

	makeBlockingSource := func(name string) *stubRegistrySource {
		return &stubRegistrySource{
			name: name,
			caps: SourceCaps{Search: true},
			searchFunc: func(ctx context.Context, _ string, _ SearchOpts) ([]Listing, error) {
				started <- name
				select {
				case <-release:
					return []Listing{{Slug: name}}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
	}

	registry := NewMultiRegistry(testLogger(), makeBlockingSource("one"), makeBlockingSource("two"))

	resultCh := make(chan searchResult, 1)
	go func() {
		listings, err := registry.Search(context.Background(), "query", SearchOpts{})
		resultCh <- searchResult{listings: listings, err: err}
	}()

	waitForStarts(t, started, 2)
	close(release)

	result := waitForSearchResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Search() error = %v", result.err)
	}
	if len(result.listings) != 2 {
		t.Fatalf("Search() listings = %#v, want 2 results", result.listings)
	}
}

func TestMultiRegistrySearchMergesAndOverridesByPriority(t *testing.T) {
	t.Parallel()

	low := &stubRegistrySource{
		name: "low",
		caps: SourceCaps{Search: true},
		searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
			return []Listing{
				{Slug: "shared", Name: "Shared Low", Version: "1.0.0"},
				{Slug: "low-only", Name: "Only Low", Version: "1.0.0"},
			}, nil
		},
	}
	high := &stubRegistrySource{
		name: "high",
		caps: SourceCaps{Search: true},
		searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
			return []Listing{
				{Slug: "shared", Name: "Shared High", Version: "2.0.0"},
				{Slug: "high-only", Name: "Only High", Version: "1.0.0"},
			}, nil
		},
	}

	registry := NewMultiRegistry(testLogger(), low, high)
	listings, err := registry.Search(context.Background(), "shared", SearchOpts{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(listings) != 3 {
		t.Fatalf("Search() listings = %#v, want 3 merged results", listings)
	}

	shared := listingBySlug(t, listings, "shared")
	if shared.Name != "Shared High" || shared.Version != "2.0.0" {
		t.Fatalf("shared listing = %#v, want high-priority result", shared)
	}
	if shared.Source != "high" {
		t.Fatalf("shared.Source = %q, want high", shared.Source)
	}
}

func TestMultiRegistrySearchSkipsNonSearchableSources(t *testing.T) {
	t.Parallel()

	skipped := &stubRegistrySource{
		name: "github",
		caps: SourceCaps{Search: false},
		searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
			return nil, errors.New("should not be called")
		},
	}
	searchable := &stubRegistrySource{
		name: "clawhub",
		caps: SourceCaps{Search: true},
		searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
			return []Listing{{Slug: "pkg"}}, nil
		},
	}

	registry := NewMultiRegistry(testLogger(), skipped, searchable)
	listings, err := registry.Search(context.Background(), "pkg", SearchOpts{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("Search() listings = %#v, want 1 result", listings)
	}
	if got := skipped.searchCalls.Load(); got != 0 {
		t.Fatalf("skipped.searchCalls = %d, want 0", got)
	}
}

func TestMultiRegistrySearchReturnsHealthyResultsOnPartialFailure(t *testing.T) {
	t.Parallel()

	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "broken",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return nil, errors.New("upstream unavailable")
			},
		},
		&stubRegistrySource{
			name: "healthy",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return []Listing{{Slug: "pkg", Version: "1.2.3"}}, nil
			},
		},
	)

	listings, err := registry.Search(context.Background(), "pkg", SearchOpts{})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil when at least one source succeeds", err)
	}
	if len(listings) != 1 || listings[0].Slug != "pkg" {
		t.Fatalf("Search() listings = %#v, want healthy source result", listings)
	}
}

func TestMultiRegistrySearchReturnsCombinedErrorWhenAllSourcesFail(t *testing.T) {
	t.Parallel()

	errOne := errors.New("source one failed")
	errTwo := errors.New("source two failed")

	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "one",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return nil, errOne
			},
		},
		&stubRegistrySource{
			name: "two",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return nil, errTwo
			},
		},
	)

	listings, err := registry.Search(context.Background(), "pkg", SearchOpts{})
	if err == nil {
		t.Fatal("Search() error = nil, want combined error")
	}
	if len(listings) != 0 {
		t.Fatalf("Search() listings = %#v, want empty", listings)
	}
	if !errors.Is(err, errOne) || !errors.Is(err, errTwo) {
		t.Fatalf("Search() error = %v, want both source errors", err)
	}
}

func TestMultiRegistrySearchWithNoSourcesReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	registry := NewMultiRegistry(testLogger())
	listings, err := registry.Search(context.Background(), "pkg", SearchOpts{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if listings == nil {
		t.Fatal("Search() listings = nil, want empty slice")
	}
	if len(listings) != 0 {
		t.Fatalf("Search() listings = %#v, want empty", listings)
	}
}

func TestMultiRegistrySearchHonorsCancellationAfterPartialResults(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "healthy",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return []Listing{{Slug: "pkg"}}, nil
			},
		},
		&stubRegistrySource{
			name: "blocking",
			caps: SourceCaps{Search: true},
			searchFunc: func(ctx context.Context, _ string, _ SearchOpts) ([]Listing, error) {
				close(started)
				<-ctx.Done()
				return nil, ctx.Err()
			},
		},
	)

	resultCh := make(chan searchResult, 1)
	go func() {
		listings, err := registry.Search(ctx, "pkg", SearchOpts{})
		resultCh <- searchResult{listings: listings, err: err}
	}()

	waitForSignal(t, started)
	cancel()

	result := waitForSearchResult(t, resultCh)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("Search() error = %v, want context.Canceled", result.err)
	}
	if result.listings != nil {
		t.Fatalf("Search() listings = %#v, want nil on cancellation", result.listings)
	}
}

func TestMultiRegistryInfoResolvesHighestPrioritySource(t *testing.T) {
	t.Parallel()

	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "low",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "pkg", Version: "1.0.0"}}, nil
			},
		},
		&stubRegistrySource{
			name: "high",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "pkg", Version: "2.0.0"}}, nil
			},
		},
	)

	detail, err := registry.Info(context.Background(), "pkg")
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if detail.Version != "2.0.0" {
		t.Fatalf("Info() detail = %#v, want high-priority version", detail)
	}
	if detail.Source != "high" {
		t.Fatalf("Info() source = %q, want high", detail.Source)
	}
}

func TestMultiRegistryInfoHonorsCancellationAfterPartialResults(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "healthy",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "pkg", Version: "1.0.0"}}, nil
			},
		},
		&stubRegistrySource{
			name: "blocking",
			infoFunc: func(ctx context.Context, _ string) (*Detail, error) {
				close(started)
				<-ctx.Done()
				return nil, ctx.Err()
			},
		},
	)

	type infoResult struct {
		detail *Detail
		err    error
	}

	resultCh := make(chan infoResult, 1)
	go func() {
		detail, err := registry.Info(ctx, "pkg")
		resultCh <- infoResult{detail: detail, err: err}
	}()

	waitForSignal(t, started)
	cancel()

	select {
	case result := <-resultCh:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("Info() error = %v, want context.Canceled", result.err)
		}
		if result.detail != nil {
			t.Fatalf("Info() detail = %#v, want nil on cancellation", result.detail)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Info() result")
	}
}

func TestMultiRegistryDownloadDelegatesToResolvedSource(t *testing.T) {
	t.Parallel()

	low := &stubRegistrySource{
		name: "low",
		infoFunc: func(context.Context, string) (*Detail, error) {
			return nil, nil
		},
		downloadFunc: func(context.Context, string, DownloadOpts) (*DownloadResult, error) {
			return nil, errors.New("low source should not download")
		},
	}
	high := &stubRegistrySource{
		name: "high",
		infoFunc: func(context.Context, string) (*Detail, error) {
			return &Detail{Listing: Listing{Slug: "pkg"}}, nil
		},
		downloadFunc: func(_ context.Context, slug string, _ DownloadOpts) (*DownloadResult, error) {
			return &DownloadResult{
				Slug:   slug,
				Reader: io.NopCloser(strings.NewReader("archive")),
			}, nil
		},
	}

	registry := NewMultiRegistry(testLogger(), low, high)
	result, err := registry.Download(context.Background(), "pkg", DownloadOpts{Version: "1.2.3"})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if result == nil || result.Slug != "pkg" {
		t.Fatalf("Download() result = %#v, want slug pkg", result)
	}
	if got := low.downloadCalls.Load(); got != 0 {
		t.Fatalf("low.downloadCalls = %d, want 0", got)
	}
	if got := high.downloadCalls.Load(); got != 1 {
		t.Fatalf("high.downloadCalls = %d, want 1", got)
	}
	if err := result.Reader.Close(); err != nil {
		t.Fatalf("result.Reader.Close() error = %v", err)
	}
}

func TestMultiRegistryCheckUpdate(t *testing.T) {
	t.Parallel()

	makeRegistry := func(version string) *MultiRegistry {
		return NewMultiRegistry(testLogger(), &stubRegistrySource{
			name: "registry",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "pkg", Version: version, Source: "registry"}}, nil
			},
		})
	}

	t.Run("Should newer version available", func(t *testing.T) {
		info, err := makeRegistry("1.2.0").CheckUpdate(context.Background(), "pkg", "1.1.0")
		if err != nil {
			t.Fatalf("CheckUpdate() error = %v", err)
		}
		if !info.HasUpdate {
			t.Fatalf("CheckUpdate() = %#v, want HasUpdate true", info)
		}
	})

	t.Run("Should equal version", func(t *testing.T) {
		info, err := makeRegistry("1.2.0").CheckUpdate(context.Background(), "pkg", "1.2.0")
		if err != nil {
			t.Fatalf("CheckUpdate() error = %v", err)
		}
		if info.HasUpdate {
			t.Fatalf("CheckUpdate() = %#v, want HasUpdate false", info)
		}
	})
}

func TestMultiRegistryCloseClosesAllSources(t *testing.T) {
	t.Parallel()

	errOne := errors.New("close one")

	one := &stubRegistrySource{
		name:      "one",
		closeFunc: func() error { return errOne },
	}
	two := &stubRegistrySource{
		name:      "two",
		closeFunc: func() error { return nil },
	}

	registry := NewMultiRegistry(testLogger(), one, two)
	err := registry.Close()
	if !errors.Is(err, errOne) {
		t.Fatalf("Close() error = %v, want joined error containing %v", err, errOne)
	}
	if got := one.closeCalls.Load(); got != 1 {
		t.Fatalf("one.closeCalls = %d, want 1", got)
	}
	if got := two.closeCalls.Load(); got != 1 {
		t.Fatalf("two.closeCalls = %d, want 1", got)
	}
}

func TestMultiRegistryValidationAndFallbackErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should search respects canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := checkMultiRegistryContext(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("checkMultiRegistryContext(canceled) error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should info requires slug", func(t *testing.T) {
		_, err := NewMultiRegistry(testLogger(), &stubRegistrySource{name: "source"}).Info(context.Background(), " ")
		if err == nil {
			t.Fatal("Info(blank slug) error = nil, want non-nil")
		}
	})

	t.Run("Should info returns not found when no source resolves slug", func(t *testing.T) {
		_, err := NewMultiRegistry(
			testLogger(),
			&stubRegistrySource{name: "source"},
		).Info(context.Background(), "missing")
		if err == nil {
			t.Fatal("Info(missing) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("Info(missing) error = %v, want not found context", err)
		}
	})

	t.Run("Should download rejects nil result", func(t *testing.T) {
		registry := NewMultiRegistry(testLogger(), &stubRegistrySource{
			name: "source",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "pkg"}}, nil
			},
			downloadFunc: func(context.Context, string, DownloadOpts) (*DownloadResult, error) {
				return nil, nil
			},
		})

		_, err := registry.Download(context.Background(), "pkg", DownloadOpts{})
		if err == nil {
			t.Fatal("Download(nil result) error = nil, want non-nil")
		}
	})
}

func TestMultiRegistryHelperFunctions(t *testing.T) {
	t.Parallel()

	if got := sourceName(nil, 7); got != "source-7" {
		t.Fatalf("sourceName(nil, 7) = %q, want source-7", got)
	}
	if got := sourceName(&stubRegistrySource{}, 3); got != "source-3" {
		t.Fatalf("sourceName(blank, 3) = %q, want source-3", got)
	}
	if got := firstNonEmpty(" ", "", "value"); got != "value" {
		t.Fatalf("firstNonEmpty() = %q, want value", got)
	}
	if got := sourceIndex(
		[]Source{&stubRegistrySource{name: "one"}},
		&stubRegistrySource{name: "missing"},
	); got != -1 {
		t.Fatalf("sourceIndex(missing) = %d, want -1", got)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForStarts(t *testing.T, started <-chan string, want int) {
	t.Helper()

	for range want {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %d concurrent search starts", want)
		}
	}
}

func waitForSearchResult(t *testing.T, resultCh <-chan searchResult) searchResult {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Search() result")
		return searchResult{}
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal")
	}
}

func listingBySlug(t *testing.T, listings []Listing, slug string) Listing {
	t.Helper()

	for _, listing := range listings {
		if listing.Slug == slug {
			return listing
		}
	}
	t.Fatalf("listing %q not found in %#v", slug, listings)
	return Listing{}
}
