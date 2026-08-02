package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	sessionpkg "github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
				Marketplace     *contract.MarketplaceListingPayload `json:"marketplace"`
				UpdateAvailable bool                                `json:"update_available"`
				RemoteVersion   string                              `json:"remote_version"`
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
		if !payload.Extensions[0].UpdateAvailable || payload.Extensions[0].RemoteVersion != entry.Version {
			t.Fatalf("extension update projection = %#v, want update to %q", payload.Extensions[0], entry.Version)
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

func TestExtensionDistributionHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should forward search filters and return the stable source page", func(t *testing.T) {
		t.Parallel()

		service := extensionServiceStub{searchFn: func(
			_ context.Context,
			req contract.ExtensionSearchRequest,
		) (contract.ExtensionSearchResponse, error) {
			want := contract.ExtensionSearchRequest{
				Query: "bridge", Sources: []string{"curated", "github"}, Limit: 7, Cursor: "next",
			}
			if !reflect.DeepEqual(req, want) {
				t.Fatalf("Search() request = %#v, want %#v", req, want)
			}
			return contract.ExtensionSearchResponse{
				Items:           []contract.ExtensionSearchItem{{Slug: "acme/bridge", Source: "github"}},
				SourcesDegraded: []string{"curated"},
			}, nil
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.GET("/extensions/search", handlers.SearchExtensions)
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/extensions/search?q=bridge&sources=curated,github&limit=7&cursor=next",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionSearchResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(search) error = %v", err)
		}
		if len(payload.Items) != 1 || payload.Items[0].Slug != "acme/bridge" ||
			!reflect.DeepEqual(payload.SourcesDegraded, []string{"curated"}) {
			t.Fatalf("search response = %#v", payload)
		}
	})

	t.Run("Should return updated and failed batch outcomes without discarding progress", func(t *testing.T) {
		t.Parallel()

		failed := &contract.DiagnosticItem{
			Code: diagnosticcontract.CodeExtensionUpdateFailed, Message: "Extension update failed.",
		}
		items := []contract.ManagedExtensionUpdatePayload{
			{Name: "a-good", Status: extensionpkg.MarketplaceUpdateStatusUpdated},
			{Name: "z-bad", Status: extensionpkg.MarketplaceUpdateStatusFailed, Error: failed},
		}
		service := extensionServiceStub{updateBatchFn: func(
			_ context.Context,
			req contract.UpdateExtensionsRequest,
			actor taskpkg.ActorContext,
		) ([]contract.ManagedExtensionUpdatePayload, error) {
			if !req.All || !req.CheckOnly || !actor.Authority.Write {
				t.Fatalf("UpdateBatch() request=%#v actor=%#v, want writable all/check", req, actor)
			}
			return items, &extensionpkg.MarketplaceUpdateBatchError{
				FailedName: "z-bad", Cause: errors.New("secret-sentinel upstream failure"),
			}
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Extensions: service, Logger: testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.POST("/extensions/update", handlers.UpdateExtensions)
		response := performRequest(
			t,
			engine,
			http.MethodPost,
			"/extensions/update",
			[]byte(`{"all":true,"check_only":true}`),
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "secret-sentinel") {
			t.Fatalf("response leaked internal error: %s", response.Body.String())
		}
		var payload contract.ExtensionUpdateBatchResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(update batch) error = %v", err)
		}
		if !reflect.DeepEqual(payload.Updates, items) {
			t.Fatalf("update batch response = %#v, want %#v", payload.Updates, items)
		}
	})

	t.Run("Should return structured issues for install validation failures", func(t *testing.T) {
		t.Parallel()

		service := extensionServiceStub{installFn: func(
			context.Context,
			contract.InstallExtensionRequest,
			taskpkg.ActorContext,
		) (contract.ExtensionPayload, error) {
			return contract.ExtensionPayload{}, &extensionpkg.ManifestValidationError{
				Field: "capabilities.provides", Value: "bridge.adapter",
				Message: "external bridge authoring is a planned follow-up",
			}
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.POST("/extensions", handlers.InstallExtension)
		response := performRequest(
			t,
			engine,
			http.MethodPost,
			"/extensions",
			[]byte(`{"source":"local_path","ref":"/tmp/bridge","allow_unverified":true}`),
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionValidationErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(validation error) error = %v", err)
		}
		if len(payload.Issues) != 1 || payload.Issues[0].Field != "capabilities.provides" ||
			!strings.Contains(payload.Issues[0].Message, "planned follow-up") {
			t.Fatalf("validation issues = %#v", payload.Issues)
		}
	})
}

type extensionServiceStub struct {
	listFn             func(context.Context) ([]contract.ExtensionPayload, error)
	searchFn           func(context.Context, contract.ExtensionSearchRequest) (contract.ExtensionSearchResponse, error)
	updateBatchFn      func(context.Context, contract.UpdateExtensionsRequest, taskpkg.ActorContext) ([]contract.ManagedExtensionUpdatePayload, error)
	installFn          func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	enableFn           func(context.Context, string, contract.EnableExtensionRequest, taskpkg.ActorContext) (contract.ExtensionEnableResult, error)
	inventoryFn        func(context.Context, string) (contract.ExtensionInventoryPayload, error)
	previewFn          func(context.Context, string) (contract.ExtensionEnablePreviewPayload, error)
	listSecretsFn      func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionSecretsPayload, error)
	setSecretsFn       func(context.Context, string, contract.SetExtensionSecretsRequest, taskpkg.ActorContext) (contract.ExtensionSecretsPayload, error)
	deleteSecretFn     func(context.Context, string, string, taskpkg.ActorContext) error
	marketplaceTrustFn func(context.Context, extensionpkg.MarketplaceTrustEvidence) (contract.ExtensionTrustReportPayload, error)
	devFn              func(context.Context, contract.DevLinkExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	reloadDevFn        func(context.Context, string, contract.ReloadExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	logsFn             func(context.Context, string, int64, taskpkg.ActorContext) ([]contract.ExtensionLogPayload, error)
	listScopedFn       func(context.Context, taskpkg.ActorContext) ([]contract.ExtensionPayload, error)
	statusScopedFn     func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	removeScopedFn     func(context.Context, string, taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
	commandsFn         func(context.Context, string) (contract.ExtensionCommandsResponse, error)
	commandsScopedFn   func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionCommandsResponse, error)
}

func (s extensionServiceStub) Search(
	ctx context.Context,
	req contract.ExtensionSearchRequest,
) (contract.ExtensionSearchResponse, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, req)
	}
	return contract.ExtensionSearchResponse{}, nil
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

func (s extensionServiceStub) UpdateBatch(
	ctx context.Context,
	req contract.UpdateExtensionsRequest,
	actor taskpkg.ActorContext,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	if s.updateBatchFn != nil {
		return s.updateBatchFn(ctx, req, actor)
	}
	return nil, nil
}

func (extensionServiceStub) Remove(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	return contract.ManagedExtensionRemovePayload{}, nil
}

func (s extensionServiceStub) Enable(
	ctx context.Context,
	name string,
	req contract.EnableExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionEnableResult, error) {
	if s.enableFn != nil {
		return s.enableFn(ctx, name, req, actor)
	}
	return contract.ExtensionEnableResult{}, nil
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

func (s extensionServiceStub) Inventory(ctx context.Context, name string) (contract.ExtensionInventoryPayload, error) {
	if s.inventoryFn != nil {
		return s.inventoryFn(ctx, name)
	}
	return contract.ExtensionInventoryPayload{}, nil
}

func (s extensionServiceStub) Preview(
	ctx context.Context,
	name string,
) (contract.ExtensionEnablePreviewPayload, error) {
	if s.previewFn != nil {
		return s.previewFn(ctx, name)
	}
	return contract.ExtensionEnablePreviewPayload{}, nil
}

func (s extensionServiceStub) ListExtensionSecrets(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	if s.listSecretsFn != nil {
		return s.listSecretsFn(ctx, name, actor)
	}
	return contract.ExtensionSecretsPayload{}, nil
}

func (s extensionServiceStub) SetExtensionSecrets(
	ctx context.Context,
	name string,
	req contract.SetExtensionSecretsRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	if s.setSecretsFn != nil {
		return s.setSecretsFn(ctx, name, req, actor)
	}
	return contract.ExtensionSecretsPayload{}, nil
}

func (s extensionServiceStub) DeleteExtensionSecret(
	ctx context.Context,
	name string,
	envName string,
	actor taskpkg.ActorContext,
) error {
	if s.deleteSecretFn != nil {
		return s.deleteSecretFn(ctx, name, envName, actor)
	}
	return nil
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

func (s extensionServiceStub) Dev(
	ctx context.Context,
	req contract.DevLinkExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.devFn != nil {
		return s.devFn(ctx, req, actor)
	}
	return contract.ExtensionPayload{}, nil
}

func (s extensionServiceStub) ReloadDev(
	ctx context.Context,
	name string,
	req contract.ReloadExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.reloadDevFn != nil {
		return s.reloadDevFn(ctx, name, req, actor)
	}
	return contract.ExtensionPayload{}, nil
}

func (s extensionServiceStub) ExtensionLogs(
	ctx context.Context,
	name string,
	after int64,
	actor taskpkg.ActorContext,
) ([]contract.ExtensionLogPayload, error) {
	if s.logsFn != nil {
		return s.logsFn(ctx, name, after, actor)
	}
	return []contract.ExtensionLogPayload{}, nil
}

func (s extensionServiceStub) ListScoped(
	ctx context.Context,
	actor taskpkg.ActorContext,
) ([]contract.ExtensionPayload, error) {
	if s.listScopedFn != nil {
		return s.listScopedFn(ctx, actor)
	}
	return s.List(ctx)
}

func (s extensionServiceStub) StatusScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.statusScopedFn != nil {
		return s.statusScopedFn(ctx, name, actor)
	}
	return s.Status(ctx, name)
}

func (s extensionServiceStub) RemoveScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if s.removeScopedFn != nil {
		return s.removeScopedFn(ctx, name, actor)
	}
	return s.Remove(ctx, name, actor)
}

func (s extensionServiceStub) Commands(
	ctx context.Context,
	extension string,
) (contract.ExtensionCommandsResponse, error) {
	if s.commandsFn != nil {
		return s.commandsFn(ctx, extension)
	}
	return contract.ExtensionCommandsResponse{}, nil
}

func (s extensionServiceStub) CommandsScoped(
	ctx context.Context,
	extension string,
	actor taskpkg.ActorContext,
) (contract.ExtensionCommandsResponse, error) {
	if s.commandsScopedFn != nil {
		return s.commandsScopedFn(ctx, extension, actor)
	}
	return s.Commands(ctx, extension)
}

func TestExtensionStatusCodeMapsDomainErrors(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name string
		err  error
		want int
	}{
		{name: "Should map nil to ok", want: http.StatusOK},
		{name: "Should map a missing extension to not found", err: extensionpkg.ErrExtensionNotFound, want: http.StatusNotFound},
		{name: "Should map a missing catalog entry to not found", err: marketplacepkg.ErrEntryNotFound, want: http.StatusNotFound},
		{name: "Should map a missing registry package to not found", err: registrypkg.ErrPackageNotFound, want: http.StatusNotFound},
		{name: "Should map an existing extension to conflict", err: extensionpkg.ErrExtensionExists, want: http.StatusConflict},
		{name: "Should map a checksum mismatch to bad request", err: extensionpkg.ErrExtensionChecksumMismatch, want: http.StatusBadRequest},
		{name: "Should map an archive digest mismatch to bad request", err: extensionpkg.ErrExtensionArchiveDigestMismatch, want: http.StatusBadRequest},
		{name: "Should map missing unverified consent to unprocessable", err: extensionpkg.ErrExtensionChecksumUnverified, want: http.StatusUnprocessableEntity},
		{name: "Should map blocked unverified policy to unprocessable", err: extensionpkg.ErrExtensionUnverifiedPolicyBlocked, want: http.StatusUnprocessableEntity},
		{name: "Should map an invalid Git repository URL to bad request", err: registrygit.ErrInvalidRepositoryRef, want: http.StatusBadRequest},
		{name: "Should map a blocked Git repository destination to forbidden", err: registrygit.ErrRepositoryDestinationBlocked, want: http.StatusForbidden},
		{name: "Should map manifest validation to bad request", err: extensionpkg.ErrManifestInvalid, want: http.StatusBadRequest},
		{name: "Should map bridge authoring rejection to bad request", err: &extensionpkg.ManifestValidationError{Field: "capabilities.provides", Value: "bridge.adapter", Message: "external bridge authoring is a planned follow-up"}, want: http.StatusBadRequest},
		{name: "Should map incompatible manifests to bad request", err: extensionpkg.ErrManifestIncompatible, want: http.StatusBadRequest},
		{name: "Should map missing manifests to bad request", err: extensionpkg.ErrManifestNotFound, want: http.StatusBadRequest},
		{name: "Should map a missing development origin to conflict", err: extensionpkg.ErrExtensionDevOriginMissing, want: http.StatusConflict},
		{name: "Should map a missing development link to conflict", err: extensionpkg.ErrExtensionNotDevLinked, want: http.StatusConflict},
		{name: "Should map an invalid generation to bad request", err: extensionpkg.ErrExtensionGenerationInvalid, want: http.StatusBadRequest},
		{name: "Should map cross-workspace access to forbidden", err: extensionpkg.ErrExtensionWorkspaceDenied, want: http.StatusForbidden},
		{name: "Should map denied extension writes to forbidden", err: taskpkg.ErrPermissionDenied, want: http.StatusForbidden},
		{name: "Should map unavailable marketplace sources to unavailable", err: extensionpkg.ErrMarketplaceSourceUnavailable, want: http.StatusServiceUnavailable},
		{name: "Should map an unavailable git binary to unavailable", err: registrygit.ErrGitUnavailable, want: http.StatusServiceUnavailable},
		{name: "Should map an unsupported git version to unavailable", err: registrygit.ErrGitVersionUnsupported, want: http.StatusServiceUnavailable},
		{name: "Should map invalid search cursors to bad request", err: extensionpkg.ErrExtensionSearchInvalid, want: http.StatusBadRequest},
		{name: "Should map network confirmation to conflict", err: extensionpkg.ErrExtensionNetworkConfirmationRequired, want: http.StatusConflict},
		{name: "Should map agent conflicts to conflict", err: extensionpkg.ErrExtensionAgentConflict, want: http.StatusConflict},
		{name: "Should map invalid bindings to bad request", err: extensionpkg.ErrExtensionEnvBindingInvalid, want: http.StatusBadRequest},
		{name: "Should map undeclared bindings to bad request", err: extensionpkg.ErrExtensionEnvBindingUndeclared, want: http.StatusBadRequest},
		{name: "Should map dangling bindings to bad request", err: extensionpkg.ErrExtensionEnvBindingDangling, want: http.StatusBadRequest},
		{name: "Should map missing local paths to bad request", err: os.ErrNotExist, want: http.StatusBadRequest},
		{name: "Should map unknown failures to internal error", err: errors.New("unknown"), want: http.StatusInternalServerError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := testCase.err
			if err != nil {
				err = fmt.Errorf("wrapped: %w", err)
			}
			if got := core.ExtensionStatusCode(err); got != testCase.want {
				t.Fatalf("ExtensionStatusCode() = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestExtensionKitHandlersReturnDedicatedPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should return inventory and preview payloads without mutation wrappers", func(t *testing.T) {
		t.Parallel()

		inventory := contract.ExtensionInventoryPayload{
			Extension: "kit",
			Enabled:   true,
			Items: []contract.ExtensionKitItemPayload{
				{Kind: "agent", ID: "agent/kit/writer", Name: "writer", Live: true},
			},
		}
		preview := contract.ExtensionEnablePreviewPayload{
			Extension: "kit",
			Changes: []contract.ExtensionKitChangePayload{{
				Kind: "agent", ID: "agent/kit/writer", Name: "writer", Change: contract.ExtensionKitChangeChanged,
			}},
			AutomationStarting: []string{"kit/daily"},
		}
		service := extensionServiceStub{
			inventoryFn: func(_ context.Context, name string) (contract.ExtensionInventoryPayload, error) {
				if name != "kit" {
					t.Fatalf("Inventory() name = %q, want kit", name)
				}
				return inventory, nil
			},
			previewFn: func(_ context.Context, name string) (contract.ExtensionEnablePreviewPayload, error) {
				if name != "kit" {
					t.Fatalf("Preview() name = %q, want kit", name)
				}
				return preview, nil
			},
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.GET("/extensions/:name/inventory", handlers.ExtensionInventory)
		engine.GET("/extensions/:name/preview", handlers.PreviewExtensionEnable)
		for _, testCase := range []struct {
			name string
			path string
			want any
		}{
			{name: "Should return the inventory payload", path: "/extensions/kit/inventory", want: inventory},
			{name: "Should return the enable preview payload", path: "/extensions/kit/preview", want: preview},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				response := performRequest(t, engine, http.MethodGet, testCase.path, nil)
				if response.Code != http.StatusOK {
					t.Fatalf("GET %s status = %d; body=%s", testCase.path, response.Code, response.Body.String())
				}
				got, err := json.Marshal(testCase.want)
				if err != nil {
					t.Fatalf("json.Marshal(want) error = %v", err)
				}
				if strings.TrimSpace(response.Body.String()) != string(got) {
					t.Fatalf("GET %s body = %s, want %s", testCase.path, response.Body.String(), got)
				}
			})
		}
	})

	t.Run("Should forward digest consent and return only the enable action result", func(t *testing.T) {
		t.Parallel()

		want := contract.ExtensionEnableResult{
			Extension:         contract.ExtensionPayload{Name: "kit", Enabled: true, State: "running"},
			AutomationStarted: []string{"kit/daily"},
		}
		service := extensionServiceStub{enableFn: func(
			_ context.Context,
			name string,
			req contract.EnableExtensionRequest,
			actor taskpkg.ActorContext,
		) (contract.ExtensionEnableResult, error) {
			if name != "kit" || req.ConfirmNetworkDigest != "digest-current" || !actor.Authority.Write {
				t.Fatalf("Enable() name=%q req=%#v actor=%#v", name, req, actor)
			}
			return want, nil
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.POST("/extensions/:name/enable", handlers.EnableExtension)
		response := performRequest(
			t,
			engine,
			http.MethodPost,
			"/extensions/kit/enable",
			[]byte(`{"confirm_network_digest":"digest-current"}`),
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var got contract.ExtensionEnableResult
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(enable) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("enable result = %#v, want %#v", got, want)
		}
		if strings.Contains(response.Body.String(), "bundles") ||
			strings.Contains(string(mustJSONMarshal(t, got.Extension)), "automation_started") {
			t.Fatalf("enable response leaked removed/status-only fields: %s", response.Body.String())
		}
	})

	t.Run("Should accept an empty enable body without Content-Length", func(t *testing.T) {
		t.Parallel()

		called := false
		service := extensionServiceStub{enableFn: func(
			_ context.Context,
			name string,
			req contract.EnableExtensionRequest,
			_ taskpkg.ActorContext,
		) (contract.ExtensionEnableResult, error) {
			called = true
			if name != "kit" || req != (contract.EnableExtensionRequest{}) {
				t.Fatalf("Enable() name=%q req=%#v, want kit with empty request", name, req)
			}
			return contract.ExtensionEnableResult{Extension: contract.ExtensionPayload{Name: name, Enabled: true}}, nil
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.POST("/extensions/:name/enable", handlers.EnableExtension)
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/extensions/kit/enable",
			strings.NewReader(""),
		)
		request.ContentLength = -1
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if !called {
			t.Fatal("Enable() was not called")
		}
	})
}

func TestExtensionOperationErrorPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should return current digest and retry command for network confirmation", func(t *testing.T) {
		t.Parallel()

		service := extensionServiceStub{enableFn: func(
			context.Context,
			string,
			contract.EnableExtensionRequest,
			taskpkg.ActorContext,
		) (contract.ExtensionEnableResult, error) {
			return contract.ExtensionEnableResult{}, &extensionpkg.NetworkConfirmationRequiredError{
				CurrentDigest: "digest-current",
			}
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.POST("/extensions/:name/enable", handlers.EnableExtension)
		response := performRequest(t, engine, http.MethodPost, "/extensions/kit/enable", nil)
		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionOperationErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(error) error = %v", err)
		}
		if payload.Code != diagnosticcontract.CodeExtensionNetworkConfirmRequired ||
			payload.CurrentDigest != "digest-current" || payload.Diagnostic == nil ||
			payload.Diagnostic.SuggestedCommand !=
				"compozy extension enable kit --confirm-network-requirement digest-current" {
			t.Fatalf("network error payload = %#v", payload)
		}
	})

	t.Run("Should return sorted conflicting agent names", func(t *testing.T) {
		t.Parallel()

		service := extensionServiceStub{enableFn: func(
			context.Context,
			string,
			contract.EnableExtensionRequest,
			taskpkg.ActorContext,
		) (contract.ExtensionEnableResult, error) {
			return contract.ExtensionEnableResult{}, &extensionpkg.AgentConflictError{
				Agents: []string{"writer", "coder"},
			}
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
		engine := gin.New()
		engine.POST("/extensions/:name/enable", handlers.EnableExtension)
		response := performRequest(t, engine, http.MethodPost, "/extensions/kit/enable", nil)
		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionOperationErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(error) error = %v", err)
		}
		if payload.Code != diagnosticcontract.CodeExtensionAgentConflict ||
			!reflect.DeepEqual(payload.Agents, []string{"coder", "writer"}) {
			t.Fatalf("agent conflict payload = %#v", payload)
		}
	})

	for _, testCase := range []struct {
		name string
		err  error
		code string
	}{
		{
			name: "Should map undeclared binding names",
			err: &extensionpkg.EnvBindingValidationError{
				EnvName: "SECRET_SENTINEL", Declared: []string{"TOKEN", "API_KEY"},
				Cause: extensionpkg.ErrExtensionEnvBindingUndeclared,
			},
			code: diagnosticcontract.CodeExtensionEnvBindingUndeclared,
		},
		{
			name: "Should map dangling Vault refs",
			err: &extensionpkg.EnvBindingValidationError{
				EnvName: "API_KEY", Cause: extensionpkg.ErrExtensionEnvBindingDangling,
			},
			code: diagnosticcontract.CodeExtensionEnvBindingDangling,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := extensionServiceStub{setSecretsFn: func(
				context.Context,
				string,
				contract.SetExtensionSecretsRequest,
				taskpkg.ActorContext,
			) (contract.ExtensionSecretsPayload, error) {
				return contract.ExtensionSecretsPayload{}, testCase.err
			}}
			handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
			engine := gin.New()
			engine.PUT("/extensions/:name/secrets", handlers.SetExtensionSecrets)
			response := performRequest(
				t,
				engine,
				http.MethodPut,
				"/extensions/kit/secrets",
				[]byte(`{"secrets":{"API_KEY":{"value":"planted-secret-value"}}}`),
			)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", response.Code, response.Body.String())
			}
			var payload contract.ExtensionOperationErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal(error) error = %v", err)
			}
			if payload.Code != testCase.code || payload.EnvName == "" ||
				strings.Contains(response.Body.String(), "planted-secret-value") {
				t.Fatalf("binding error payload = %#v body=%s", payload, response.Body.String())
			}
			if errors.Is(testCase.err, extensionpkg.ErrExtensionEnvBindingUndeclared) &&
				!reflect.DeepEqual(payload.DeclaredEnv, []string{"API_KEY", "TOKEN"}) {
				t.Fatalf("declared env = %#v, want sorted names", payload.DeclaredEnv)
			}
		})
	}
}

func mustJSONMarshal(t *testing.T, value any) []byte {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}

func TestDevelopmentExtensionHandlersBindTrustedWorkspace(t *testing.T) {
	t.Parallel()

	trustedActor, err := taskpkg.DeriveAgentSessionActorContext("session-a", "workspace-a")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
	}

	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		body       []byte
		configure  func(*extensionServiceStub, func(taskpkg.ActorContext))
		wantStatus int
	}{
		{
			name:       "Should ignore a forged workspace in the dev body",
			method:     http.MethodPost,
			path:       "/extensions/dev",
			body:       []byte(`{"origin_path":"/workspace/a/ext","generation_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","workspace_id":"workspace-b"}`),
			wantStatus: http.StatusCreated,
			configure: func(service *extensionServiceStub, record func(taskpkg.ActorContext)) {
				service.devFn = func(
					_ context.Context,
					_ contract.DevLinkExtensionRequest,
					actor taskpkg.ActorContext,
				) (contract.ExtensionPayload, error) {
					record(actor)
					return contract.ExtensionPayload{Name: "bound"}, nil
				}
			},
		},
		{
			name:       "Should ignore a forged workspace in the reload body",
			method:     http.MethodPost,
			path:       "/extensions/bound/reload",
			body:       []byte(`{"generation_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","workspace_id":"workspace-b"}`),
			wantStatus: http.StatusOK,
			configure: func(service *extensionServiceStub, record func(taskpkg.ActorContext)) {
				service.reloadDevFn = func(
					_ context.Context,
					_ string,
					_ contract.ReloadExtensionRequest,
					actor taskpkg.ActorContext,
				) (contract.ExtensionPayload, error) {
					record(actor)
					return contract.ExtensionPayload{Name: "bound"}, nil
				}
			},
		},
		{
			name:       "Should ignore a forged workspace query for agent logs",
			method:     http.MethodGet,
			path:       "/extensions/bound/logs?workspace=workspace-b",
			wantStatus: http.StatusOK,
			configure: func(service *extensionServiceStub, record func(taskpkg.ActorContext)) {
				service.logsFn = func(
					_ context.Context,
					_ string,
					_ int64,
					actor taskpkg.ActorContext,
				) ([]contract.ExtensionLogPayload, error) {
					record(actor)
					return []contract.ExtensionLogPayload{}, nil
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var recorded taskpkg.ActorContext
			service := extensionServiceStub{}
			testCase.configure(&service, func(actor taskpkg.ActorContext) { recorded = actor })
			handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
				TransportName: "http",
				Extensions:    service,
				TaskActorContextResolver: func(_ *gin.Context, _ string) (taskpkg.ActorContext, error) {
					return trustedActor, nil
				},
			})
			engine := gin.New()
			engine.POST("/extensions/dev", handlers.DevExtension)
			engine.POST("/extensions/:name/reload", handlers.ReloadDevExtension)
			engine.GET("/extensions/:name/logs", handlers.ExtensionLogs)

			response := performRequest(t, engine, testCase.method, testCase.path, testCase.body)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, testCase.wantStatus, response.Body.String())
			}
			if recorded.Scope.WorkspaceID != "workspace-a" || recorded.Scope.Operator {
				t.Fatalf("recorded actor scope = %#v, want trusted agent workspace A", recorded.Scope)
			}
		})
	}

	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		body       []byte
		wantStatus int
		configure  func(*extensionServiceStub, *bool)
	}{
		{
			name:       "Should reject agent dev links when the trusted session has no workspace",
			method:     http.MethodPost,
			path:       "/extensions/dev",
			body:       []byte(`{"origin_path":"/workspace/a/ext","generation_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
			wantStatus: http.StatusBadRequest,
			configure: func(service *extensionServiceStub, called *bool) {
				service.devFn = func(
					_ context.Context,
					_ contract.DevLinkExtensionRequest,
					_ taskpkg.ActorContext,
				) (contract.ExtensionPayload, error) {
					*called = true
					return contract.ExtensionPayload{Name: "bound"}, nil
				}
			},
		},
		{
			name:       "Should reject agent reloads when the trusted session has no workspace",
			method:     http.MethodPost,
			path:       "/extensions/bound/reload",
			body:       []byte(`{"generation_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`),
			wantStatus: http.StatusBadRequest,
			configure: func(service *extensionServiceStub, called *bool) {
				service.reloadDevFn = func(
					_ context.Context,
					_ string,
					_ contract.ReloadExtensionRequest,
					_ taskpkg.ActorContext,
				) (contract.ExtensionPayload, error) {
					*called = true
					return contract.ExtensionPayload{Name: "bound"}, nil
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			called := false
			service := extensionServiceStub{}
			testCase.configure(&service, &called)
			sessions := testutil.StubSessionManager{
				StatusFn: func(_ context.Context, id string) (*sessionpkg.Info, error) {
					return &sessionpkg.Info{
						ID: id, AgentName: "coder", State: sessionpkg.StateActive,
					}, nil
				},
			}
			handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
				TransportName: "http",
				Extensions:    service,
				Sessions:      sessions,
			})
			engine := gin.New()
			engine.POST("/extensions/dev", handlers.DevExtension)
			engine.POST("/extensions/:name/reload", handlers.ReloadDevExtension)

			response := testutil.PerformRequestWithHeaders(
				t,
				engine,
				testCase.method,
				testCase.path,
				testCase.body,
				map[string]string{
					agentidentity.HeaderSessionID: "session-without-workspace",
					agentidentity.HeaderAgent:     "coder",
				},
			)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, testCase.wantStatus, response.Body.String())
			}
			if called {
				t.Fatal("extension dev service called without a trusted workspace")
			}
			if !strings.Contains(response.Body.String(), "workspace_id is required") {
				t.Fatalf("body = %s, want trusted workspace validation", response.Body.String())
			}
		})
	}
}

