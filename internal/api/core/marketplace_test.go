package core_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	extensionpkg "github.com/compozy/agh/internal/extension"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	registrypkg "github.com/compozy/agh/internal/registry"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/skills"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
)

type marketplaceCatalogStub struct {
	browseFn     func(context.Context, marketplacepkg.Kind, string, int) (marketplacepkg.BrowseResult, error)
	browsePageFn func(context.Context, marketplacepkg.Kind, string, int, int) (marketplacepkg.BrowseResult, error)
	detailFn     func(context.Context, marketplacepkg.Kind, string) (*marketplacepkg.Entry, error)
	refreshFn    func(context.Context, ...marketplacepkg.Kind) (marketplacepkg.RefreshReport, error)
}

func (s marketplaceCatalogStub) Browse(
	ctx context.Context,
	kind marketplacepkg.Kind,
	query string,
	offset int,
	limit int,
) (marketplacepkg.BrowseResult, error) {
	if s.browsePageFn != nil {
		return s.browsePageFn(ctx, kind, query, offset, limit)
	}
	if s.browseFn != nil {
		return s.browseFn(ctx, kind, query, limit)
	}
	return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{}}, nil
}

func (s marketplaceCatalogStub) Detail(
	ctx context.Context,
	kind marketplacepkg.Kind,
	entryID string,
) (*marketplacepkg.Entry, error) {
	if s.detailFn != nil {
		return s.detailFn(ctx, kind, entryID)
	}
	return nil, marketplacepkg.ErrEntryNotFound
}

func (s marketplaceCatalogStub) ResolveExtensionInstall(
	ctx context.Context,
	installSlug string,
	version string,
) (*marketplacepkg.Entry, error) {
	return s.Detail(ctx, marketplacepkg.KindExtension, installSlug+"@"+version)
}

func (s marketplaceCatalogStub) ResolveSkillInstalls(
	ctx context.Context,
	installSlugs []string,
) ([]marketplacepkg.Entry, error) {
	result, err := s.Browse(ctx, marketplacepkg.KindSkill, "", 0, len(installSlugs))
	if err != nil {
		return nil, err
	}
	requested := make(map[string]struct{}, len(installSlugs))
	for _, slug := range installSlugs {
		requested[strings.TrimSpace(slug)] = struct{}{}
	}
	entries := make([]marketplacepkg.Entry, 0, len(result.Entries))
	for _, entry := range result.Entries {
		if _, ok := requested[entry.InstallSlug]; ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (s marketplaceCatalogStub) Refresh(
	ctx context.Context,
	kinds ...marketplacepkg.Kind,
) (marketplacepkg.RefreshReport, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx, kinds...)
	}
	return marketplacepkg.RefreshReport{Outcomes: []marketplacepkg.RefreshOutcome{}}, nil
}

func (marketplaceCatalogStub) Status(context.Context) ([]marketplacepkg.KindState, error) {
	return []marketplacepkg.KindState{}, nil
}

type installedSkillMarketplaceStub struct {
	items []skillmarketplace.InstalledSkill
	err   error
}

func (s installedSkillMarketplaceStub) ListInstalled(context.Context) ([]skillmarketplace.InstalledSkill, error) {
	return s.items, s.err
}

type marketplaceBundleServiceStub struct {
	catalog     []bundlepkg.CatalogEntry
	activations []bundlepkg.ActivationPreview
	catalogErr  error
}

func (s marketplaceBundleServiceStub) Catalog(context.Context) ([]bundlepkg.CatalogEntry, error) {
	return s.catalog, s.catalogErr
}

func (marketplaceBundleServiceStub) PreviewActivation(
	context.Context,
	bundlepkg.ActivateRequest,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, nil
}

func (marketplaceBundleServiceStub) Activate(
	context.Context,
	bundlepkg.ActivateRequest,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, nil
}

func (s marketplaceBundleServiceStub) ListActivations(context.Context) ([]bundlepkg.ActivationPreview, error) {
	return s.activations, nil
}

func (marketplaceBundleServiceStub) GetActivation(
	context.Context,
	string,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, nil
}

func (marketplaceBundleServiceStub) UpdateActivation(
	context.Context,
	bundlepkg.UpdateActivationRequest,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, nil
}

func (marketplaceBundleServiceStub) Deactivate(context.Context, string) error { return nil }

func (marketplaceBundleServiceStub) NetworkSettings(context.Context) (bundlepkg.NetworkSettings, error) {
	return bundlepkg.NetworkSettings{}, nil
}

