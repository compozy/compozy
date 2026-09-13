package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
)

type marketplaceInspectionService struct {
	extensionServiceStub
	inspectFn func(context.Context, marketplacepkg.Entry, string) (contract.MarketplaceExtensionDetailPayload, error)
}

func (s marketplaceInspectionService) InspectCatalogExtension(
	ctx context.Context, entry marketplacepkg.Entry, profile string,
) (contract.MarketplaceExtensionDetailPayload, error) {
	return s.inspectFn(ctx, entry, profile)
}

type marketplaceCatalogStub struct {
	browseFn     func(context.Context, string, int) (marketplacepkg.BrowseResult, error)
	browsePageFn func(context.Context, string, int, int) (marketplacepkg.BrowseResult, error)
	detailFn     func(context.Context, string) (*marketplacepkg.Entry, error)
	refreshFn    func(context.Context) (marketplacepkg.RefreshReport, error)
}

func (s marketplaceCatalogStub) Browse(
	ctx context.Context,
	query string,
	offset int,
	limit int,
) (marketplacepkg.BrowseResult, error) {
	if s.browsePageFn != nil {
		return s.browsePageFn(ctx, query, offset, limit)
	}
	if s.browseFn != nil {
		return s.browseFn(ctx, query, limit)
	}
	return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{}}, nil
}

func (s marketplaceCatalogStub) Detail(
	ctx context.Context,
	_ string, entryID string,
) (*marketplacepkg.Entry, error) {
	if s.detailFn != nil {
		return s.detailFn(ctx, entryID)
	}
	return nil, marketplacepkg.ErrEntryNotFound
}

func (s marketplaceCatalogStub) Entry(
	ctx context.Context,
	origin marketplacepkg.Origin,
) (*marketplacepkg.Entry, error) {
	if origin.SourceRef != marketplacepkg.CompozyCatalogRef {
		return nil, marketplacepkg.ErrEntryNotFound
	}
	return s.Detail(ctx, marketplacepkg.CompozyCatalogSource, origin.EntryID)
}

func (s marketplaceCatalogStub) ResolveExtensionInstall(
	ctx context.Context,
	installSlug string,
	version string,
) (*marketplacepkg.Entry, error) {
	return s.Detail(ctx, marketplacepkg.CompozyCatalogSource, installSlug+"@"+version)
}

func (s marketplaceCatalogStub) Refresh(
	ctx context.Context, _ ...string,
) (marketplacepkg.RefreshReport, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx)
	}
	return marketplacepkg.RefreshReport{Outcomes: []marketplacepkg.RefreshOutcome{}}, nil
}

func (marketplaceCatalogStub) Status(context.Context) ([]marketplacepkg.SourceState, error) {
	return []marketplacepkg.SourceState{}, nil
}

func marketplaceListHTTP(t *testing.T, handlers *core.BaseHandlers) contract.MarketplaceListResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/api/marketplace", http.NoBody,
	)
	handlers.ListMarketplace(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"GET /api/marketplace status = %d, want %d; body=%s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}
	var response contract.MarketplaceListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode GET /api/marketplace response error = %v; body=%s", err, recorder.Body.String())
	}
	return response
}

type marketplaceHandlerFixture struct {
	catalogBrowse            func(context.Context, string, int) (marketplacepkg.BrowseResult, error)
	catalogRefresh           func(context.Context) (marketplacepkg.RefreshReport, error)
	catalogDetail            func(context.Context, string) (*marketplacepkg.Entry, error)
	extensionTier            string
	extensionCatalogEntryID  string
	extensionInstalledSlug   string
	extensionInstalledFormat string
	extensionTrust           func(
		context.Context,
		extensionpkg.MarketplaceTrustEvidence,
	) (contract.ExtensionTrustReportPayload, error)
}