func TestExtensionHandlersHaveHTTPUDSParity(t *testing.T) {
	t.Parallel()

	const (
		apiWorkspaceRootCanary = "/private/API-WORKSPACE-ROOT-CANARY"
		apiOriginCanary        = apiWorkspaceRootCanary + "/API-ORIGIN-CANARY"
		apiRawCauseCanary      = "API-RAW-BOOT-CAUSE-CANARY"
	)
	safeBootFailure := extensionpkg.DescribeExtension(&extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{
			Name: "boot-failure", Source: extensionpkg.SourceWorkspace, Enabled: true,
		},
		Status: extensionpkg.ExtensionStatus{
			Name: "boot-failure", WorkspaceID: "workspace-a",
			LastError: apiRawCauseCanary + " at " + apiOriginCanary, FailureCode: "missing_origin",
		},
		DevLink: &extensionpkg.DevLink{
			ExtensionName: "boot-failure", WorkspaceID: "workspace-a", OriginPath: apiOriginCanary,
		},
	}, true, time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC))

	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		"workspace-a",
		taskpkg.OriginKindCLI,
		"extension parity",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	minimum, maximum := float64(1), float64(10)
	service := extensionServiceStub{
		listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
			return []contract.ExtensionPayload{safeBootFailure}, nil
		},
		searchFn: func(
			_ context.Context,
			req contract.ExtensionSearchRequest,
		) (contract.ExtensionSearchResponse, error) {
			return contract.ExtensionSearchResponse{
				Items: []contract.ExtensionSearchItem{{
					Slug: "acme/parity", Name: req.Query, Source: "github", Tier: "unverified",
				}},
			}, nil
		},
		updateBatchFn: func(
			_ context.Context,
			_ contract.UpdateExtensionsRequest,
			_ taskpkg.ActorContext,
		) ([]contract.ManagedExtensionUpdatePayload, error) {
			return []contract.ManagedExtensionUpdatePayload{
				{Name: "a-good", Status: extensionpkg.MarketplaceUpdateStatusUpdated},
				{
					Name: "z-bad", Status: extensionpkg.MarketplaceUpdateStatusFailed,
					Error: &contract.DiagnosticItem{Code: diagnosticcontract.CodeExtensionUpdateFailed},
				},
			}, &extensionpkg.MarketplaceUpdateBatchError{FailedName: "z-bad", Cause: errors.New("upstream failure")}
		},
		devFn: func(
			_ context.Context,
			req contract.DevLinkExtensionRequest,
			actor taskpkg.ActorContext,
		) (contract.ExtensionPayload, error) {
			return contract.ExtensionPayload{
				Name: "parity", WorkspaceID: actor.Scope.WorkspaceID,
				OriginPath: req.OriginPath, GenerationHash: req.GenerationHash, Dev: true,
			}, nil
		},
		reloadDevFn: func(
			_ context.Context,
			name string,
			req contract.ReloadExtensionRequest,
			actor taskpkg.ActorContext,
		) (contract.ExtensionPayload, error) {
			return contract.ExtensionPayload{
				Name: name, WorkspaceID: actor.Scope.WorkspaceID,
				GenerationHash: req.GenerationHash, Dev: true,
			}, nil
		},
		logsFn: func(
			_ context.Context,
			_ string,
			_ int64,
			_ taskpkg.ActorContext,
		) ([]contract.ExtensionLogPayload, error) {
			return []contract.ExtensionLogPayload{{
				Sequence:  1,
				Timestamp: time.Date(2026, 7, 28, 13, 0, 0, 0, time.UTC),
				Message:   "ready",
			}}, nil
		},
		commandsScopedFn: func(
			_ context.Context,
			filter string,
			actor taskpkg.ActorContext,
		) (contract.ExtensionCommandsResponse, error) {
			if filter != "parity" || actor.Scope.WorkspaceID != "workspace-a" {
				return contract.ExtensionCommandsResponse{}, fmt.Errorf(
					"unexpected command scope filter=%q workspace=%q",
					filter,
					actor.Scope.WorkspaceID,
				)
			}
			return contract.ExtensionCommandsResponse{
				Commands: []contract.ExtensionCommandPayload{{
					Extension: "parity", Verb: "review/fetch", ToolID: "ext__parity__review_fetch",
					Summary: "Fetch review", RiskClass: toolspkg.RiskMutating, ApprovalRequired: true,
					Flags: []toolspkg.CommandFlag{{
						Name: "round", Field: "round", Type: toolspkg.CommandFlagInteger,
						Required: true, Enum: []string{"1", "2"}, Default: json.RawMessage(`1`),
						Minimum: &minimum, Maximum: &maximum,
					}},
				}},
				Groups: []contract.ExtensionCommandGroupPayload{{
					Extension: "parity", Path: "review", Summary: "Review commands",
				}},
			}, nil
		},
		inventoryFn: func(_ context.Context, name string) (contract.ExtensionInventoryPayload, error) {
			return contract.ExtensionInventoryPayload{
				Extension: name,
				Items:     []contract.ExtensionKitItemPayload{{Kind: "agent", Name: "writer", Live: true}},
			}, nil
		},
		previewFn: func(_ context.Context, name string) (contract.ExtensionEnablePreviewPayload, error) {
			return contract.ExtensionEnablePreviewPayload{
				Extension: name, AutomationStarting: []string{name + "/daily"},
			}, nil
		},
		listSecretsFn: func(
			_ context.Context,
			_ string,
			_ taskpkg.ActorContext,
		) (contract.ExtensionSecretsPayload, error) {
			return contract.ExtensionSecretsPayload{DeclaredEnv: []string{"API_KEY"}}, nil
		},
		setSecretsFn: func(
			_ context.Context,
			_ string,
			_ contract.SetExtensionSecretsRequest,
			_ taskpkg.ActorContext,
		) (contract.ExtensionSecretsPayload, error) {
			return contract.ExtensionSecretsPayload{
				DeclaredEnv: []string{"API_KEY"}, BoundEnvKeys: []string{"API_KEY"},
			}, nil
		},
	}
	for _, request := range []struct {
		method string
		path   string
		body   []byte
	}{
		{
			method: http.MethodPost,
			path:   "/extensions/dev",
			body: []byte(
				`{"origin_path":"/workspace/a/parity","generation_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
			),
		},
		{
			method: http.MethodPost,
			path:   "/extensions/parity/reload",
			body: []byte(
				`{"generation_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`,
			),
		},
		{method: http.MethodGet, path: "/extensions/parity/logs"},
		{method: http.MethodGet, path: "/extensions/search?q=parity&sources=github&limit=1"},
		{method: http.MethodGet, path: "/extensions"},
		{method: http.MethodGet, path: "/extensions/commands?extension=parity&workspace=workspace-a"},
		{method: http.MethodPost, path: "/extensions/update", body: []byte(`{"all":true}`)},
		{method: http.MethodGet, path: "/extensions/parity/inventory"},
		{method: http.MethodGet, path: "/extensions/parity/preview"},
		{method: http.MethodGet, path: "/extensions/parity/secrets"},
		{
			method: http.MethodPut,
			path:   "/extensions/parity/secrets",
			body:   []byte(`{"secrets":{"API_KEY":{"value":"transport-secret"}}}`),
		},
		{method: http.MethodDelete, path: "/extensions/parity/secrets/API_KEY"},
	} {
		t.Run("Should return identical payloads for "+request.method+" "+request.path, func(t *testing.T) {
			t.Parallel()

			responses := make(map[string]struct {
				status int
				body   string
			})
			for _, transport := range []string{"http", "uds"} {
				handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
					TransportName: transport,
					Extensions:    service,
					TaskActorContextResolver: func(_ *gin.Context, _ string) (taskpkg.ActorContext, error) {
						return actor, nil
					},
				})
				engine := gin.New()
				engine.POST("/extensions/dev", handlers.DevExtension)
				engine.POST("/extensions/:name/reload", handlers.ReloadDevExtension)
				engine.POST("/extensions/update", handlers.UpdateExtensions)
				engine.GET("/extensions", handlers.ListExtensions)
				engine.GET("/extensions/:name/logs", handlers.ExtensionLogs)
				engine.GET("/extensions/search", handlers.SearchExtensions)
				engine.GET("/extensions/commands", handlers.ExtensionCommands)
				engine.GET("/extensions/:name/inventory", handlers.ExtensionInventory)
				engine.GET("/extensions/:name/preview", handlers.PreviewExtensionEnable)
				engine.GET("/extensions/:name/secrets", handlers.ListExtensionSecrets)
				engine.PUT("/extensions/:name/secrets", handlers.SetExtensionSecrets)
				engine.DELETE("/extensions/:name/secrets/:env_name", handlers.DeleteExtensionSecret)
				response := performRequest(t, engine, request.method, request.path, request.body)
				responses[transport] = struct {
					status int
					body   string
				}{status: response.Code, body: response.Body.String()}
			}
			if !reflect.DeepEqual(responses["http"], responses["uds"]) {
				t.Fatalf("transport responses differ: %#v", responses)
			}
			if request.path == "/extensions" {
				body := responses["http"].body
				for _, forbidden := range []string{apiWorkspaceRootCanary, apiOriginCanary, apiRawCauseCanary} {
					if strings.Contains(body, forbidden) {
						t.Fatalf("extension inventory body leaked %q: %s", forbidden, body)
					}
				}
				if strings.Contains(body, `"origin_path"`) ||
					!strings.Contains(body, `"failure_code":"missing_origin"`) ||
					!strings.Contains(body, `"last_error":"extension development origin is unavailable"`) {
					t.Fatalf("extension inventory body = %s, want safe boot-failure projection", body)
				}
			}
			if strings.Contains(request.path, "/extensions/commands") {
				var payload contract.ExtensionCommandsResponse
				if err := json.Unmarshal([]byte(responses["http"].body), &payload); err != nil {
					t.Fatalf("json.Unmarshal(commands) error = %v", err)
				}
				if len(payload.Commands) != 1 || len(payload.Groups) != 1 {
					t.Fatalf("commands payload = %#v", payload)
				}
				flag := payload.Commands[0].Flags[0]
				if flag.Type != toolspkg.CommandFlagInteger || !flag.Required || flag.Nullable ||
					len(flag.Enum) != 2 || string(flag.Default) != "1" || flag.Minimum == nil || flag.Maximum == nil ||
					!payload.Commands[0].ApprovalRequired {
					t.Fatalf("command flag contract = %#v; command=%#v", flag, payload.Commands[0])
				}
			}
		})
	}
}