func TestMarketplaceSearchPreservesKindIsolationAndInstalledTruth(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate kind failures and preserve installed truth", func(t *testing.T) {
		t.Parallel()

		remoteSearchCalls := 0
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			remoteSearch: func(context.Context, string, int) ([]registrypkg.Listing, error) {
				remoteSearchCalls++
				return nil, nil
			},
			bundleError: errors.New("bundle catalog unavailable"),
		})
		engine := gin.New()
		engine.GET("/marketplace/search", handlers.SearchMarketplace)

		response := performRequest(t, engine, http.MethodGet, "/marketplace/search", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.MarketplaceSearchResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if remoteSearchCalls != 0 {
			t.Fatalf("remote skill search calls = %d, want 0 for idle browse", remoteSearchCalls)
		}
		wantKinds := []string{"mcp", "extension", "skill", "bundle"}
		if len(payload.Kinds) != len(wantKinds) {
			t.Fatalf("len(kinds) = %d, want %d", len(payload.Kinds), len(wantKinds))
		}
		for index, want := range wantKinds {
			if payload.Kinds[index].Kind != want {
				t.Fatalf("kinds[%d].kind = %q, want %q", index, payload.Kinds[index].Kind, want)
			}
		}
		if got := payload.Kinds[3].Error; got != "bundle catalog unavailable" {
			t.Fatalf("bundle error = %q, want %q", got, "bundle catalog unavailable")
		}
		if len(payload.Kinds[0].Items) != 1 || !payload.Kinds[0].Items[0].Installed {
			t.Fatalf("MCP items = %#v, want installed catalog provenance join", payload.Kinds[0].Items)
		}
		if payload.Kinds[0].Items[0].InstalledVersion != "" || payload.Kinds[0].Items[0].UpdateAvailable {
			t.Fatalf("MCP update fields = %#v, want install-state only", payload.Kinds[0].Items[0])
		}
		for _, index := range []int{1, 2} {
			if len(payload.Kinds[index].Items) != 1 || !payload.Kinds[index].Items[0].Installed ||
				!payload.Kinds[index].Items[0].UpdateAvailable {
				t.Fatalf("kinds[%d].items = %#v, want truthful semver update join", index, payload.Kinds[index].Items)
			}
			if payload.Kinds[index].Items[0].InstallSlug == "" {
				t.Fatalf("kinds[%d].items[0].install_slug = empty, want acquisition identity", index)
			}
			if payload.Kinds[index].Total == nil || *payload.Kinds[index].Total != 1 {
				t.Fatalf("kinds[%d].total = %v, want exact curated total 1", index, payload.Kinds[index].Total)
			}
		}
		if got := payload.Kinds[2].Items[0].ManagePath; got != "/marketplace/skills?tab=installed" {
			t.Fatalf("skill manage path = %q, want %q", got, "/marketplace/skills?tab=installed")
		}
		if got := payload.Kinds[0].Items[0].ManagePath; got != "/marketplace/mcps?tab=installed" {
			t.Fatalf("mcp manage path = %q, want %q", got, "/marketplace/mcps?tab=installed")
		}
		for _, want := range []string{
			`"installed_name":"github"`,
			`"installed_name":"extension"`,
			`"installed_name":"skill"`,
		} {
			if !strings.Contains(response.Body.String(), want) {
				t.Fatalf("response body = %s, want canonical installed identity %s", response.Body.String(), want)
			}
		}
	})

	t.Run("Should mask isolated kind failures when internal error masking is enabled", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			bundleError: errors.New("database path /private/catalog.db unavailable"),
		})
		handlers.MaskInternalErrors = true
		response, err := handlers.MarketplaceSearch(t.Context(), core.MarketplaceSearchRequest{})
		if err != nil {
			t.Fatalf("MarketplaceSearch() error = %v", err)
		}
		if got := response.Kinds[3].Error; got != http.StatusText(http.StatusInternalServerError) {
			t.Fatalf("masked bundle error = %q, want %q", got, http.StatusText(http.StatusInternalServerError))
		}
	})

	t.Run("Should mask persisted stale diagnostics when internal error masking is enabled", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogBrowse: func(
				_ context.Context,
				kind marketplacepkg.Kind,
				_ string,
				_ int,
			) (marketplacepkg.BrowseResult, error) {
				entry := marketplaceEntriesForTest()[kind]
				return marketplacepkg.BrowseResult{
					Entries: []marketplacepkg.Entry{entry},
					State: marketplacepkg.KindState{
						Kind: kind, Stale: true, ErrorClass: "store",
						LastError: "database path /private/catalog.db unavailable",
					},
				}, nil
			},
		})
		handlers.MaskInternalErrors = true
		response, err := handlers.MarketplaceSearch(t.Context(), core.MarketplaceSearchRequest{})
		if err != nil {
			t.Fatalf("MarketplaceSearch() error = %v", err)
		}
		if got := response.Kinds[0].Error; got != http.StatusText(http.StatusInternalServerError) {
			t.Fatalf("persisted stale error = %q, want masked %q", got, http.StatusText(http.StatusInternalServerError))
		}
	})

	t.Run("Should include workspace identity in MCP manage links", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		response, err := handlers.MarketplaceSearch(t.Context(), core.MarketplaceSearchRequest{
			Scope: "workspace", WorkspaceID: "ws-a",
		})
		if err != nil {
			t.Fatalf("MarketplaceSearch(workspace) error = %v", err)
		}
		if got := response.Kinds[0].Items[0].ManagePath; got != "/marketplace/mcps?tab=installed" {
			t.Fatalf("workspace MCP manage path = %q", got)
		}
	})

	t.Run("Should omit continuation cursors from grouped search", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogBrowse: func(
				_ context.Context,
				kind marketplacepkg.Kind,
				_ string,
				_ int,
			) (marketplacepkg.BrowseResult, error) {
				entry := marketplaceEntriesForTest()[kind]
				return marketplacepkg.BrowseResult{
					Entries: []marketplacepkg.Entry{entry},
					Total:   2,
					State: marketplacepkg.KindState{
						Kind: kind, FetchedAt: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
					},
				}, nil
			},
		})
		response, err := handlers.MarketplaceSearch(t.Context(), core.MarketplaceSearchRequest{Limit: 1})
		if err != nil {
			t.Fatalf("MarketplaceSearch() error = %v", err)
		}
		for _, result := range response.Kinds {
			if result.NextCursor != "" {
				t.Fatalf("kind %q next_cursor = %q, want omitted for grouped search", result.Kind, result.NextCursor)
			}
		}
	})

	t.Run("Should page one kind with an opaque cursor bound to its filters and scope", func(t *testing.T) {
		t.Parallel()

		entries := []marketplacepkg.Entry{
			marketplaceEntriesForTest()[marketplacepkg.KindMCP],
			marketplaceEntriesForTest()[marketplacepkg.KindMCP],
		}
		entries[0].EntryID = "mcp-alpha"
		entries[0].Name = "Alpha"
		entries[0].Payload = json.RawMessage(
			`{"entry_id":"mcp-alpha","name":"Alpha","description":"Alpha MCP","transport":"stdio","command":"alpha"}`,
		)
		entries[1].EntryID = "mcp-beta"
		entries[1].Name = "Beta"
		entries[1].Payload = json.RawMessage(
			`{"entry_id":"mcp-beta","name":"Beta","description":"Beta MCP","transport":"stdio","command":"beta"}`,
		)
		offsets := make([]int, 0, 3)
		revision := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			MarketplaceCatalog: marketplaceCatalogStub{browsePageFn: func(
				_ context.Context,
				kind marketplacepkg.Kind,
				query string,
				offset int,
				limit int,
			) (marketplacepkg.BrowseResult, error) {
				if kind != marketplacepkg.KindMCP || query != "server" || limit != 1 {
					t.Fatalf("Browse() = (%q, %q, %d, %d), want MCP/server/offset/1", kind, query, offset, limit)
				}
				offsets = append(offsets, offset)
				return marketplacepkg.BrowseResult{
					Entries: entries[offset : offset+1], Total: len(entries),
					State: marketplacepkg.KindState{
						Kind: marketplacepkg.KindMCP, FetchedAt: revision, EntryCount: len(entries),
					},
				}, nil
			}},
			Settings: &stubSettingsService{ListCollectionFn: func(
				context.Context,
				settingspkg.CollectionRequest,
			) (settingspkg.CollectionEnvelope, error) {
				return settingspkg.CollectionEnvelope{MCPServers: []settingspkg.MCPServerItem{}}, nil
			}},
			Logger: testutil.DiscardLogger(),
		})

		first, err := handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "mcp", Query: " server ", Limit: 1, Scope: "workspace", WorkspaceID: "ws-a",
		})
		if err != nil {
			t.Fatalf("MarketplaceKind(first) error = %v", err)
		}
		if first.Total == nil || *first.Total != 2 || len(first.Items) != 1 ||
			first.Items[0].EntryID != "mcp-alpha" || first.NextCursor == "" {
			t.Fatalf("MarketplaceKind(first) = %#v, want first row, exact total, and cursor", first)
		}
		second, err := handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "mcp", Query: "server", Cursor: first.NextCursor, Limit: 1,
			Scope: "workspace", WorkspaceID: "ws-a",
		})
		if err != nil {
			t.Fatalf("MarketplaceKind(second) error = %v", err)
		}
		if len(second.Items) != 1 || second.Items[0].EntryID != "mcp-beta" || second.NextCursor != "" {
			t.Fatalf("MarketplaceKind(second) = %#v, want final non-overlapping row", second)
		}
		if !slices.Equal(offsets, []int{0, 1}) {
			t.Fatalf("Browse() offsets = %v, want [0 1]", offsets)
		}
		_, err = handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "mcp", Query: "different", Cursor: first.NextCursor, Limit: 1,
			Scope: "workspace", WorkspaceID: "ws-a",
		})
		if !errors.Is(err, core.ErrMarketplaceValidation) {
			t.Fatalf("MarketplaceKind(mismatched cursor) error = %v, want validation", err)
		}

		revision = revision.Add(time.Second)
		_, err = handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "mcp", Query: "server", Cursor: first.NextCursor, Limit: 1,
			Scope: "workspace", WorkspaceID: "ws-a",
		})
		if !errors.Is(err, core.ErrMarketplaceValidation) {
			t.Fatalf("MarketplaceKind(changed catalog revision) error = %v, want validation", err)
		}
	})

	t.Run("Should join an installed extension by stable catalog entry identity", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			extensionCatalogEntryID: "extension-entry",
			extensionInstalledSlug:  "",
		})
		response, err := handlers.MarketplaceSearch(t.Context(), core.MarketplaceSearchRequest{})
		if err != nil {
			t.Fatalf("MarketplaceSearch() error = %v", err)
		}
		if item := response.Kinds[1].Items[0]; !item.Installed || item.InstalledName != "extension" {
			t.Fatalf("extension item = %#v, want installed catalog identity join", item)
		}
	})

	t.Run("Should not satisfy a stable catalog ID lookup with an unrelated install slug", func(t *testing.T) {
		t.Parallel()

		entry := marketplaceEntriesForTest()[marketplacepkg.KindExtension]
		entry.EntryID = "acme/extension"
		entry.InstallSlug = "other/extension"
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			extensionCatalogEntryID: "different-entry",
			extensionInstalledSlug:  "acme/extension",
			catalogBrowse: func(
				_ context.Context,
				kind marketplacepkg.Kind,
				_ string,
				_ int,
			) (marketplacepkg.BrowseResult, error) {
				if kind == marketplacepkg.KindExtension {
					return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{entry}}, nil
				}
				fixtureEntry := marketplaceEntriesForTest()[kind]
				return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{fixtureEntry}}, nil
			},
		})
		response, err := handlers.MarketplaceSearch(t.Context(), core.MarketplaceSearchRequest{})
		if err != nil {
			t.Fatalf("MarketplaceSearch() error = %v", err)
		}
		if item := response.Kinds[1].Items[0]; item.Installed {
			t.Fatalf("extension item = %#v, want unrelated slug collision ignored", item)
		}
	})

	t.Run("Should expose stale feed state while serving durable rows", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogBrowse: func(
				_ context.Context,
				kind marketplacepkg.Kind,
				_ string,
				_ int,
			) (marketplacepkg.BrowseResult, error) {
				entries := marketplaceEntriesForTest()
				state := marketplacepkg.KindState{Kind: kind}
				if kind == marketplacepkg.KindMCP {
					state.Stale = true
					state.ErrorClass = "network"
					state.LastError = "catalog feed unavailable"
				}
				return marketplacepkg.BrowseResult{
					Entries: []marketplacepkg.Entry{entries[kind]},
					State:   state,
				}, nil
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/search", handlers.SearchMarketplace)
		engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)

		search := performRequest(t, engine, http.MethodGet, "/marketplace/search", nil)
		if search.Code != http.StatusOK {
			t.Fatalf("search status = %d, want %d; body=%s", search.Code, http.StatusOK, search.Body.String())
		}
		var searchPayload struct {
			Kinds []struct {
				Kind       string                               `json:"kind"`
				Stale      bool                                 `json:"stale"`
				ErrorClass string                               `json:"error_class"`
				Error      string                               `json:"error"`
				Items      []contract.MarketplaceListingPayload `json:"items"`
			} `json:"kinds"`
		}
		if err := json.Unmarshal(search.Body.Bytes(), &searchPayload); err != nil {
			t.Fatalf("json.Unmarshal(search) error = %v", err)
		}
		if len(searchPayload.Kinds) == 0 || !searchPayload.Kinds[0].Stale ||
			searchPayload.Kinds[0].ErrorClass != "network" ||
			searchPayload.Kinds[0].Error != "catalog feed unavailable" ||
			len(searchPayload.Kinds[0].Items) != 1 {
			t.Fatalf("stale search kind = %#v, want marked stale row with redacted diagnostic", searchPayload.Kinds)
		}

		browse := performRequest(t, engine, http.MethodGet, "/marketplace/mcp", nil)
		if browse.Code != http.StatusOK {
			t.Fatalf("browse status = %d, want %d; body=%s", browse.Code, http.StatusOK, browse.Body.String())
		}
		var browsePayload struct {
			Stale      bool                                 `json:"stale"`
			ErrorClass string                               `json:"error_class"`
			Error      string                               `json:"error"`
			Items      []contract.MarketplaceListingPayload `json:"items"`
		}
		if err := json.Unmarshal(browse.Body.Bytes(), &browsePayload); err != nil {
			t.Fatalf("json.Unmarshal(browse) error = %v", err)
		}
		if !browsePayload.Stale || browsePayload.ErrorClass != "network" ||
			browsePayload.Error != "catalog feed unavailable" || len(browsePayload.Items) != 1 {
			t.Fatalf("stale browse = %#v, want marked stale row with redacted diagnostic", browsePayload)
		}
	})
}

func TestMarketplaceContinuationRejectsChangedProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a remote skill continuation when its boundary changes", func(t *testing.T) {
		t.Parallel()

		listings := []registrypkg.Listing{
			{Slug: "@acme/alpha", Name: "Alpha", Version: "1.0.0", Source: "clawhub"},
			{Slug: "@acme/beta", Name: "Beta", Version: "1.0.0", Source: "clawhub"},
			{Slug: "@acme/gamma", Name: "Gamma", Version: "1.0.0", Source: "clawhub"},
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			MarketplaceCatalog: marketplaceCatalogStub{},
			SkillMarketplace: stubSkillMarketplaceService{SearchPageFn: func(
				_ context.Context,
				query string,
				offset int,
				limit int,
			) ([]registrypkg.Listing, error) {
				if query != "review" || offset != 0 {
					t.Fatalf("Search() = (%q, %d, %d), want review/0/limit", query, offset, limit)
				}
				return slices.Clone(listings[offset:min(offset+limit, len(listings))]), nil
			}},
			InstalledSkillMarketplace: installedSkillMarketplaceStub{items: []skillmarketplace.InstalledSkill{}},
			Logger:                    testutil.DiscardLogger(),
		})

		first, err := handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "skill", Query: "review", Limit: 1,
		})
		if err != nil {
			t.Fatalf("MarketplaceKind(first) error = %v", err)
		}
		if first.NextCursor == "" {
			t.Fatal("MarketplaceKind(first) next_cursor = empty, want continuation")
		}

		listings[0].Version = "2.0.0"
		_, err = handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "skill", Query: "review", Cursor: first.NextCursor, Limit: 1,
		})
		if !errors.Is(err, core.ErrMarketplaceValidation) {
			t.Fatalf("MarketplaceKind(changed remote boundary) error = %v, want validation", err)
		}
	})

	t.Run("Should page remote skills with bounded look-behind requests", func(t *testing.T) {
		t.Parallel()

		listings := []registrypkg.Listing{
			{Slug: "@acme/alpha", Name: "Alpha", Source: "clawhub"},
			{Slug: "@acme/beta", Name: "Beta", Source: "clawhub"},
			{Slug: "@acme/gamma", Name: "Gamma", Source: "clawhub"},
			{Slug: "@acme/delta", Name: "Delta", Source: "clawhub"},
		}
		type searchRequest struct {
			offset int
			limit  int
		}
		requests := make([]searchRequest, 0, 3)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			MarketplaceCatalog: marketplaceCatalogStub{},
			SkillMarketplace: stubSkillMarketplaceService{SearchPageFn: func(
				_ context.Context,
				query string,
				offset int,
				limit int,
			) ([]registrypkg.Listing, error) {
				if query != "review" {
					t.Fatalf("Search() query = %q, want review", query)
				}
				requests = append(requests, searchRequest{offset: offset, limit: limit})
				return slices.Clone(listings[offset:min(offset+limit, len(listings))]), nil
			}},
			InstalledSkillMarketplace: installedSkillMarketplaceStub{items: []skillmarketplace.InstalledSkill{}},
			Logger:                    testutil.DiscardLogger(),
		})

		cursor := ""
		gotSlugs := make([]string, 0, len(listings))
		for cursor != "" || len(gotSlugs) == 0 {
			page, err := handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
				Kind: "skill", Query: "review", Cursor: cursor, Limit: 1,
			})
			if err != nil {
				t.Fatalf("MarketplaceKind() error = %v", err)
			}
			if len(page.Items) != 1 {
				t.Fatalf("len(items) = %d, want 1", len(page.Items))
			}
			gotSlugs = append(gotSlugs, page.Items[0].Name)
			cursor = page.NextCursor
		}
		if diff := cmp.Diff([]string{"Alpha", "Beta", "Gamma", "Delta"}, gotSlugs); diff != "" {
			t.Fatalf("paged names mismatch (-want +got):\n%s", diff)
		}
		wantRequests := []searchRequest{
			{offset: 0, limit: 2},
			{offset: 0, limit: 3},
			{offset: 1, limit: 3},
			{offset: 2, limit: 3},
		}
		if diff := cmp.Diff(wantRequests, requests, cmp.AllowUnexported(searchRequest{})); diff != "" {
			t.Fatalf("Search() requests mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Should reject a bundle continuation when the catalog projection changes", func(t *testing.T) {
		t.Parallel()

		catalog := []bundlepkg.CatalogEntry{
			{ExtensionName: "acme", Bundle: extensionpkg.BundleSpec{Name: "Alpha"}},
			{ExtensionName: "acme", Bundle: extensionpkg.BundleSpec{Name: "Beta"}},
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Bundles: marketplaceBundleServiceStub{catalog: catalog},
			Logger:  testutil.DiscardLogger(),
		})

		first, err := handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "bundle", Limit: 1,
		})
		if err != nil {
			t.Fatalf("MarketplaceKind(first) error = %v", err)
		}
		if first.NextCursor == "" {
			t.Fatal("MarketplaceKind(first) next_cursor = empty, want continuation")
		}

		catalog[0].Bundle.Description = "Changed"
		_, err = handlers.MarketplaceKind(t.Context(), core.MarketplaceKindRequest{
			Kind: "bundle", Cursor: first.NextCursor, Limit: 1,
		})
		if !errors.Is(err, core.ErrMarketplaceValidation) {
			t.Fatalf("MarketplaceKind(changed bundle projection) error = %v, want validation", err)
		}
	})
}

