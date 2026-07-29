//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

type extensionSearchCatalogStub struct {
	marketplacepkg.Service
	browseFn func(context.Context, marketplacepkg.Kind, string, int, int) (marketplacepkg.BrowseResult, error)
}

func (s extensionSearchCatalogStub) Browse(
	ctx context.Context,
	kind marketplacepkg.Kind,
	query string,
	offset int,
	limit int,
) (marketplacepkg.BrowseResult, error) {
	return s.browseFn(ctx, kind, query, offset, limit)
}

type extensionSearchSourceStub struct {
	searchFn func(context.Context, string, registrypkg.SearchOpts) ([]registrypkg.Listing, error)
	closed   atomic.Int32
}

func (*extensionSearchSourceStub) Name() string { return extensionSearchSourceGitHub }
func (*extensionSearchSourceStub) Capabilities() registrypkg.SourceCaps {
	return registrypkg.SourceCaps{Search: true}
}
func (s *extensionSearchSourceStub) Search(
	ctx context.Context,
	query string,
	opts registrypkg.SearchOpts,
) ([]registrypkg.Listing, error) {
	return s.searchFn(ctx, query, opts)
}
func (*extensionSearchSourceStub) Info(context.Context, string) (*registrypkg.Detail, error) {
	return nil, registrypkg.ErrNotSupported
}
func (*extensionSearchSourceStub) Download(
	context.Context,
	string,
	registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	return nil, registrypkg.ErrNotSupported
}
func (s *extensionSearchSourceStub) Close() error {
	s.closed.Add(1)
	return nil
}