func marketplaceHandlersForTest(t *testing.T, fixture marketplaceHandlerFixture) *core.BaseHandlers {
	t.Helper()

	entry := marketplaceEntryForTest()
	if fixture.extensionTier != "" {
		entry.Tier = fixture.extensionTier
	}

	catalogDetail := fixture.catalogDetail
	if catalogDetail == nil {
		catalogDetail = func(_ context.Context, entryID string) (*marketplacepkg.Entry, error) {
			if entry.EntryID != entryID {
				return nil, marketplacepkg.ErrEntryNotFound
			}
			return &entry, nil
		}
	}
	catalogBrowse := fixture.catalogBrowse
	if catalogBrowse == nil {
		catalogBrowse = func(
			_ context.Context,
			_ string,
			_ int,
		) (marketplacepkg.BrowseResult, error) {
			return marketplacepkg.BrowseResult{
				Entries: []marketplacepkg.Entry{entry}, Total: 1,
			}, nil
		}
	}
	catalog := marketplaceCatalogStub{
		browseFn: catalogBrowse, detailFn: catalogDetail, refreshFn: fixture.catalogRefresh,
	}
	homePaths := testutil.NewTestHomePaths(t)
	config := testConfigWithDisabledNetwork(homePaths)
	installedSlug := fixture.extensionInstalledSlug
	if installedSlug == "" && fixture.extensionCatalogEntryID == "" {
		installedSlug = "acme/extension"
	}
	// The fixture represents an installation with persisted acquisition evidence.
	// Origin is explicit; tests for unclassified records construct a payload without it.
	originEntryID := fixture.extensionCatalogEntryID
	if originEntryID == "" && installedSlug == "acme/extension" {
		originEntryID = "extension-entry"
	}
	var origin *contract.MarketplaceOriginPayload
	if originEntryID != "" {
		origin = &contract.MarketplaceOriginPayload{Source: marketplacepkg.CompozyCatalogSource,
			SourceRef: marketplacepkg.CompozyCatalogRef, EntryID: originEntryID}
	}
	return core.NewBaseHandlers(&core.BaseHandlerConfig{
		MarketplaceCatalog: catalog,
		Extensions: extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
			return []contract.ExtensionPayload{{
				Name: "extension", Version: "1.0.0", Format: fixture.extensionInstalledFormat, Origin: origin,
				Provenance: &contract.ExtensionProvenancePayload{
					Slug: installedSlug, CatalogEntryID: fixture.extensionCatalogEntryID,
				},
			}}, nil
		}, marketplaceTrustFn: fixture.extensionTrust},
		HomePaths: homePaths,
		Config:    config,
		Logger:    testutil.DiscardLogger(),
	})
}

// marketplaceExtensionPayloadForTest rebuilds the curated extension payload with an optional format
// marker, matching what the feed reader stores after a strict decode.
func marketplaceExtensionPayloadForTest(t *testing.T, format string) json.RawMessage {
	t.Helper()
	payload := map[string]string{
		"entry_id":      "extension-entry",
		"name":          "Extension",
		"description":   "Extension",
		"version":       "1.2.0",
		"install_slug":  "acme/extension",
		"artifact_url":  "https://downloads.example.test/extension-v1.2.0.tar.gz",
		"digest_sha256": strings.Repeat("a", 64),
	}
	if format != "" {
		payload["format"] = format
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(extension marketplace payload) error = %v", err)
	}
	return encoded
}

func marketplaceEntryForTest() marketplacepkg.Entry {
	return marketplacepkg.Entry{
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
	}
}