func TestMarketplaceSearchUsesRemoteSkillsOnlyForQueries(t *testing.T) {
	t.Parallel()

	t.Run("Should use remote skill search only for non-empty queries", func(t *testing.T) {
		t.Parallel()

		remoteSearchCalls := 0
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			remoteSearch: func(_ context.Context, query string, limit int) ([]registrypkg.Listing, error) {
				remoteSearchCalls++
				if query != "review" || limit != 8 {
					t.Fatalf("remote search = (%q, %d), want (review, 8 sentinel page)", query, limit)
				}
				return nil, errors.New("clawhub unavailable")
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/search", handlers.SearchMarketplace)

		response := performRequest(t, engine, http.MethodGet, "/marketplace/search?q=review&limit=7", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.MarketplaceSearchResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if remoteSearchCalls != 1 {
			t.Fatalf("remote skill search calls = %d, want 1", remoteSearchCalls)
		}
		if got := payload.Kinds[2].Error; got != "clawhub unavailable" {
			t.Fatalf("skill error = %q, want %q", got, "clawhub unavailable")
		}
		if len(payload.Kinds[0].Items) != 1 || len(payload.Kinds[1].Items) != 1 {
			t.Fatalf("grouped response = %#v, want skill-only failure", payload.Kinds)
		}
	})
}

func TestMarketplaceNormalizesRemoteSkillErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should map remote search validation and availability errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			err        error
			wantStatus int
		}{
			{name: "Should map validation", err: skillmarketplace.ErrValidation, wantStatus: http.StatusBadRequest},
			{
				name: "Should map unavailable", err: skillmarketplace.ErrUnavailable,
				wantStatus: http.StatusServiceUnavailable,
			},
			{
				name: "Should map unconfigured", err: skillmarketplace.ErrNotConfigured,
				wantStatus: http.StatusServiceUnavailable,
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
					remoteSearch: func(context.Context, string, int) ([]registrypkg.Listing, error) {
						return nil, tc.err
					},
				})
				engine := gin.New()
				engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)
				response := performRequest(t, engine, http.MethodGet, "/marketplace/skill?q=remote", nil)
				if response.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
				}
				if !strings.Contains(response.Body.String(), tc.err.Error()) {
					t.Fatalf("body = %s, want normalized cause %q", response.Body.String(), tc.err.Error())
				}
			})
		}
	})

	t.Run("Should map a missing remote detail to not found", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			remoteInfo: func(context.Context, string) (*registrypkg.Detail, error) {
				return nil, skillmarketplace.ErrNotFound
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)
		entryID := "skill_" + base64.RawURLEncoding.EncodeToString([]byte("@acme/missing"))
		response := performRequest(t, engine, http.MethodGet, "/marketplace/skill/"+entryID, nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), skillmarketplace.ErrNotFound.Error()) {
			t.Fatalf("body = %s, want normalized not-found cause", response.Body.String())
		}
	})
}