func TestExtensionSearchUnion(t *testing.T) {
	t.Parallel()

	t.Run("Should project an installed update without discovery flags", func(t *testing.T) {
		t.Parallel()

		_, registry, _, _ := newNativeExtensionToolDeps(t)
		fixtureDir := writeNativeLocalExtensionFixture(t, "installed-a", "1.0.0")
		manifest, err := extensionpkg.LoadManifest(fixtureDir)
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}
		checksum, err := extensionpkg.ComputeDirectoryChecksum(fixtureDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
		}
		if err := registry.Install(
			manifest,
			fixtureDir,
			checksum,
			extensionpkg.WithInstallSource(extensionpkg.SourceMarketplace),
			extensionpkg.WithInstallRegistryMetadata("acme/a", "curated", "1.0.0"),
		); err != nil {
			t.Fatalf("Registry.Install() error = %v", err)
		}

		entry := curatedSearchEntry("curated-a", "acme/a", "A bridge")
		entry.Version = "2.0.0"
		catalog := extensionSearchCatalogStub{browseFn: func(
			context.Context,
			marketplacepkg.Kind,
			string,
			int,
			int,
		) (marketplacepkg.BrowseResult, error) {
			return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{entry}}, nil
		}}
		github := &extensionSearchSourceStub{searchFn: func(
			context.Context,
			string,
			registrypkg.SearchOpts,
		) ([]registrypkg.Listing, error) {
			return nil, nil
		}}
		service := newExtensionSearchTestService(catalog, github)
		service.registry = registry

		result, err := service.Search(t.Context(), contract.ExtensionSearchRequest{
			Query: "bridge", Sources: []string{"curated"}, Limit: 20,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Items) != 1 || !result.Items[0].Installed ||
			result.Items[0].InstalledVersion != "1.0.0" || !result.Items[0].UpdateAvailable {
			t.Fatalf("Search() = %#v, want installed 1.0.0 update to 2.0.0", result)
		}
	})

	t.Run("Should merge source-tagged results and preserve a stable cached cursor", func(t *testing.T) {
		t.Parallel()

		var curatedCalls atomic.Int32
		var githubCalls atomic.Int32
		catalog := extensionSearchCatalogStub{browseFn: func(
			_ context.Context,
			kind marketplacepkg.Kind,
			query string,
			offset int,
			limit int,
		) (marketplacepkg.BrowseResult, error) {
			curatedCalls.Add(1)
			if kind != marketplacepkg.KindExtension || query != "bridge" || offset != 0 || limit != 100 {
				t.Fatalf("Browse() = (%q, %q, %d, %d), want extension/bridge/0/100", kind, query, offset, limit)
			}
			return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{
				curatedSearchEntry("curated-z", "acme/z", "Z bridge"),
				curatedSearchEntry("curated-a", "acme/a", "A bridge"),
			}}, nil
		}}
		github := &extensionSearchSourceStub{searchFn: func(
			_ context.Context,
			query string,
			opts registrypkg.SearchOpts,
		) ([]registrypkg.Listing, error) {
			githubCalls.Add(1)
			if query != "bridge" || opts.Limit != 100 || opts.Type != registrypkg.PackageTypeExtension {
				t.Fatalf("Search() = (%q, %#v), want bridge extension snapshot", query, opts)
			}
			return []registrypkg.Listing{{
				Slug: "acme/00-github", Name: "GitHub bridge", Source: "github",
				Type: registrypkg.PackageTypeExtension,
			}}, nil
		}}
		service := newExtensionSearchTestService(catalog, github)

		first, err := service.Search(t.Context(), contract.ExtensionSearchRequest{Query: " Bridge ", Limit: 1})
		if err != nil {
			t.Fatalf("Search(first) error = %v", err)
		}
		if len(first.Items) != 1 || first.Items[0].Slug != "acme/a" || first.Items[0].Source != "curated" ||
			first.Items[0].Tier != "official" || first.Items[0].Integrity != "catalog_digest" ||
			first.Items[0].DigestMatched {
			t.Fatalf("Search(first) = %#v, want curated integrity-only result", first)
		}
		if first.NextCursor == "" {
			t.Fatal("Search(first) next_cursor empty, want stable continuation")
		}
		second, err := service.Search(t.Context(), contract.ExtensionSearchRequest{
			Query: "bridge", Limit: 1, Cursor: first.NextCursor,
		})
		if err != nil {
			t.Fatalf("Search(second) error = %v", err)
		}
		if len(second.Items) != 1 || second.Items[0].Slug != "acme/z" {
			t.Fatalf("Search(second) = %#v, want second curated snapshot item", second)
		}
		if curatedCalls.Load() != 1 || githubCalls.Load() != 1 || github.closed.Load() != 1 {
			t.Fatalf(
				"source calls curated=%d github=%d close=%d, want one cached fetch",
				curatedCalls.Load(), githubCalls.Load(), github.closed.Load(),
			)
		}
	})

	t.Run("Should return a healthy source and mark a failed source degraded", func(t *testing.T) {
		t.Parallel()

		catalog := extensionSearchCatalogStub{browseFn: func(
			context.Context,
			marketplacepkg.Kind,
			string,
			int,
			int,
		) (marketplacepkg.BrowseResult, error) {
			return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{
				curatedSearchEntry("curated-a", "acme/a", "A bridge"),
			}}, nil
		}}
		github := &extensionSearchSourceStub{searchFn: func(
			context.Context,
			string,
			registrypkg.SearchOpts,
		) ([]registrypkg.Listing, error) {
			return nil, errors.New("upstream unavailable")
		}}
		result, err := newExtensionSearchTestService(catalog, github).Search(
			t.Context(),
			contract.ExtensionSearchRequest{Query: "bridge", Limit: 20},
		)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Items) != 1 || len(result.SourcesDegraded) != 1 ||
			result.SourcesDegraded[0] != extensionSearchSourceGitHub {
			t.Fatalf("Search() = %#v, want curated result with github degraded", result)
		}
	})

	t.Run("Should omit results and mark every source degraded when all sources fail", func(t *testing.T) {
		t.Parallel()

		catalog := extensionSearchCatalogStub{browseFn: func(
			context.Context,
			marketplacepkg.Kind,
			string,
			int,
			int,
		) (marketplacepkg.BrowseResult, error) {
			return marketplacepkg.BrowseResult{}, errors.New("curated unavailable")
		}}
		github := &extensionSearchSourceStub{searchFn: func(
			context.Context,
			string,
			registrypkg.SearchOpts,
		) ([]registrypkg.Listing, error) {
			return nil, errors.New("github unavailable")
		}}
		result, err := newExtensionSearchTestService(catalog, github).Search(
			t.Context(),
			contract.ExtensionSearchRequest{Query: "bridge", Limit: 20},
		)
		if err != nil {
			t.Fatalf("Search() error = %v, want non-blocking degraded result", err)
		}
		if len(result.Items) != 0 || !slices.Equal(
			result.SourcesDegraded,
			[]string{extensionSearchSourceCurated, extensionSearchSourceGitHub},
		) {
			t.Fatalf("Search() = %#v, want empty results with both sources degraded", result)
		}
	})

	t.Run("Should reject a cursor when source filters change", func(t *testing.T) {
		t.Parallel()

		catalog := extensionSearchCatalogStub{browseFn: func(
			context.Context,
			marketplacepkg.Kind,
			string,
			int,
			int,
		) (marketplacepkg.BrowseResult, error) {
			return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{
				curatedSearchEntry("a", "acme/a", "A"), curatedSearchEntry("b", "acme/b", "B"),
			}}, nil
		}}
		github := &extensionSearchSourceStub{searchFn: func(
			context.Context,
			string,
			registrypkg.SearchOpts,
		) ([]registrypkg.Listing, error) {
			return nil, nil
		}}
		service := newExtensionSearchTestService(catalog, github)
		first, err := service.Search(t.Context(), contract.ExtensionSearchRequest{
			Query: "x", Sources: []string{"curated"}, Limit: 1,
		})
		if err != nil {
			t.Fatalf("Search(first) error = %v", err)
		}
		_, err = service.Search(t.Context(), contract.ExtensionSearchRequest{
			Query: "x", Sources: []string{"github"}, Limit: 1, Cursor: first.NextCursor,
		})
		if !errors.Is(err, extensionpkg.ErrExtensionSearchInvalid) {
			t.Fatalf("Search(changed sources) error = %v, want ErrExtensionSearchInvalid", err)
		}
	})
}

func newExtensionSearchTestService(
	catalog marketplacepkg.Service,
	github registrypkg.Source,
) *daemonExtensionService {
	return &daemonExtensionService{
		registry:           &extensionpkg.Registry{},
		marketplaceCatalog: catalog,
		marketplaceLoader: func(context.Context, compozyconfig.ExtensionsConfig) ([]registrypkg.Source, error) {
			return []registrypkg.Source{github}, nil
		},
		now: func() time.Time { return time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC) },
	}
}

func curatedSearchEntry(entryID string, slug string, name string) marketplacepkg.Entry {
	payload, err := json.Marshal(map[string]string{
		"author": "Compozy", "install_slug": slug, "artifact_url": "https://example.test/" + slug,
		"digest_sha256": strings.Repeat("a", 64),
	})
	if err != nil {
		panic(err)
	}
	return marketplacepkg.Entry{
		Kind: marketplacepkg.KindExtension, EntryID: entryID, Name: name, Description: name,
		Version: "1.0.0", DigestSHA256: strings.Repeat("a", 64), Tier: "official",
		InstallSlug: slug, Payload: payload,
	}
}

var _ io.Closer = (*extensionSearchSourceStub)(nil)