func TestListExtensionsErrorResponses(t *testing.T) {
	t.Parallel()

	t.Run("Should return service unavailable when the extension service is not configured", func(t *testing.T) {
		t.Parallel()

		homePaths := testutil.NewTestHomePaths(t)
		cfg := testConfigWithDisabledNetwork(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "api-core-test",
			HomePaths:     homePaths,
			Config:        cfg,
			Logger:        testutil.DiscardLogger(),
			StartedAt:     time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			Now: func() time.Time {
				return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC)
			},
			HTTPPort: cfg.HTTP.Port,
		})

		engine := gin.New()
		engine.Use(gin.Recovery())
		engine.GET("/extensions", handlers.ListExtensions)

		response := performRequest(t, engine, http.MethodGet, "/extensions", nil)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusServiceUnavailable,
				response.Body.String(),
			)
		}
		if !strings.Contains(response.Body.String(), "extension service is not configured") {
			t.Fatalf("body = %s, want unavailable diagnostic", response.Body.String())
		}
	})

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

func TestInstallExtensionReturnsDiagnosticsOverHTTP(t *testing.T) {
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
			[]byte(`{"source":"local_path","ref":"/tmp/blocked","allow_unverified":true}`),
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
			"extensions.trust.allow_unverified",
		} {
			if !strings.Contains(response.Body.String(), want) {
				t.Fatalf("body = %q, want %q", response.Body.String(), want)
			}
		}
	})

	for _, testCase := range []struct {
		name        string
		installErr  error
		wantCode    string
		wantMessage string
	}{
		{
			name:        "Should return the missing Git dependency diagnostic over HTTP",
			installErr:  registrygit.ErrGitUnavailable,
			wantCode:    diagnosticcontract.CodeExtensionGitUnavailable,
			wantMessage: "Install Git",
		},
		{
			name:        "Should return the unsupported Git version diagnostic over HTTP",
			installErr:  registrygit.ErrGitVersionUnsupported,
			wantCode:    diagnosticcontract.CodeExtensionGitVersionUnsupported,
			wantMessage: "Git 2.37 or newer",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
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
					return contract.ExtensionPayload{}, fmt.Errorf("install extension: %w", testCase.installErr)
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
				[]byte(`{"source":"git","ref":"https://github.com/acme/example.git"}`),
			)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf(
					"status = %d, want %d; body=%s",
					response.Code,
					http.StatusServiceUnavailable,
					response.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal(response) error = %v", err)
			}
			if payload.Diagnostic == nil ||
				payload.Diagnostic.Code != testCase.wantCode ||
				payload.Diagnostic.SuggestedCommand != "git --version" ||
				!strings.Contains(payload.Diagnostic.Message, testCase.wantMessage) {
				t.Fatalf("diagnostic = %#v, want registered Git recovery diagnostic", payload.Diagnostic)
			}
		})
	}
}