func TestMarketplaceWorkspaceScopeDoesNotLeakBundleOrMCPInstallations(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate workspace-owned MCP and bundle installation state", func(t *testing.T) {
		t.Parallel()

		settingsRequests := make([]settingspkg.CollectionRequest, 0, 2)
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			settingsList: func(_ context.Context, req settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error) {
				settingsRequests = append(settingsRequests, req)
				items := []settingspkg.MCPServerItem{}
				if req.WorkspaceID == "ws-a" {
					items = append(items, settingspkg.MCPServerItem{Name: "github", CatalogEntry: "mcp-entry"})
				}
				return settingspkg.CollectionEnvelope{MCPServers: items}, nil
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/search", handlers.SearchMarketplace)

		wsA := performRequest(
			t, engine, http.MethodGet, "/marketplace/search?scope=workspace&workspace_id=ws-a", nil,
		)
		wsB := performRequest(
			t, engine, http.MethodGet, "/marketplace/search?scope=workspace&workspace_id=ws-b", nil,
		)
		for _, response := range []*httptest.ResponseRecorder{wsA, wsB} {
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
		}
		var payloadA, payloadB contract.MarketplaceSearchResponse
		if err := json.Unmarshal(wsA.Body.Bytes(), &payloadA); err != nil {
			t.Fatalf("json.Unmarshal(wsA) error = %v", err)
		}
		if err := json.Unmarshal(wsB.Body.Bytes(), &payloadB); err != nil {
			t.Fatalf("json.Unmarshal(wsB) error = %v", err)
		}
		if !payloadA.Kinds[0].Items[0].Installed || payloadB.Kinds[0].Items[0].Installed {
			t.Fatalf(
				"MCP installed A/B = %v/%v, want true/false",
				payloadA.Kinds[0].Items[0].Installed,
				payloadB.Kinds[0].Items[0].Installed,
			)
		}
		if got := payloadA.Kinds[0].Items[0].ManagePath; got != "/marketplace/mcps?tab=installed" {
			t.Fatalf(
				"workspace mcp manage path = %q, want %q",
				got,
				"/marketplace/mcps?tab=installed",
			)
		}
		if !payloadA.Kinds[3].Items[0].Installed || payloadB.Kinds[3].Items[0].Installed {
			t.Fatalf(
				"bundle installed A/B = %v/%v, want true/false",
				payloadA.Kinds[3].Items[0].Installed,
				payloadB.Kinds[3].Items[0].Installed,
			)
		}
		if len(settingsRequests) != 2 ||
			settingsRequests[0].WorkspaceID != "ws-a" ||
			settingsRequests[1].WorkspaceID != "ws-b" {
			t.Fatalf("settings requests = %#v, want isolated ws-a/ws-b reads", settingsRequests)
		}
	})
}

func TestMarketplaceSkillJoinRejectsNonSemverUpdateClaims(t *testing.T) {
	t.Parallel()

	t.Run("Should keep update_available false when the catalog version is not semver", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			skillCatalogVersion: "rolling",
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)

		response := performRequest(t, engine, http.MethodGet, "/marketplace/skill", nil)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusOK,
				response.Body.String(),
			)
		}
		var payload contract.MarketplaceKindResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(payload.Items) != 1 || !payload.Items[0].Installed || payload.Items[0].UpdateAvailable {
			t.Fatalf("skill items = %#v, want installed without an update claim", payload.Items)
		}
	})
}

