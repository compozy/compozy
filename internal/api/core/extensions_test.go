package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	extensionpkg "github.com/compozy/agh/internal/extension"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func TestListExtensionsJoinsMarketplaceByExactCatalogEntryID(t *testing.T) {
	t.Parallel()

	t.Run("Should enrich installed extension without browsing the capped catalog", func(t *testing.T) {
		t.Parallel()

		entry := marketplaceEntriesForTest()[marketplacepkg.KindExtension]
		browseCalled := false
		detailEntryID := ""
		homePaths := testutil.NewTestHomePaths(t)
		cfg := testConfigWithDisabledNetwork(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "http",
			Extensions: extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{{
					Name: "extension", Version: "1.0.0", Type: "wasm", Source: "marketplace",
					Provenance: &contract.ExtensionProvenancePayload{
						Slug: "acme/extension", CatalogEntryID: entry.EntryID,
					},
				}}, nil
			}},
			MarketplaceCatalog: marketplaceCatalogStub{
				browseFn: func(
					context.Context,
					marketplacepkg.Kind,
					string,
					int,
				) (marketplacepkg.BrowseResult, error) {
					browseCalled = true
					return marketplacepkg.BrowseResult{}, errors.New("capped browse must not own the join")
				},
				detailFn: func(
					_ context.Context,
					kind marketplacepkg.Kind,
					entryID string,
				) (*marketplacepkg.Entry, error) {
					if kind != marketplacepkg.KindExtension {
						t.Fatalf("detail kind = %q, want %q", kind, marketplacepkg.KindExtension)
					}
					detailEntryID = entryID
					return &entry, nil
				},
			},
			HomePaths: homePaths,
			Config:    cfg,
			Logger:    testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.GET("/extensions", handlers.ListExtensions)

		response := performRequest(t, engine, http.MethodGet, "/extensions", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload struct {
			Extensions []struct {
				Marketplace *contract.MarketplaceListingPayload `json:"marketplace"`
			} `json:"extensions"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if browseCalled {
			t.Fatal("list extensions browsed the capped marketplace catalog")
		}
		if detailEntryID != entry.EntryID {
			t.Fatalf("detail entry id = %q, want %q", detailEntryID, entry.EntryID)
		}
		if len(payload.Extensions) != 1 || payload.Extensions[0].Marketplace == nil {
			t.Fatalf("extensions = %#v, want one exact marketplace projection", payload.Extensions)
		}
		listing := payload.Extensions[0].Marketplace
		if listing.EntryID != entry.EntryID || listing.Description != entry.Description ||
			!listing.Installed || listing.InstalledVersion != "1.0.0" || !listing.UpdateAvailable {
			t.Fatalf("marketplace listing = %#v, want exact installed update projection", listing)
		}
	})

	t.Run("Should retain local inventory when exact catalog enrichment is unavailable", func(t *testing.T) {
		t.Parallel()

		homePaths := testutil.NewTestHomePaths(t)
		cfg := testConfigWithDisabledNetwork(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "http",
			Extensions: extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{{
					Name: "offline-extension", Version: "1.0.0",
					Provenance: &contract.ExtensionProvenancePayload{CatalogEntryID: "extension.offline"},
				}}, nil
			}},
			MarketplaceCatalog: marketplaceCatalogStub{detailFn: func(
				context.Context,
				marketplacepkg.Kind,
				string,
			) (*marketplacepkg.Entry, error) {
				return nil, errors.Join(errors.New("catalog offline"), marketplacepkg.ErrEntryNotFound)
			}},
			HomePaths: homePaths,
			Config:    cfg,
			Logger:    testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.GET("/extensions", handlers.ListExtensions)

		response := performRequest(t, engine, http.MethodGet, "/extensions", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.ExtensionsResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.Extensions) != 1 || payload.Extensions[0].Name != "offline-extension" {
			t.Fatalf("extensions = %#v, want retained local inventory", payload.Extensions)
		}
		if payload.Extensions[0].Marketplace != nil {
			t.Fatalf("marketplace = %#v, want no invented catalog projection", payload.Extensions[0].Marketplace)
		}
	})
}

type extensionServiceStub struct {
	listFn             func(context.Context) ([]contract.ExtensionPayload, error)
	installFn          func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	marketplaceTrustFn func(context.Context, extensionpkg.MarketplaceTrustEvidence) (contract.ExtensionTrustReportPayload, error)
}

func (s extensionServiceStub) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}

func (s extensionServiceStub) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.installFn != nil {
		return s.installFn(ctx, req, actor)
	}
	return contract.ExtensionPayload{}, nil
}

func (extensionServiceStub) Update(
	context.Context,
	string,
	contract.UpdateExtensionRequest,
	taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	return contract.ManagedExtensionUpdatePayload{}, nil
}

func (extensionServiceStub) Remove(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	return contract.ManagedExtensionRemovePayload{}, nil
}

func (extensionServiceStub) Enable(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, nil
}

func (extensionServiceStub) Disable(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, nil
}

func (extensionServiceStub) Status(context.Context, string) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, nil
}

func (extensionServiceStub) Provenance(context.Context, string) (contract.ExtensionProvenancePayload, error) {
	return contract.ExtensionProvenancePayload{}, nil
}

func (s extensionServiceStub) MarketplaceTrust(
	ctx context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	if s.marketplaceTrustFn != nil {
		return s.marketplaceTrustFn(ctx, evidence)
	}
	return extensionpkg.MarketplaceEntryTrustReport(evidence, false)
}

func TestListExtensionsRespectsMaskInternalErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should mask internal extension errors when handler masking is enabled", func(t *testing.T) {
		// not parallel: gin.SetMode mutates process-global state.
		gin.SetMode(gin.TestMode)

		homePaths := testutil.NewTestHomePaths(t)
		cfg := testConfigWithDisabledNetwork(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			MaskInternalErrors: true,
			Extensions: extensionServiceStub{
				listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
					return nil, errors.New("extension registry token=super-secret failed")
				},
			},
			HomePaths: homePaths,
			Config:    cfg,
			Logger:    testutil.DiscardLogger(),
			StartedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			Now: func() time.Time {
				return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC)
			},
			HTTPPort: cfg.HTTP.Port,
		})

		engine := gin.New()
		engine.Use(gin.Recovery())
		engine.GET("/extensions", handlers.ListExtensions)

		response := performRequest(t, engine, http.MethodGet, "/extensions", nil)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusInternalServerError,
				response.Body.String(),
			)
		}
		if strings.Contains(response.Body.String(), "super-secret") {
			t.Fatalf("response body leaked internal error detail: %s", response.Body.String())
		}
	})
}

func TestInstallExtensionReturnsPolicyDiagnosticOverHTTP(t *testing.T) {
	t.Parallel()
	t.Run("Should return the side-load policy diagnostic over HTTP", func(t *testing.T) {
		t.Parallel()

		homePaths := testutil.NewTestHomePaths(t)
		cfg := testConfigWithDisabledNetwork(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "http",
			Extensions: extensionServiceStub{installFn: func(
				context.Context,
				contract.InstallExtensionRequest,
				taskpkg.ActorContext,
			) (contract.ExtensionPayload, error) {
				return contract.ExtensionPayload{}, extensionpkg.ValidateUnverifiedSideLoad(
					"acme/blocked", "local", false, true,
				)
			}},
			HomePaths: homePaths,
			Config:    cfg,
			Logger:    testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.POST("/extensions", handlers.InstallExtension)

		response := performRequest(
			t,
			engine,
			http.MethodPost,
			"/extensions",
			[]byte(`{"path":"/tmp/blocked","checksum":"sha256:abc","allow_unverified":true}`),
		)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusUnprocessableEntity,
				response.Body.String(),
			)
		}
		for _, want := range []string{
			contract.CodeExtensionUnverifiedPolicyBlocked,
			"Settings › Extensions",
			"extensions.marketplace.allow_unverified",
		} {
			if !strings.Contains(response.Body.String(), want) {
				t.Fatalf("body = %q, want %q", response.Body.String(), want)
			}
		}
	})
}
