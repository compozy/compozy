//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/gin-gonic/gin"
)

func TestAppTokenRevocationFencesInflightMutationIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should roll back a browser mutation when app-token revocation wins", func(t *testing.T) {
		t.Parallel()
		exerciseAppTokenRevocation(t)
	})
}

func exerciseAppTokenRevocation(t *testing.T) {
	t.Helper()
	database, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.WithoutCancel(t.Context())); err != nil {
			t.Error(err)
		}
	})
	service := newIntegrationAppTokenService(t, database)
	issued, err := service.CreateAppToken(t.Context(), "Browser integration", []string{"http://app.test"})
	if err != nil {
		t.Fatal(err)
	}
	const revokeRoute = "/api/app-tokens/:app_token_id"
	api := &Server{host: "127.0.0.1", httpExtendedServices: httpExtendedServices{
		appTokenAuth: service, appAccess: func(string, string) bool { return true },
	}}
	middleware := api.appTokenAuthMiddleware()
	handlers := newHandlers(&handlerConfig{appTokens: service, boundHost: "127.0.0.1"})
	credentialID, credential := issued.Token.ID, issued.Secret
	productDB, err := store.OpenSQLiteDatabase(
		t.Context(),
		filepath.Join(t.TempDir(), "product.db"),
		func(ctx context.Context, db *sql.DB) error {
			_, createErr := db.ExecContext(ctx, `CREATE TABLE mutations (id TEXT PRIMARY KEY)`)
			return createErr
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if err := productDB.Close(); err != nil {
			t.Errorf("productDB.Close() error = %v", err)
		}
	})

	mutationStarted := make(chan struct{})
	router := gin.New()
	router.POST("/mutation", middleware, func(c *gin.Context) {
		detached := context.WithoutCancel(c.Request.Context())
		err := store.ExecuteWrite(detached, productDB, func(ctx context.Context, tx *store.WriteTx) error {
			if _, execErr := tx.ExecContext(ctx, `INSERT INTO mutations (id) VALUES ('partial')`); execErr != nil {
				return execErr
			}
			close(mutationStarted)
			<-c.Request.Context().Done()
			return nil
		})
		if err != nil {
			core.RespondError(c, http.StatusInternalServerError, err, true)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.DELETE(revokeRoute, middleware, handlers.RevokeAppToken)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	type responseOutcome struct {
		status int
		body   []byte
		err    error
	}
	mutationDone := make(chan responseOutcome, 1)
	go func() {
		request, requestErr := http.NewRequestWithContext(
			t.Context(), http.MethodPost, server.URL+"/mutation", http.NoBody,
		)
		if requestErr != nil {
			mutationDone <- responseOutcome{err: requestErr}
			return
		}
		request.Header.Set("Authorization", "Bearer "+credential)
		request.Header.Set("Origin", "http://app.test")
		response, requestErr := server.Client().Do(request)
		if requestErr != nil {
			mutationDone <- responseOutcome{err: requestErr}
			return
		}
		body, readErr := io.ReadAll(response.Body)
		mutationDone <- responseOutcome{
			status: response.StatusCode,
			body:   body,
			err:    errors.Join(readErr, response.Body.Close()),
		}
	}()
	select {
	case <-mutationStarted:
	case early := <-mutationDone:
		t.Fatalf(
			"mutation ended before entering its write: status=%d body=%s err=%v",
			early.status,
			early.body,
			early.err,
		)
	case <-t.Context().Done():
		t.Fatal("test context ended before mutation admission")
	}

	revokeRequest, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		server.URL+strings.ReplaceAll(revokeRoute, ":app_token_id", credentialID),
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(revoke) error = %v", err)
	}
	revokeResponse, err := server.Client().Do(revokeRequest)
	if err != nil {
		t.Fatalf("revoke request error = %v", err)
	}
	revokeBody := readAndCloseHTTPBody(t, revokeResponse)
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("revoke response = %d/%s", revokeResponse.StatusCode, revokeBody)
	}

	mutation := <-mutationDone
	if mutation.err != nil {
		t.Fatalf("mutation request error = %v", mutation.err)
	}
	var payload contract.ErrorPayload
	if err := json.Unmarshal(mutation.body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(mutation error) = %v; body=%s", err, mutation.body)
	}
	const wantCode = "app_token_revoked"
	if mutation.status != http.StatusUnauthorized || payload.Code != wantCode {
		t.Fatalf("mutation response = %d/%#v, want 401 %s", mutation.status, payload, wantCode)
	}
	var count int
	if err := productDB.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM mutations`).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext(mutation count) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("mutations count = %d, want rollback with no partial write", count)
	}
}

func newIntegrationAppTokenService(t *testing.T, database *globaldb.GlobalDB) *apptoken.Service {
	t.Helper()
	service, err := apptoken.NewService(database, remotehost.NewMutationGate(database.RevalidateAppTokenEpoch),
		remotehost.NewConnectionRegistry(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.LoadAppTokenOrigins(t.Context()); err != nil {
		t.Fatal(err)
	}
	return service
}

// IT-001. Keep the 0.7.8 credentials and exact token origins across the upgrade and local HTTP auth.
func TestAppTokenGatewayUpgradeIntegration(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should migrate only app credentials and authenticate an existing token from its own origin",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
			prior, err := remotetest.OpenLegacyGlobalDB(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			const at = "2026-11-24T00:00:00Z"
			const revokedAt = "2026-11-24T01:00:00Z"
			secret, err := remotehost.GenerateOpaqueSecret(bytes.NewReader(bytes.Repeat([]byte{0x58}, 32)), "cpz_app_")
			if err != nil {
				t.Fatal(err)
			}
			activeHash := remotehost.HashCredential(secret)
			rows := []struct{ id, name, kind, hash, revoked string }{
				{"apt_active", "Dashboard", "app_token", activeHash, ""},
				{"apt_revoked", "Past integration", "app_token", strings.Repeat("a", 64), revokedAt},
				{"device_1", "Phone", "operator_device", strings.Repeat("b", 64), ""},
				{"device_2", "Laptop", "operator_device", strings.Repeat("c", 64), ""},
				{"device_3", "Tablet", "operator_device", strings.Repeat("d", 64), revokedAt},
				{"profile_1", "CLI", "cli_profile", strings.Repeat("e", 64), ""},
			}
			for _, row := range rows {
				if _, err := prior.ExecContext(ctx, `INSERT INTO gateway_device_sessions
				(id, name, token_hash, actor_kind, pairing_origin, revoke_epoch, created_at, last_seen_at, revoked_at)
				VALUES (?, ?, ?, ?, 'local', ?, ?, ?, NULLIF(?, ''))`,
					row.id, row.name, row.hash, row.kind, 2, at, at, row.revoked); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := prior.ExecContext(ctx, `INSERT INTO gateway_device_origins (device_id, origin, created_at)
			VALUES ('apt_active', 'https://a.example', ?), ('apt_active', 'https://b.example', ?),
			('device_1', 'https://phone.example', ?), ('device_2', 'https://laptop.example', ?),
			('device_3', 'https://tablet.example', ?)`, at, at, at, at, at); err != nil {
				t.Fatal(err)
			}
			if _, err := prior.ExecContext(ctx, `INSERT INTO gateway_providers (name, install_source)
			VALUES ('tailscale', 'builtin'), ('external', 'local')`); err != nil {
				t.Fatal(err)
			}
			if _, err := prior.ExecContext(ctx, `INSERT INTO extensions
			(name, version, source, manifest_path, installed_at, provides_json, checksum)
			VALUES ('tailscale', '0.1.0', 'bundled', 'extensions/tailscale/extension.json',
			        ?, '["connectivity.provider"]', 'old-package')`, at); err != nil {
				t.Fatal(err)
			}
			if err := prior.Close(); err != nil {
				t.Fatal(err)
			}
			database, err := globaldb.OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := database.Close(context.WithoutCancel(ctx)); err != nil {
					t.Error(err)
				}
			})
			for _, want := range rows[:2] {
				var name, hash, revoked, seen string
				var epoch int
				if err := database.ReadDB().QueryRowContext(ctx, `SELECT name, token_hash,
				coalesce(revoked_at, ''), last_seen_at, revoke_epoch FROM app_tokens WHERE id = ?`, want.id).
					Scan(&name, &hash, &revoked, &seen, &epoch); err != nil {
					t.Fatal(err)
				}
				if name != want.name || hash != want.hash || revoked != want.revoked || seen != at || epoch != 2 {
					t.Fatalf("token %s metadata changed during upgrade", want.id)
				}
			}
			for query, want := range map[string]int{
				"SELECT count(*) FROM extensions WHERE name = 'tailscale'":                          0,
				"SELECT count(*) FROM app_tokens":                                                   2,
				"SELECT count(*) FROM app_token_origins":                                            2,
				"SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name GLOB 'gateway_*'": 0,
			} {
				var got int
				if err := database.ReadDB().QueryRowContext(ctx, query).Scan(&got); err != nil || got != want {
					t.Fatalf("%s = %d, %v, want %d", query, got, err, want)
				}
			}
			notices, err := database.ListMigrationNotices(ctx)
			if err != nil || len(notices) != 1 || notices[0].Key != "gateway_removed" ||
				notices[0].Payload.Devices != 4 || notices[0].Payload.Providers != 2 {
				t.Fatalf("migration notices = %+v, %v", notices, err)
			}

			service := newIntegrationAppTokenService(t, database)
			router := newAppTokenTestRouter(t, service, func(string, string) bool { return true }, nil)
			router.GET("/api/sessions", func(c *gin.Context) {
				token, ok := core.AppTokenFromContext(c.Request.Context())
				if !ok {
					t.Error("app token was not attributed to the request")
				}
				c.JSON(http.StatusOK, map[string]string{"token_id": token.ID})
			})
			server := httptest.NewServer(router)
			t.Cleanup(server.Close)
			for _, tc := range []struct {
				origin string
				status int
				body   string
			}{
				{"https://a.example", http.StatusOK, "apt_active"},
				{"https://b.example", http.StatusOK, "apt_active"},
				{"https://evil.example", http.StatusForbidden, "app_origin_not_allowed"},
			} {
				response := mustHTTPRequest(t, server.Client(), http.MethodGet, server.URL+"/api/sessions",
					nil, map[string]string{"Origin": tc.origin, "Authorization": "Bearer " + secret})
				body := readAndCloseHTTPBody(t, response)
				if response.StatusCode != tc.status || !bytes.Contains(body, []byte(tc.body)) {
					t.Fatalf("origin %s: status = %d body = %s", tc.origin, response.StatusCode, body)
				}
			}
		},
	)
}