func TestMarketplaceDetailAndRefreshValidateStableIdentityAndKind(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve stable IDs and reject unknown entries and refresh kinds", func(t *testing.T) {
		t.Parallel()

		workspaceActivationID := "workspace/activation?1"
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			bundleActivations: []bundlepkg.ActivationPreview{
				{Activation: bundlepkg.Activation{
					ID: "a-global", ExtensionName: "acme-extension", BundleName: "team",
					Scope: bundlepkg.ScopeGlobal,
				}},
				{Activation: bundlepkg.Activation{
					ID: workspaceActivationID, ExtensionName: "acme-extension", BundleName: "team",
					Scope: bundlepkg.ScopeWorkspace, WorkspaceID: "ws-a",
				}, SpecDrift: true},
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)
		engine.POST("/marketplace/refresh", handlers.RefreshMarketplaceCatalog)

		browse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace/bundle?scope=workspace&workspace_id=ws-a",
			nil,
		)
		if browse.Code != http.StatusOK {
			t.Fatalf("browse status = %d, want %d; body=%s", browse.Code, http.StatusOK, browse.Body.String())
		}
		var kind contract.MarketplaceKindResponse
		if err := json.Unmarshal(browse.Body.Bytes(), &kind); err != nil {
			t.Fatalf("json.Unmarshal(browse) error = %v", err)
		}
		if kind.Total == nil || *kind.Total != 1 || len(kind.Items) != 1 {
			t.Fatalf("bundle browse = %#v, want exact total and one row", kind)
		}
		if !kind.Items[0].Installed || !kind.Items[0].UpdateAvailable {
			t.Fatalf("bundle browse row = %#v, want installed drift projection", kind.Items[0])
		}
		wantBrowseManagePath := "/marketplace/bundles/activations/workspace%2Factivation%3F1"
		if kind.Items[0].ManagePath != wantBrowseManagePath {
			t.Fatalf("bundle browse manage path = %q, want %q", kind.Items[0].ManagePath, wantBrowseManagePath)
		}
		detail := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace/bundle/"+kind.Items[0].EntryID+"?scope=workspace&workspace_id=ws-a",
			nil,
		)
		if detail.Code != http.StatusOK {
			t.Fatalf("detail status = %d, want %d; body=%s", detail.Code, http.StatusOK, detail.Body.String())
		}
		var entry contract.MarketplaceEntryResponse
		if err := json.Unmarshal(detail.Body.Bytes(), &entry); err != nil {
			t.Fatalf("json.Unmarshal(detail) error = %v", err)
		}
		if !entry.Entry.Installed || !entry.Entry.UpdateAvailable {
			t.Fatalf("bundle detail row = %#v, want installed drift projection", entry.Entry)
		}
		wantDetailManagePath := "/marketplace/bundles/activations/workspace%2Factivation%3F1"
		if entry.Entry.ManagePath != wantDetailManagePath {
			t.Fatalf("bundle detail manage path = %q, want %q", entry.Entry.ManagePath, wantDetailManagePath)
		}
		if entry.Bundle == nil || len(entry.Bundle.Profiles) != 1 || entry.Bundle.Profiles[0].Layouts != 1 {
			t.Fatalf("bundle detail profiles = %#v, want one window layout", entry.Bundle)
		}
		missing := performRequest(t, engine, http.MethodGet, "/marketplace/mcp/missing", nil)
		if missing.Code != http.StatusNotFound {
			t.Fatalf("missing status = %d, want %d; body=%s", missing.Code, http.StatusNotFound, missing.Body.String())
		}
		unknownKind := performRequest(t, engine, http.MethodGet, "/marketplace/unknown", nil)
		if unknownKind.Code != http.StatusNotFound {
			t.Fatalf(
				"unknown kind status = %d, want %d; body=%s",
				unknownKind.Code,
				http.StatusNotFound,
				unknownKind.Body.String(),
			)
		}
		refresh := performRequest(t, engine, http.MethodPost, "/marketplace/refresh?kind=bundle", nil)
		if refresh.Code != http.StatusBadRequest {
			t.Fatalf(
				"bundle refresh status = %d, want %d; body=%s",
				refresh.Code,
				http.StatusBadRequest,
				refresh.Body.String(),
			)
		}
	})

	t.Run("Should return failed refresh outcomes that preserve the stale projection", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogRefresh: func(
				context.Context,
				...marketplacepkg.Kind,
			) (marketplacepkg.RefreshReport, error) {
				return marketplacepkg.RefreshReport{Outcomes: []marketplacepkg.RefreshOutcome{{
					Kind: marketplacepkg.KindMCP, Outcome: marketplacepkg.RefreshOutcomeFailed,
					EntryCount: 1, Stale: true, ErrorClass: "network",
				}}}, errors.New("catalog feed unavailable")
			},
		})
		engine := gin.New()
		engine.POST("/marketplace/refresh", handlers.RefreshMarketplaceCatalog)

		response := performRequest(t, engine, http.MethodPost, "/marketplace/refresh?kind=mcp", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.MarketplaceRefreshResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(payload.Kinds) != 1 || payload.Kinds[0].Kind != "mcp" ||
			payload.Kinds[0].Outcome != string(marketplacepkg.RefreshOutcomeFailed) ||
			payload.Kinds[0].EntryCount != 1 || !payload.Kinds[0].Stale ||
			payload.Kinds[0].ErrorClass != "network" {
			t.Fatalf("refresh payload = %#v, want stale failure report", payload)
		}
	})

	t.Run("Should resolve installed-only items through unified detail routes", func(t *testing.T) {
		t.Parallel()

		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			MarketplaceCatalog: marketplaceCatalogStub{detailFn: func(
				context.Context,
				marketplacepkg.Kind,
				string,
			) (*marketplacepkg.Entry, error) {
				return nil, marketplacepkg.ErrEntryNotFound
			}},
			Extensions: extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{{
					Name: "local-extension", Version: "1.4.0", Source: "local", Enabled: true,
				}}, nil
			}},
			Settings: &stubSettingsService{ListCollectionFn: func(
				context.Context,
				settingspkg.CollectionRequest,
			) (settingspkg.CollectionEnvelope, error) {
				return settingspkg.CollectionEnvelope{MCPServers: []settingspkg.MCPServerItem{{
					Name: "local-mcp", Transport: "http", URL: "https://mcp.example.test",
				}}}, nil
			}},
			SkillsRegistry: testutil.StubSkillsRegistry{ListFn: func() []*skills.Skill {
				return []*skills.Skill{{
					Meta: skills.SkillMeta{
						Name: "local-skill", Description: "Local skill", Version: "2.0.0",
					},
					Source: skills.SourceUser,
				}}
			}},
			Logger: testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)

		tests := []struct {
			name       string
			path       string
			kind       string
			entryName  string
			managePath string
		}{
			{
				name: "skill", path: "/marketplace/skill/local-skill",
				kind: contract.MarketplaceKindSkill, entryName: "local-skill",
				managePath: "/marketplace/skills?tab=installed",
			},
			{
				name: "extension", path: "/marketplace/extension/local-extension",
				kind: contract.MarketplaceKindExtension, entryName: "local-extension",
				managePath: "/marketplace/extensions?tab=installed",
			},
			{
				name: "mcp", path: "/marketplace/mcp/local-mcp",
				kind: contract.MarketplaceKindMCP, entryName: "local-mcp",
				managePath: "/marketplace/mcps?tab=installed",
			},
		}
		for _, test := range tests {
			t.Run("Should resolve installed "+test.name, func(t *testing.T) {
				t.Parallel()

				response := performRequest(t, engine, http.MethodGet, test.path, nil)
				if response.Code != http.StatusOK {
					t.Fatalf(
						"status = %d, want %d; body=%s",
						response.Code,
						http.StatusOK,
						response.Body.String(),
					)
				}
				var payload contract.MarketplaceEntryResponse
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.Entry.Kind != test.kind || payload.Entry.Name != test.entryName ||
					!payload.Entry.Installed || payload.Entry.ManagePath != test.managePath {
					t.Fatalf("installed detail = %#v, want kind/name/installed/manage path", payload.Entry)
				}
			})
		}
	})

	t.Run("Should resolve exact installed identity before a colliding curated entry", func(t *testing.T) {
		t.Parallel()

		curated := marketplaceEntriesForTest()[marketplacepkg.KindMCP]
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			MarketplaceCatalog: marketplaceCatalogStub{detailFn: func(
				context.Context,
				marketplacepkg.Kind,
				string,
			) (*marketplacepkg.Entry, error) {
				return &curated, nil
			}},
			Settings: &stubSettingsService{ListCollectionFn: func(
				context.Context,
				settingspkg.CollectionRequest,
			) (settingspkg.CollectionEnvelope, error) {
				return settingspkg.CollectionEnvelope{MCPServers: []settingspkg.MCPServerItem{{
					Name: "mcp-entry", Transport: "http", URL: "https://custom.example.test/mcp",
				}}}, nil
			}},
			Logger: testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)

		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace/mcp/mcp-entry?installed_name=mcp-entry",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.MarketplaceEntryResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if payload.Entry.Name != "mcp-entry" || payload.Entry.Source != "installed" ||
			payload.MCP == nil || payload.MCP.URL != "https://custom.example.test/mcp" {
			t.Fatalf("installed detail = %#v, want exact custom MCP rather than curated collision", payload)
		}
	})

	t.Run("Should resolve remote skill IDs without consulting the curated catalog", func(t *testing.T) {
		t.Parallel()

		catalogDetailCalls := 0
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			remoteSearch: func(context.Context, string, int) ([]registrypkg.Listing, error) {
				return []registrypkg.Listing{{
					Slug: "@acme/remote", Name: "Remote", Description: "Remote skill", Source: "clawhub",
				}}, nil
			},
			remoteInfo: func(_ context.Context, slug string) (*registrypkg.Detail, error) {
				return &registrypkg.Detail{Listing: registrypkg.Listing{
					Slug: slug, Name: "Remote", Description: "Remote skill", Source: "clawhub",
				}}, nil
			},
			catalogDetail: func(context.Context, marketplacepkg.Kind, string) (*marketplacepkg.Entry, error) {
				catalogDetailCalls++
				return nil, errors.New("catalog unavailable")
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)

		browse := performRequest(t, engine, http.MethodGet, "/marketplace/skill?q=remote", nil)
		if browse.Code != http.StatusOK {
			t.Fatalf("browse status = %d, want %d; body=%s", browse.Code, http.StatusOK, browse.Body.String())
		}
		var kind contract.MarketplaceKindResponse
		if err := json.Unmarshal(browse.Body.Bytes(), &kind); err != nil {
			t.Fatalf("json.Unmarshal(browse) error = %v", err)
		}
		if len(kind.Items) != 1 {
			t.Fatalf("remote browse items = %#v, want one", kind.Items)
		}
		detail := performRequest(
			t, engine, http.MethodGet, "/marketplace/skill/"+kind.Items[0].EntryID, nil,
		)
		if detail.Code != http.StatusOK {
			t.Fatalf("detail status = %d, want %d; body=%s", detail.Code, http.StatusOK, detail.Body.String())
		}
		if catalogDetailCalls != 0 {
			t.Fatalf("catalog detail calls = %d, want 0 for remote ID", catalogDetailCalls)
		}
	})

	t.Run("Should preserve curated skill identity across remote search", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			remoteSearch: func(context.Context, string, int) ([]registrypkg.Listing, error) {
				return []registrypkg.Listing{{
					Slug: "@acme/skill", Name: "Remote match", Description: "Remote skill", Source: "clawhub",
				}}, nil
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)

		response := performRequest(t, engine, http.MethodGet, "/marketplace/skill?q=remote", nil)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusOK,
				response.Body.String(),
			)
		}
		var kind contract.MarketplaceKindResponse
		if err := json.Unmarshal(response.Body.Bytes(), &kind); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(kind.Items) != 1 || kind.Items[0].EntryID != "skill-entry" {
			t.Fatalf("remote browse items = %#v, want curated entry_id skill-entry", kind.Items)
		}
	})

	t.Run("Should retain curated skill detail when remote enrichment fails", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			remoteInfo: func(context.Context, string) (*registrypkg.Detail, error) {
				return nil, errors.New("clawhub unavailable")
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)

		response := performRequest(t, engine, http.MethodGet, "/marketplace/skill/skill-entry", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var detail contract.MarketplaceEntryResponse
		if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if detail.Skill == nil || detail.Skill.InstallSlug != "@acme/skill" {
			t.Fatalf("curated skill detail = %#v, want catalog acquisition detail", detail.Skill)
		}
	})

	t.Run("Should expose daemon-derived extension trust on browse and detail", func(t *testing.T) {
		t.Parallel()

		trustCalls := 0
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			extensionTier: extensionpkg.ExtensionRegistryTierUnverified,
			extensionTrust: func(
				_ context.Context,
				evidence extensionpkg.MarketplaceTrustEvidence,
			) (contract.ExtensionTrustReportPayload, error) {
				trustCalls++
				if evidence.CatalogEntryID != "extension-entry" ||
					evidence.Version != "1.2.0" ||
					evidence.ArchiveDigestSHA256 != strings.Repeat("a", 64) ||
					evidence.RegistryTier != extensionpkg.ExtensionRegistryTierUnverified {
					t.Fatalf("MarketplaceTrust() evidence = %#v", evidence)
				}
				return extensionpkg.MarketplaceEntryTrustReport(evidence, true)
			},
		})
		engine := gin.New()
		engine.GET("/marketplace/:kind", handlers.BrowseMarketplaceKind)
		engine.GET("/marketplace/:kind/:entry_id", handlers.GetMarketplaceEntry)

		browse := performRequest(t, engine, http.MethodGet, "/marketplace/extension", nil)
		if browse.Code != http.StatusOK {
			t.Fatalf("browse status = %d, want %d; body=%s", browse.Code, http.StatusOK, browse.Body.String())
		}
		var kind contract.MarketplaceKindResponse
		if err := json.Unmarshal(browse.Body.Bytes(), &kind); err != nil {
			t.Fatalf("json.Unmarshal(browse) error = %v", err)
		}
		if len(kind.Items) != 1 || kind.Items[0].Trust == nil ||
			kind.Items[0].Trust.Decision != extensionpkg.ExtensionTrustDecisionAllowedUnverified ||
			len(kind.Items[0].Trust.Warnings) != 1 {
			t.Fatalf("extension browse trust = %#v", kind.Items)
		}

		detailResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace/extension/extension-entry",
			nil,
		)
		if detailResponse.Code != http.StatusOK {
			t.Fatalf(
				"detail status = %d, want %d; body=%s",
				detailResponse.Code,
				http.StatusOK,
				detailResponse.Body.String(),
			)
		}
		var detail contract.MarketplaceEntryResponse
		if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
			t.Fatalf("json.Unmarshal(detail) error = %v", err)
		}
		if detail.Entry.Trust == nil ||
			detail.Entry.Trust.Decision != extensionpkg.ExtensionTrustDecisionAllowedUnverified ||
			detail.Entry.Trust.ChecksumVerified || !detail.Entry.Trust.AllowUnverified {
			t.Fatalf("extension detail trust = %#v", detail.Entry.Trust)
		}
		if trustCalls != 2 {
			t.Fatalf("MarketplaceTrust() calls = %d, want 2", trustCalls)
		}
	})
}

