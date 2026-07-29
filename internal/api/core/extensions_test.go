package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	devFn              func(context.Context, contract.DevLinkExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	reloadDevFn        func(context.Context, string, contract.ReloadExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	logsFn             func(context.Context, string, int64, taskpkg.ActorContext) ([]contract.ExtensionLogPayload, error)
	listScopedFn       func(context.Context, taskpkg.ActorContext) ([]contract.ExtensionPayload, error)
	statusScopedFn     func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	removeScopedFn     func(context.Context, string, taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
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

func TestExtensionStatusCodeMapsDevelopmentErrors(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name string
		err  error
		want int
	}{
		{name: "Should map a missing development origin to conflict", err: extensionpkg.ErrExtensionDevOriginMissing, want: http.StatusConflict},
		{name: "Should map a missing development link to conflict", err: extensionpkg.ErrExtensionNotDevLinked, want: http.StatusConflict},
		{name: "Should map an invalid generation to bad request", err: extensionpkg.ErrExtensionGenerationInvalid, want: http.StatusBadRequest},
		{name: "Should map cross-workspace access to forbidden", err: extensionpkg.ErrExtensionWorkspaceDenied, want: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := core.ExtensionStatusCode(fmt.Errorf("wrapped: %w", testCase.err)); got != testCase.want {
				t.Fatalf("ExtensionStatusCode() = %d, want %d", got, testCase.want)
			}
		})
	}
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
}

func TestDevelopmentExtensionHandlersHaveHTTPUDSParity(t *testing.T) {
	t.Parallel()

	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		"workspace-a",
		taskpkg.OriginKindCLI,
		"extension parity",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	service := extensionServiceStub{
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
				engine.GET("/extensions/:name/logs", handlers.ExtensionLogs)
				response := performRequest(t, engine, request.method, request.path, request.body)
				responses[transport] = struct {
					status int
					body   string
				}{status: response.Code, body: response.Body.String()}
			}
			if !reflect.DeepEqual(responses["http"], responses["uds"]) {
				t.Fatalf("transport responses differ: %#v", responses)
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
			"extensions.trust.allow_unverified",
		} {
			if !strings.Contains(response.Body.String(), want) {
				t.Fatalf("body = %q, want %q", response.Body.String(), want)
			}
		}
	})
}