// Invariant: catalog pages join installed state only by origin and retain exact source/cursor metadata.
// Owner: API core catalog projection; canonical suite: marketplace_test.go (UT-004, UT-011, UT-068).
func TestMarketplaceCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should mask persisted stale diagnostics when internal error masking is enabled", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogBrowse: func(
				_ context.Context,
				_ string,
				_ int,
			) (marketplacepkg.BrowseResult, error) {
				entry := marketplaceEntryForTest()
				return marketplacepkg.BrowseResult{
					Entries:    []marketplacepkg.Entry{entry},
					Stale:      true,
					ErrorClass: "store",
					LastError:  "database path /private/catalog.db unavailable",
					Sources: []marketplacepkg.SourceState{{Enabled: true, Kind: marketplacepkg.SourceKindFeed,
						Source: marketplacepkg.CompozyCatalogSource, Stale: true, ErrorClass: "store",
						LastError: "database path /private/catalog.db unavailable",
					}},
				}, nil
			},
		})
		handlers.MaskInternalErrors = true
		response, err := handlers.MarketplaceList(t.Context(), core.MarketplaceListRequest{})
		if err != nil {
			t.Fatalf("MarketplaceList() error = %v", err)
		}
		if got := response.Error; got != http.StatusText(http.StatusInternalServerError) {
			t.Fatalf("persisted stale error = %q, want masked %q", got, http.StatusText(http.StatusInternalServerError))
		}
	})

	t.Run("Should project the curated format marker and let an installed instance override it", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name            string
			feedFormat      string
			installedFormat string
			installed       bool
			wantFormat      string
		}{
			{name: "portable marker before install", feedFormat: "agent-plugin", wantFormat: "agent-plugin"},
			{name: "native marker before install", feedFormat: "compozy", wantFormat: "compozy"},
			{name: "marker-less entry before install", wantFormat: "compozy"},
			{
				name:            "installed portable package under a marker-less entry",
				installed:       true,
				installedFormat: "agent-plugin",
				wantFormat:      "agent-plugin",
			},
			{
				name:            "installed native extension under a portable marker",
				feedFormat:      "agent-plugin",
				installed:       true,
				installedFormat: "compozy",
				wantFormat:      "compozy",
			},
		}
		for _, tt := range tests {
			t.Run("Should resolve the "+tt.name, func(t *testing.T) {
				t.Parallel()

				entry := marketplaceEntryForTest()
				entry.Payload = marketplaceExtensionPayloadForTest(t, tt.feedFormat)
				handlerFixture := marketplaceHandlerFixture{
					extensionInstalledFormat: tt.installedFormat,
					catalogBrowse: func(
						_ context.Context,
						_ string,
						_ int,
					) (marketplacepkg.BrowseResult, error) {
						return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{entry}, Total: 1}, nil
					},
				}
				if tt.installed {
					handlerFixture.extensionCatalogEntryID = entry.EntryID
				} else {
					handlerFixture.extensionInstalledSlug = "unrelated/extension"
				}
				handlers := marketplaceHandlersForTest(t, handlerFixture)
				response := marketplaceListHTTP(t, handlers)
				item := response.Items[0]
				if item.Installed != tt.installed {
					t.Fatalf("extension item installed = %t, want %t: %#v", item.Installed, tt.installed, item)
				}
				if item.Format != tt.wantFormat {
					t.Fatalf("extension item format = %q, want %q", item.Format, tt.wantFormat)
				}
			})
		}
	})

	t.Run("Should not satisfy a stable catalog ID lookup with an unrelated install slug", func(t *testing.T) {
		t.Parallel()

		entry := marketplaceEntryForTest()
		entry.EntryID = "acme/extension"
		entry.InstallSlug = "other/extension"
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			extensionCatalogEntryID: "different-entry",
			extensionInstalledSlug:  "acme/extension",
			catalogBrowse: func(
				_ context.Context,
				_ string,
				_ int,
			) (marketplacepkg.BrowseResult, error) {
				return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{entry}}, nil
			},
		})
		response, err := handlers.MarketplaceList(t.Context(), core.MarketplaceListRequest{})
		if err != nil {
			t.Fatalf("MarketplaceList() error = %v", err)
		}
		if item := response.Items[0]; item.Installed {
			t.Fatalf("extension item = %#v, want unrelated slug collision ignored", item)
		}
	})

	t.Run("Should map a curated catalog failure to service unavailable", func(t *testing.T) {
		t.Parallel()

		catalogErr := errors.New("catalog request failed")
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogBrowse: func(
				context.Context,
				string,
				int,
			) (marketplacepkg.BrowseResult, error) {
				return marketplacepkg.BrowseResult{}, errors.Join(
					marketplacepkg.ErrSourceUnavailable,
					catalogErr,
				)
			},
		})
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)

		response := performRequest(t, engine, http.MethodGet, "/marketplace", nil)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusServiceUnavailable,
				response.Body.String(),
			)
		}
		if !strings.Contains(response.Body.String(), catalogErr.Error()) {
			t.Fatalf("body = %s, want catalog failure %q", response.Body.String(), catalogErr.Error())
		}
	})

	t.Run("Should preserve internal status for a curated store failure", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogBrowse: func(
				context.Context,
				string,
				int,
			) (marketplacepkg.BrowseResult, error) {
				return marketplacepkg.BrowseResult{}, errors.New("catalog store failed")
			},
		})
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)

		response := performRequest(t, engine, http.MethodGet, "/marketplace", nil)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusInternalServerError,
				response.Body.String(),
			)
		}
	})

	t.Run("Should return failed refresh outcomes that preserve the stale projection", func(t *testing.T) {
		t.Parallel()

		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{
			catalogRefresh: func(
				context.Context,

			) (marketplacepkg.RefreshReport, error) {
				return marketplacepkg.RefreshReport{Outcomes: []marketplacepkg.RefreshOutcome{
					{
						Source:     marketplacepkg.CompozyCatalogSource,
						Outcome:    marketplacepkg.RefreshOutcomeFailed,
						EntryCount: 1,
						Stale:      true,
						ErrorClass: "network",
					},
				}}, errors.New("catalog feed unavailable")
			},
		})
		engine := gin.New()
		engine.POST("/marketplace/refresh", handlers.RefreshMarketplaceCatalog)

		response := performRequest(t, engine, http.MethodPost, "/marketplace/refresh", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.MarketplaceRefreshResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(payload.Sources) != 1 || payload.Sources[0].Source != marketplacepkg.CompozyCatalogSource ||
			payload.Sources[0].Outcome != string(marketplacepkg.RefreshOutcomeFailed) ||
			payload.Sources[0].EntryCount != 1 || !payload.Sources[0].Stale ||
			payload.Sources[0].ErrorClass != "network" {
			t.Fatalf("refresh payload = %#v, want stale failure report", payload)
		}
	})

	t.Run("Should resolve workspace extension listings and details from one scoped projection", func(t *testing.T) {
		t.Parallel()

		entry := marketplaceEntryForTest()
		resolvedActor, err := taskpkg.DeriveHumanActorContextForWorkspace(
			"marketplace-resolver",
			"ws-alpha",
			taskpkg.OriginKindWeb,
			"marketplace.test",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
		}
		scopedCalls := 0
		resolvedActions := make([]string, 0, 2)
		service := extensionServiceStub{
			listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{}, nil
			},
			listScopedFn: func(
				_ context.Context,
				actor taskpkg.ActorContext,
			) ([]contract.ExtensionPayload, error) {
				scopedCalls++
				if actor.Actor.Ref != "marketplace-resolver" ||
					actor.Scope.WorkspaceID != "ws-alpha" || !actor.Authority.Read ||
					actor.ReadScope.ProfileID != store.DefaultProfileID {
					t.Fatalf("scoped extension actor = %#v, want resolved readable workspace actor", actor)
				}
				return []contract.ExtensionPayload{
					{
						Name:        "epoch-probe",
						Version:     "0.1.0",
						Source:      "workspace",
						Dev:         true,
						WorkspaceID: "ws-alpha",
						Origin: &contract.MarketplaceOriginPayload{
							Source:    marketplacepkg.CompozyCatalogSource,
							SourceRef: marketplacepkg.CompozyCatalogRef,
							EntryID:   entry.EntryID,
						},
						Provenance: &contract.ExtensionProvenancePayload{Slug: entry.InstallSlug},
					},
				}, nil
			},
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			MarketplaceCatalog: marketplaceCatalogStub{
				browseFn: func(
					_ context.Context,
					_ string,
					_ int,
				) (marketplacepkg.BrowseResult, error) {
					return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{entry}, Total: 1}, nil
				},
				detailFn: func(context.Context, string) (*marketplacepkg.Entry, error) {
					return nil, marketplacepkg.ErrEntryNotFound
				},
			},
			Extensions: service,
			Logger:     testutil.DiscardLogger(),
			TaskActorContextResolver: func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
				resolvedActions = append(resolvedActions, action)
				return resolvedActor, nil
			},
		})
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)
		engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)

		workspaceResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace?scope=workspace&workspace_id=ws-alpha",
			nil,
		)
		if workspaceResponse.Code != http.StatusOK {
			t.Fatalf(
				"workspace listing status = %d, want %d; body=%s",
				workspaceResponse.Code,
				http.StatusOK,
				workspaceResponse.Body.String(),
			)
		}
		var workspace contract.MarketplaceListResponse
		if err := json.Unmarshal(workspaceResponse.Body.Bytes(), &workspace); err != nil {
			t.Fatalf("json.Unmarshal(workspace listing) error = %v", err)
		}
		if len(workspace.Items) != 1 || !workspace.Items[0].Installed ||
			workspace.Items[0].InstalledName != "epoch-probe" {
			t.Fatalf("workspace extension listing = %#v, want scoped installation", workspace.Items)
		}

		globalResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace?scope=global",
			nil,
		)
		if globalResponse.Code != http.StatusOK {
			t.Fatalf(
				"global listing status = %d, want %d; body=%s",
				globalResponse.Code,
				http.StatusOK,
				globalResponse.Body.String(),
			)
		}
		var global contract.MarketplaceListResponse
		if err := json.Unmarshal(globalResponse.Body.Bytes(), &global); err != nil {
			t.Fatalf("json.Unmarshal(global listing) error = %v", err)
		}
		if len(global.Items) != 1 || global.Items[0].Installed {
			t.Fatalf("global extension listing = %#v, want no workspace installation", global.Items)
		}

		detailResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace/entries/"+entry.EntryID+
				"?installed_name=epoch-probe&scope=workspace&workspace_id=ws-alpha",
			nil,
		)
		if detailResponse.Code != http.StatusOK {
			t.Fatalf(
				"workspace detail status = %d, want %d; body=%s",
				detailResponse.Code,
				http.StatusOK,
				detailResponse.Body.String(),
			)
		}
		var detail contract.MarketplaceEntryResponse
		if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
			t.Fatalf("json.Unmarshal(workspace detail) error = %v", err)
		}
		if !detail.Entry.Installed || detail.Entry.Name != "epoch-probe" {
			t.Fatalf("workspace extension detail = %#v, want scoped installed identity", detail.Entry)
		}
		if scopedCalls != 2 {
			t.Fatalf("ListScoped() calls = %d, want listing and detail", scopedCalls)
		}
		if diff := cmp.Diff(
			[]string{"marketplace.browse", "marketplace.entry"},
			resolvedActions,
		); diff != "" {
			t.Fatalf("resolved marketplace actions mismatch (-want +got):\n%s", diff)
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
		engine.GET("/marketplace", handlers.ListMarketplace)
		engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)

		browse := performRequest(t, engine, http.MethodGet, "/marketplace", nil)
		if browse.Code != http.StatusOK {
			t.Fatalf("browse status = %d, want %d; body=%s", browse.Code, http.StatusOK, browse.Body.String())
		}
		var kind contract.MarketplaceListResponse
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
			"/marketplace/entries/extension-entry",
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

	// Invariant: obsolete selectors fail before catalog dispatch on canonical routes.
	// Owner: shared Marketplace handlers; canonical suite: TestMarketplaceCatalog.
	t.Run("Should reject kind selectors on canonical browse, detail and refresh", func(t *testing.T) {
		t.Parallel()
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{})
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)
		engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)
		engine.POST("/marketplace/refresh", handlers.RefreshMarketplaceCatalog)
		for _, request := range []struct{ method, path string }{{http.MethodGet, "/marketplace"}, {http.MethodGet, "/marketplace/entries/review"}, {http.MethodPost, "/marketplace/refresh"}} {
			for _, value := range []string{"", "extension"} {
				response := performRequest(t, engine, request.method, request.path+"?kind="+value, nil)
				if response.Code != http.StatusBadRequest ||
					!strings.Contains(response.Body.String(), "kind is not supported") {
					t.Fatalf(
						"obsolete selector %s kind=%q: %d %s",
						request.path,
						value,
						response.Code,
						response.Body.String(),
					)
				}
			}
		}
	})

	// Invariant: exact-origin installed details preserve observed state; acquisition details use pinned inspection.
	// Owner: shared marketplace transport. Canonical suite: TestMarketplaceCatalog.
	t.Run("Should distinguish installed observations from pinned package declarations", func(t *testing.T) {
		t.Parallel()
		for _, installed := range []bool{false, true} {
			entry := marketplaceEntryForTest()
			handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
			handlers.MarketplaceCatalog = marketplaceCatalogStub{
				detailFn: func(context.Context, string) (*marketplacepkg.Entry, error) { return &entry, nil },
			}
			inspections := 0
			servers := []contract.MarketplaceServerPayload{
				{Name: "remote", Owner: "extension:custom-instance", Transport: "http"},
			}
			if installed {
				servers[0].Status, servers[0].RuntimeName = "running", "custom-instance.remote"
			}
			handlers.Extensions = marketplaceInspectionService{
				extensionServiceStub: extensionServiceStub{
					listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
						sourceRef := "github:team/other"
						if installed {
							sourceRef = marketplacepkg.CompozyCatalogRef
						}
						return []contract.ExtensionPayload{{Name: "custom-instance", Version: "1.0.0",
							Origin:   &contract.MarketplaceOriginPayload{SourceRef: sourceRef, EntryID: entry.EntryID},
							Contents: contract.ExtensionContentsPayload{MCPServers: 1}, MCPServers: servers,
						}}, nil
					},
				},
				inspectFn: func(_ context.Context, got marketplacepkg.Entry, profile string) (contract.MarketplaceExtensionDetailPayload, error) {
					inspections++
					if got.EntryID != entry.EntryID || got.DigestSHA256 != entry.DigestSHA256 || profile != "" {
						t.Fatalf("inspection selection = %#v profile=%q", got, profile)
					}
					return contract.MarketplaceExtensionDetailPayload{
						Contents: contract.ExtensionContentsPayload{MCPServers: 1}, MCPServers: servers,
					}, nil
				},
			}
			engine := gin.New()
			engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)
			response := performRequest(t, engine, http.MethodGet, "/marketplace/entries/"+entry.EntryID, nil)
			if response.Code != http.StatusOK {
				t.Fatalf("detail = %d %s", response.Code, response.Body.String())
			}
			var result contract.MarketplaceEntryResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Extension == nil || result.Extension.Contents.MCPServers != 1 ||
				len(result.Extension.MCPServers) != 1 {
				t.Fatalf("detail = %#v", result.Extension)
			}
			if diff := cmp.Diff(servers, result.Extension.MCPServers); diff != "" {
				t.Fatalf("MCP state (-want +got): %s", diff)
			}
			if (inspections == 0) != installed {
				t.Fatalf("installed=%t inspections=%d", installed, inspections)
			}
		}
	})
	t.Run("Should report changed inspection bytes as a typed conflict", func(t *testing.T) {
		t.Parallel()
		entry := marketplaceEntryForTest()
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		handlers.MarketplaceCatalog = marketplaceCatalogStub{
			detailFn: func(context.Context, string) (*marketplacepkg.Entry, error) { return &entry, nil },
		}
		listed, fetched := strings.Repeat("a", 64), strings.Repeat("b", 64)
		handlers.Extensions = marketplaceInspectionService{
			inspectFn: func(context.Context, marketplacepkg.Entry, string) (contract.MarketplaceExtensionDetailPayload, error) {
				return contract.MarketplaceExtensionDetailPayload{}, &extensionpkg.SourceChangedError{
					ListedDigest:  listed,
					FetchedDigest: fetched,
				}
			},
		}
		engine := gin.New()
		engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)
		response := performRequest(t, engine, http.MethodGet, "/marketplace/entries/"+entry.EntryID, nil)
		if response.Code != http.StatusConflict {
			t.Fatalf("detail = %d %s", response.Code, response.Body.String())
		}
		var result contract.ExtensionOperationErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Code != "extension_source_changed" || result.ListedDigest != listed ||
			result.FetchedDigest != fetched {
			t.Fatalf("conflict = %#v", result)
		}
	})
	// Invariant: discovery detail exposes install prompts and typed defaults before installation.
	// Owner: shared HTTP/UDS detail payload. Canonical suite: marketplace_test.go.
	t.Run("Should expose packaged input declarations before installation", func(t *testing.T) {
		t.Parallel()
		entry := marketplaceEntryForTest()
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		payload["inputs"] = json.RawMessage(
			`[{"id":"debug","prompt":"Debug","type":"boolean","required":false,"binding":{"type":"env","name":"DEBUG"},"default":false}]`,
		)
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		entry.Payload = raw
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		handlers.MarketplaceCatalog = marketplaceCatalogStub{
			detailFn: func(context.Context, string) (*marketplacepkg.Entry, error) { return &entry, nil },
		}
		engine := gin.New()
		engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)
		response := performRequest(t, engine, http.MethodGet, "/marketplace/entries/"+entry.EntryID, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("detail = %d %s", response.Code, response.Body.String())
		}
		var result contract.MarketplaceEntryResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Extension == nil || len(result.Extension.Inputs) != 1 {
			t.Fatalf("detail = %#v", result.Extension)
		}
		input := result.Extension.Inputs[0]
		if input.ID != "debug" || input.Prompt != "Debug" || string(input.Default) != "false" ||
			input.Binding.Name != "DEBUG" {
			t.Fatalf("input = %#v", input)
		}
	})

	t.Run("Should join only the exact origin and emit the one-catalog envelope", func(t *testing.T) {
		t.Parallel()
		entry := marketplaceEntryForTest()
		query := strings.Repeat("é", 201)
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		handlers.MarketplaceCatalog = marketplaceCatalogStub{
			browsePageFn: func(_ context.Context, q string, offset, limit int) (marketplacepkg.BrowseResult, error) {
				if q != strings.Repeat("é", 200) || offset != 0 ||
					limit != 100 {
					t.Fatalf("browse %q %d %d", q, offset, limit)
				}
				return marketplacepkg.BrowseResult{
					Entries:  []marketplacepkg.Entry{entry},
					Total:    1,
					Revision: "content-a",
					Sources: []marketplacepkg.SourceState{
						{
							Enabled:    true,
							Kind:       marketplacepkg.SourceKindFeed,
							Source:     marketplacepkg.CompozyCatalogSource,
							Revision:   "content-a",
							EntryCount: 9,
							FetchedAt:  time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
						},
					},
				}, nil
			},
		}
		origins := []*contract.MarketplaceOriginPayload{
			nil,
			{Source: "team", SourceRef: "github:team/plugins", EntryID: entry.EntryID},
			{Source: "old-display", SourceRef: "catalog:compozy", EntryID: entry.EntryID},
		}
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)
		for index, origin := range origins {
			handlers.Extensions = extensionServiceStub{
				listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
					return []contract.ExtensionPayload{
						{
							Name:    "custom-instance",
							Version: "0.0.1",
							Origin:  origin,
							Provenance: &contract.ExtensionProvenancePayload{
								Slug:           entry.InstallSlug,
								CatalogEntryID: entry.EntryID,
							},
						},
					}, nil
				},
			}
			response := performRequest(t, engine, http.MethodGet, "/marketplace?q="+query, nil)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var body contract.MarketplaceListResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Total != 1 || body.Revision != "content-a" || len(body.Sources) != 1 ||
				body.Sources[0].Count != 9 ||
				len(body.Items) != 1 {
				t.Fatalf("envelope=%#v", body)
			}
			item := body.Items[0]
			if item.Installed != (index == 2) || item.UpdateAvailable != (index == 2) {
				t.Fatalf("origin=%#v listing=%#v", origin, item)
			}
			if item.Source != "compozy-catalog" || item.SourceRef != "catalog:compozy" ||
				item.InstallSlug != "compozy/"+entry.EntryID ||
				item.ManagePath != "/marketplace/installed" ||
				!item.Installable ||
				item.DigestSHA256 != entry.DigestSHA256 {
				t.Fatalf("listing=%#v", item)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
				t.Fatal(err)
			}
			if _, exists := wire["next_cursor"]; exists {
				t.Fatal("final page emitted next_cursor")
			}
			var items []map[string]json.RawMessage
			if err := json.Unmarshal(wire["items"], &items); err != nil {
				t.Fatal(err)
			}
			if _, exists := items[0]["kind"]; exists {
				t.Fatal("one-catalog listing emitted kind")
			}
			if _, exists := items[0]["icon"]; exists {
				t.Fatal("absent icon must be omitted")
			}
		}
	})
	t.Run("Should reject a stale content revision and cursors from a different query", func(t *testing.T) {
		t.Parallel()
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		revision := "content-a"
		handlers.MarketplaceCatalog = marketplaceCatalogStub{
			browsePageFn: func(_ context.Context, _ string, offset, limit int) (marketplacepkg.BrowseResult, error) {
				entry := marketplaceEntryForTest()
				return marketplacepkg.BrowseResult{
					Entries:  []marketplacepkg.Entry{entry},
					Total:    2,
					Revision: revision,
					Sources: []marketplacepkg.SourceState{
						{
							Enabled:    true,
							Kind:       marketplacepkg.SourceKindFeed,
							Source:     marketplacepkg.CompozyCatalogSource,
							Revision:   revision,
							EntryCount: 2,
						},
					},
				}, nil
			},
		}
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)
		first := performRequest(t, engine, http.MethodGet, "/marketplace?limit=1", nil)
		if first.Code != 200 {
			t.Fatalf("first=%d %s", first.Code, first.Body.String())
		}
		var page contract.MarketplaceListResponse
		if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.NextCursor == "" {
			t.Fatal("missing continuation")
		}
		matching := performRequest(t, engine, http.MethodGet, "/marketplace?limit=1&cursor="+page.NextCursor, nil)
		if matching.Code != 200 {
			t.Fatalf("same revision=%d %s", matching.Code, matching.Body.String())
		}
		revision = "content-b"
		stale := performRequest(t, engine, http.MethodGet, "/marketplace?limit=1&cursor="+page.NextCursor, nil)
		var failure contract.MarketplaceCursorStalePayload
		if err := json.Unmarshal(stale.Body.Bytes(), &failure); err != nil {
			t.Fatal(err)
		}
		if stale.Code != 409 || failure.Code != "marketplace_cursor_stale" || !failure.Restart {
			t.Fatalf("stale=%d %#v", stale.Code, failure)
		}
		mismatch := performRequest(t, engine, http.MethodGet, "/marketplace?q=different&cursor="+page.NextCursor, nil)
		if mismatch.Code != 400 || !strings.Contains(mismatch.Body.String(), "cursor") {
			t.Fatalf("mismatch=%d %s", mismatch.Code, mismatch.Body.String())
		}
	})
	t.Run("Should distinguish an unrefreshed source from a failed refresh", func(t *testing.T) {
		t.Parallel()
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		handlers.MarketplaceCatalog = marketplaceCatalogStub{
			browseFn: func(context.Context, string, int) (marketplacepkg.BrowseResult, error) {
				return marketplacepkg.BrowseResult{Entries: []marketplacepkg.Entry{}, Stale: true,
					Sources: []marketplacepkg.SourceState{{Source: marketplacepkg.CompozyCatalogSource,
						Kind: marketplacepkg.SourceKindFeed, Enabled: true, Stale: true}},
				}, nil
			},
		}
		response, err := handlers.MarketplaceList(t.Context(), core.MarketplaceListRequest{})
		if err != nil || len(response.Sources) != 1 || response.Sources[0].State != "never" || !response.Stale {
			t.Fatalf("unrefreshed catalog = %+v, err=%v", response, err)
		}
	})
	t.Run("Should return truthful degraded source state with no cached entries", func(t *testing.T) {
		t.Parallel()
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		handlers.MarketplaceCatalog = marketplaceCatalogStub{
			browseFn: func(context.Context, string, int) (marketplacepkg.BrowseResult, error) {
				return marketplacepkg.BrowseResult{
					Entries:    []marketplacepkg.Entry{},
					Stale:      true,
					ErrorClass: "network",
					LastError:  "source unreachable",
					Sources: []marketplacepkg.SourceState{
						{
							Enabled:    true,
							Kind:       marketplacepkg.SourceKindFeed,
							Source:     marketplacepkg.CompozyCatalogSource,
							Stale:      true,
							ErrorClass: "network",
							LastError:  "source unreachable",
						},
					},
				}, nil
			},
		}
		engine := gin.New()
		engine.GET("/marketplace", handlers.ListMarketplace)
		response := performRequest(t, engine, http.MethodGet, "/marketplace", nil)
		var body contract.MarketplaceListResponse
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || !body.Stale || body.ErrorClass != "network" || body.Items == nil ||
			len(body.Sources) != 1 ||
			body.Sources[0].State != "degraded" {
			t.Fatalf("degraded=%d %#v", response.Code, body)
		}
	})
	t.Run("Should retain an unclassified installed detail without inventing catalog identity", func(t *testing.T) {
		t.Parallel()
		handlers := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		handlers.Extensions = extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
			return []contract.ExtensionPayload{
				{Name: "sideload", Source: "local_path", Contents: contract.ExtensionContentsPayload{Skills: 2}},
			}, nil
		}}
		engine := gin.New()
		engine.GET("/marketplace/entries/:entry_id", handlers.GetMarketplaceCatalogEntry)
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/marketplace/entries/sideload?installed_name=sideload",
			nil,
		)
		var body contract.MarketplaceEntryResponse
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || body.Entry.SourceRef != "" || body.Entry.Source != "local_path" ||
			body.Entry.InstallSlug != "" ||
			!body.Entry.Installed ||
			body.Extension == nil ||
			body.Extension.Contents.Skills != 2 {
			t.Fatalf("installed=%d %#v", response.Code, body)
		}
		missing := performRequest(t, engine, http.MethodGet, "/marketplace/entries/sideload?source=foreign", nil)
		if missing.Code != 404 || !strings.Contains(missing.Body.String(), "not found") {
			t.Fatalf("unknown source=%d %s", missing.Code, missing.Body.String())
		}
	})
}