type marketplaceHandlerFixture struct {
	catalogBrowse           func(context.Context, marketplacepkg.Kind, string, int) (marketplacepkg.BrowseResult, error)
	catalogRefresh          func(context.Context, ...marketplacepkg.Kind) (marketplacepkg.RefreshReport, error)
	remoteSearch            func(context.Context, string, int) ([]registrypkg.Listing, error)
	remoteInfo              func(context.Context, string) (*registrypkg.Detail, error)
	catalogDetail           func(context.Context, marketplacepkg.Kind, string) (*marketplacepkg.Entry, error)
	settingsList            func(context.Context, settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error)
	bundleError             error
	bundleSpecDrift         bool
	skillCatalogVersion     string
	installedSkillVersion   string
	extensionTier           string
	extensionCatalogEntryID string
	extensionInstalledSlug  string
	bundleActivations       []bundlepkg.ActivationPreview
	extensionTrust          func(
		context.Context,
		extensionpkg.MarketplaceTrustEvidence,
	) (contract.ExtensionTrustReportPayload, error)
}

func marketplaceHandlersForTest(t *testing.T, fixture marketplaceHandlerFixture) *core.BaseHandlers {
	t.Helper()

	entries := marketplaceEntriesForTest()
	if fixture.extensionTier != "" {
		entry := entries[marketplacepkg.KindExtension]
		entry.Tier = fixture.extensionTier
		entries[marketplacepkg.KindExtension] = entry
	}
	if fixture.skillCatalogVersion != "" {
		entry := entries[marketplacepkg.KindSkill]
		entry.Version = fixture.skillCatalogVersion
		entries[marketplacepkg.KindSkill] = entry
	}
	catalogDetail := fixture.catalogDetail
	if catalogDetail == nil {
		catalogDetail = func(_ context.Context, kind marketplacepkg.Kind, entryID string) (*marketplacepkg.Entry, error) {
			entry, ok := entries[kind]
			if !ok || entry.EntryID != entryID {
				return nil, marketplacepkg.ErrEntryNotFound
			}
			return &entry, nil
		}
	}
	catalogBrowse := fixture.catalogBrowse
	if catalogBrowse == nil {
		catalogBrowse = func(
			_ context.Context,
			kind marketplacepkg.Kind,
			_ string,
			_ int,
		) (marketplacepkg.BrowseResult, error) {
			return marketplacepkg.BrowseResult{
				Entries: []marketplacepkg.Entry{entries[kind]}, Total: 1,
			}, nil
		}
	}
	catalog := marketplaceCatalogStub{
		browseFn: catalogBrowse, detailFn: catalogDetail, refreshFn: fixture.catalogRefresh,
	}
	settingsList := fixture.settingsList
	if settingsList == nil {
		settingsList = func(context.Context, settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error) {
			return settingspkg.CollectionEnvelope{MCPServers: []settingspkg.MCPServerItem{{
				Name: "github", CatalogEntry: "mcp-entry", CatalogVersion: "1.0.0",
			}}}, nil
		}
	}
	remoteSearch := fixture.remoteSearch
	if remoteSearch == nil {
		remoteSearch = func(context.Context, string, int) ([]registrypkg.Listing, error) {
			return []registrypkg.Listing{}, nil
		}
	}
	installedSkillVersion := fixture.installedSkillVersion
	if installedSkillVersion == "" {
		installedSkillVersion = "1.0.0"
	}
	homePaths := testutil.NewTestHomePaths(t)
	config := testConfigWithDisabledNetwork(homePaths)
	activations := fixture.bundleActivations
	if activations == nil {
		activations = []bundlepkg.ActivationPreview{{
			Activation: bundlepkg.Activation{
				ID:            "activation-ws-a",
				ExtensionName: "acme-extension",
				BundleName:    "team",
				Scope:         bundlepkg.ScopeWorkspace,
				WorkspaceID:   "ws-a",
			},
			SpecDrift: fixture.bundleSpecDrift,
		}}
	}
	installedSlug := fixture.extensionInstalledSlug
	if installedSlug == "" && fixture.extensionCatalogEntryID == "" {
		installedSlug = "acme/extension"
	}
	bundles := marketplaceBundleServiceStub{
		catalogErr: fixture.bundleError,
		catalog: []bundlepkg.CatalogEntry{{
			ExtensionName: "acme-extension",
			Bundle: extensionpkg.BundleSpec{
				Name:        "team",
				Description: "Team bundle",
				Profiles: []extensionpkg.BundleProfile{{
					Name:    "default",
					Layouts: []extensionpkg.BundleLayout{{Path: "layouts/team.json"}},
				}},
			},
		}},
		activations: activations,
	}
	return core.NewBaseHandlers(&core.BaseHandlerConfig{
		MarketplaceCatalog: catalog,
		Settings:           &stubSettingsService{ListCollectionFn: settingsList},
		Extensions: extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
			return []contract.ExtensionPayload{{
				Name: "extension", Version: "1.0.0",
				Provenance: &contract.ExtensionProvenancePayload{
					Slug: installedSlug, CatalogEntryID: fixture.extensionCatalogEntryID,
				},
			}}, nil
		}, marketplaceTrustFn: fixture.extensionTrust},
		SkillMarketplace: stubSkillMarketplaceService{SearchFn: remoteSearch, InfoFn: fixture.remoteInfo},
		InstalledSkillMarketplace: installedSkillMarketplaceStub{items: []skillmarketplace.InstalledSkill{{
			Name: "skill", Provenance: skills.Provenance{Slug: "@acme/skill", Version: installedSkillVersion},
		}}},
		Bundles:   bundles,
		HomePaths: homePaths,
		Config:    config,
		Logger:    testutil.DiscardLogger(),
	})
}

