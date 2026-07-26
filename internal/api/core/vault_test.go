package core_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/vault"
	"github.com/gin-gonic/gin"
)

type stubVaultService struct {
	getMetadataFn  func(context.Context, string) (vault.Metadata, error)
	listMetadataFn func(context.Context, string) ([]vault.Metadata, error)
	putSecretFn    func(context.Context, string, string, string) (vault.Metadata, error)
	deleteSecretFn func(context.Context, string) error
}

var _ core.VaultService = (*stubVaultService)(nil)

func (s *stubVaultService) GetMetadata(ctx context.Context, ref string) (vault.Metadata, error) {
	if s.getMetadataFn != nil {
		return s.getMetadataFn(ctx, ref)
	}
	return vault.Metadata{}, vault.ErrSecretNotFound
}

func (s *stubVaultService) ListMetadata(ctx context.Context, prefix string) ([]vault.Metadata, error) {
	if s.listMetadataFn != nil {
		return s.listMetadataFn(ctx, prefix)
	}
	return nil, vault.ErrSecretNotFound
}

func (s *stubVaultService) PutSecret(
	ctx context.Context,
	ref string,
	kind string,
	plaintext string,
) (vault.Metadata, error) {
	if s.putSecretFn != nil {
		return s.putSecretFn(ctx, ref, kind, plaintext)
	}
	return vault.Metadata{}, vault.ErrSecretNotFound
}

func (s *stubVaultService) DeleteSecret(ctx context.Context, ref string) error {
	if s.deleteSecretFn != nil {
		return s.deleteSecretFn(ctx, ref)
	}
	return vault.ErrSecretNotFound
}

func TestVaultHandlersListGetPutAndDeleteMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should expose safe vault metadata operations without plaintext reads", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC)
		var listedPrefix string
		var fetchedRef string
		var storedRef string
		var storedKind string
		var storedPlaintext string
		var deletedRef string

		fixture := newVaultHandlerFixture(t, &stubVaultService{
			listMetadataFn: func(_ context.Context, prefix string) ([]vault.Metadata, error) {
				listedPrefix = prefix
				return []vault.Metadata{
					{
						Ref:       "vault:sessions/sess-1/github-token",
						Kind:      "token",
						Present:   true,
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						Ref:       "vault:sessions/sess-1/slack-token",
						Kind:      "token",
						Present:   true,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}, nil
			},
			getMetadataFn: func(_ context.Context, ref string) (vault.Metadata, error) {
				fetchedRef = ref
				return vault.Metadata{
					Ref:       ref,
					Kind:      "token",
					Present:   true,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
			putSecretFn: func(_ context.Context, ref string, kind string, plaintext string) (vault.Metadata, error) {
				storedRef = ref
				storedKind = kind
				storedPlaintext = plaintext
				if strings.Contains(diagnostics.Redact(plaintext), plaintext) {
					t.Fatalf("PutSecret() observed unregistered plaintext %q at API boundary", plaintext)
				}
				return vault.Metadata{
					Ref:       ref,
					Kind:      kind,
					Present:   true,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
			deleteSecretFn: func(_ context.Context, ref string) error {
				deletedRef = ref
				return nil
			},
		})

		listResp := performRequest(
			t,
			fixture,
			http.MethodGet,
			"/api/vault/secrets?namespace=sessions",
			nil,
		)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", listResp.Code, listResp.Body.String())
		}
		if listedPrefix != "vault:sessions/" {
			t.Fatalf("ListMetadata() prefix = %q, want session namespace prefix", listedPrefix)
		}
		var listPayload contract.VaultSecretsResponse
		testutil.DecodeJSONResponse(t, listResp, &listPayload)
		if got, want := len(listPayload.Secrets), 2; got != want {
			t.Fatalf("len(secrets) = %d, want %d", got, want)
		}
		if listPayload.Secrets[0].Namespace != "sessions" || !listPayload.Secrets[0].Present {
			t.Fatalf("list secret = %#v, want session metadata", listPayload.Secrets[0])
		}

		getResp := performRequest(
			t,
			fixture,
			http.MethodGet,
			"/api/vault/secrets/metadata?ref=vault:sessions/sess-1/github-token",
			nil,
		)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get status = %d body=%s", getResp.Code, getResp.Body.String())
		}
		if fetchedRef != "vault:sessions/sess-1/github-token" {
			t.Fatalf("GetMetadata() ref = %q, want session vault ref", fetchedRef)
		}
		var getPayload contract.VaultSecretResponse
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if getPayload.Secret.Ref != fetchedRef || getPayload.Secret.Namespace != "sessions" {
			t.Fatalf("get secret = %#v, want session metadata", getPayload.Secret)
		}

		putResp := performRequest(
			t,
			fixture,
			http.MethodPut,
			"/api/vault/secrets",
			[]byte(
				`{"ref":"vault:sessions/sess-1/github-token","kind":"token","secret_value":"super-secret-token"}`,
			),
		)
		if putResp.Code != http.StatusOK {
			t.Fatalf("put status = %d body=%s", putResp.Code, putResp.Body.String())
		}
		if storedRef != "vault:sessions/sess-1/github-token" ||
			storedKind != "token" ||
			storedPlaintext != "super-secret-token" {
			t.Fatalf(
				"PutSecret() args = %q/%q/%q, want write-only secret payload",
				storedRef,
				storedKind,
				storedPlaintext,
			)
		}
		if strings.Contains(putResp.Body.String(), "super-secret-token") {
			t.Fatalf("put response leaked plaintext: %s", putResp.Body.String())
		}
		var putPayload contract.VaultSecretResponse
		testutil.DecodeJSONResponse(t, putResp, &putPayload)
		if putPayload.Secret.Ref != storedRef || !putPayload.Secret.Present {
			t.Fatalf("put secret = %#v, want stored metadata", putPayload.Secret)
		}

		deleteResp := performRequest(
			t,
			fixture,
			http.MethodDelete,
			"/api/vault/secrets?ref=vault:sessions/sess-1/github-token",
			nil,
		)
		if deleteResp.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
		}
		if deletedRef != "vault:sessions/sess-1/github-token" {
			t.Fatalf("DeleteSecret() ref = %q, want session vault ref", deletedRef)
		}
		if got := diagnostics.Redact(storedPlaintext); got != storedPlaintext {
			t.Fatalf("Redact(%q) = %q after delete, want dynamic cleanup", storedPlaintext, got)
		}
	})
}

func TestVaultHandlersRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		method    string
		path      string
		body      []byte
		wantError string
	}{
		{
			name:      "Should reject unsupported namespace filters",
			method:    http.MethodGet,
			path:      "/api/vault/secrets?namespace=unknown",
			wantError: "vault: unsupported secret ref",
		},
		{
			name:      "Should reject namespace and prefix mismatches",
			method:    http.MethodGet,
			path:      "/api/vault/secrets?namespace=sessions&prefix=vault:providers/openrouter/",
			wantError: "vault: unsupported secret ref",
		},
		{
			name:      "Should reject missing metadata ref",
			method:    http.MethodGet,
			path:      "/api/vault/secrets/metadata",
			wantError: "vault: unsupported secret ref",
		},
		{
			name:      "Should reject env refs for daemon vault writes",
			method:    http.MethodPut,
			path:      "/api/vault/secrets",
			body:      []byte(`{"ref":"env:OPENROUTER_API_KEY","kind":"api_key","secret_value":"secret"}`),
			wantError: "vault: unsupported secret ref",
		},
		{
			name:      "Should reject blank daemon vault secret values",
			method:    http.MethodPut,
			path:      "/api/vault/secrets",
			body:      []byte(`{"ref":"vault:sessions/sess-1/github-token","kind":"token","secret_value":"   "}`),
			wantError: "vault: secret value missing",
		},
		{
			name:      "Should reject missing delete ref",
			method:    http.MethodDelete,
			path:      "/api/vault/secrets",
			wantError: "vault: unsupported secret ref",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := newVaultHandlerFixture(t, &stubVaultService{
				getMetadataFn: func(context.Context, string) (vault.Metadata, error) {
					t.Fatal("GetMetadata() should not be called for invalid vault request")
					return vault.Metadata{}, nil
				},
				listMetadataFn: func(context.Context, string) ([]vault.Metadata, error) {
					t.Fatal("ListMetadata() should not be called for invalid vault request")
					return nil, nil
				},
				putSecretFn: func(context.Context, string, string, string) (vault.Metadata, error) {
					t.Fatal("PutSecret() should not be called for invalid vault request")
					return vault.Metadata{}, nil
				},
				deleteSecretFn: func(context.Context, string) error {
					t.Fatal("DeleteSecret() should not be called for invalid vault request")
					return nil
				},
			})

			resp := performRequest(t, fixture, tc.method, tc.path, tc.body)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s, want 400", resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantError) {
				t.Fatalf("error body = %s, want %q", resp.Body.String(), tc.wantError)
			}
		})
	}
}

func newVaultHandlerFixture(t *testing.T, vaultService core.VaultService) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:      "api-core-test",
		MaskInternalErrors: false,
		Vault:              vaultService,
		HomePaths:          homePaths,
		Config:             cfg,
		Logger:             testutil.DiscardLogger(),
	})

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.GET("/api/vault/secrets", handlers.ListVaultSecrets)
	engine.GET("/api/vault/secrets/metadata", handlers.GetVaultSecretMetadata)
	engine.PUT("/api/vault/secrets", handlers.PutVaultSecret)
	engine.DELETE("/api/vault/secrets", handlers.DeleteVaultSecret)
	return engine
}