// Invariant: catalog installed joins use the selected profile within its workspace overlay.
// Owner: shared catalog read boundary; canonical suite: marketplace_test.go.
func TestMarketplaceCatalogProfileWorkspace(t *testing.T) {
	t.Parallel()
	t.Run("Should retain both profile and workspace in a catalog continuation", func(t *testing.T) {
		t.Parallel()
		entry := marketplaceEntryForTest()
		actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
			"operator",
			"ws-a",
			taskpkg.OriginKindHTTP,
			"marketplace.browse",
		)
		if err != nil {
			t.Fatal(err)
		}
		actor.ReadScope = store.ReadScope{ProfileID: "profile-work"}
		h := marketplaceHandlersForTest(t, marketplaceHandlerFixture{})
		h.MarketplaceCatalog = marketplaceCatalogStub{
			browsePageFn: func(context.Context, string, int, int) (marketplacepkg.BrowseResult, error) {
				return marketplacepkg.BrowseResult{
					Entries:  []marketplacepkg.Entry{entry},
					Total:    2,
					Revision: "same",
					Sources: []marketplacepkg.SourceState{
						{
							Enabled:  true,
							Kind:     marketplacepkg.SourceKindFeed,
							Source:   marketplacepkg.CompozyCatalogSource,
							Revision: "same",
						},
					},
				}, nil
			},
		}
		h.Extensions = extensionServiceStub{
			listScopedFn: func(_ context.Context, got taskpkg.ActorContext) ([]contract.ExtensionPayload, error) {
				if got.Scope.WorkspaceID != "ws-a" || got.ReadScope.ProfileID != "profile-work" {
					t.Fatalf("actor=%#v", got)
				}
				return []contract.ExtensionPayload{
					{
						Name:    "installed",
						Version: entry.Version,
						Origin: &contract.MarketplaceOriginPayload{
							SourceRef: marketplacepkg.CompozyCatalogRef,
							EntryID:   entry.EntryID,
						},
					},
				}, nil
			},
		}
		request := core.MarketplaceListRequest{
			Scope:       "workspace",
			WorkspaceID: "ws-a",
			ProfileName: "work",
			Actor:       &actor,
			Limit:       1,
		}
		first, err := h.MarketplaceList(t.Context(), request)
		if err != nil || len(first.Items) != 1 || !first.Items[0].Installed || first.NextCursor == "" {
			t.Fatalf("page=%#v error=%v", first, err)
		}
		request.Cursor = first.NextCursor
		if _, err := h.MarketplaceList(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		request.ProfileName = "personal"
		if _, err := h.MarketplaceList(t.Context(), request); !errors.Is(err, core.ErrMarketplaceValidation) {
			t.Fatalf("cross-profile cursor error=%v", err)
		}
	})
}