func marketplaceEntriesForTest() map[marketplacepkg.Kind]marketplacepkg.Entry {
	return map[marketplacepkg.Kind]marketplacepkg.Entry{
		marketplacepkg.KindMCP: {
			Kind:        marketplacepkg.KindMCP,
			EntryID:     "mcp-entry",
			Name:        "GitHub",
			Description: "GitHub MCP",
			Version:     "2.0.0",
			Payload: json.RawMessage(
				`{"entry_id":"mcp-entry","name":"GitHub","description":"GitHub MCP","version":"2.0.0","transport":"stdio","command":"github"}`,
			),
		},
		marketplacepkg.KindExtension: {
			Kind:         marketplacepkg.KindExtension,
			EntryID:      "extension-entry",
			Name:         "Extension",
			Description:  "Extension",
			Version:      "1.2.0",
			InstallSlug:  "acme/extension",
			DigestSHA256: strings.Repeat("a", 64),
			Tier:         extensionpkg.ExtensionRegistryTierOfficial,
			Payload: json.RawMessage(
				`{"entry_id":"extension-entry","name":"Extension","description":"Extension","version":"1.2.0","install_slug":"acme/extension","artifact_url":"https://downloads.example.test/extension-v1.2.0.tar.gz","digest_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
			),
		},
		marketplacepkg.KindSkill: {
			Kind:        marketplacepkg.KindSkill,
			EntryID:     "skill-entry",
			Name:        "Skill",
			Description: "Skill",
			Version:     "1.2.0",
			InstallSlug: "@acme/skill",
			Payload: json.RawMessage(
				`{"entry_id":"skill-entry","name":"Skill","description":"Skill","version":"1.2.0","install_slug":"@acme/skill"}`,
			),
		},
	}
}
