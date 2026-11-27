package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	apispec "github.com/compozy/compozy/internal/api/spec"
	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/gin-gonic/gin"
)

// The app-token listener ignores the retired device cookie.
type appTokenHTTPAuthenticatorStub struct {
	authenticate func(context.Context, string) (apptoken.Token, error)
	origin       func(string) bool
	touch        func(context.Context, string, uint64) error
	acquire      func(context.Context, string, uint64) (context.Context, error)
	revalidate   func(context.Context, string, uint64) error
	registry     remotehost.ConnectionRegistry
}

func (s appTokenHTTPAuthenticatorStub) AuthenticateAppToken(
	ctx context.Context,
	secret string,
) (apptoken.Token, error) {
	if s.authenticate == nil {
		return apptoken.Token{}, apptoken.ErrUnauthenticated
	}
	return s.authenticate(ctx, secret)
}

func (s appTokenHTTPAuthenticatorStub) AppTokenOriginAllowed(origin string) bool {
	return s.origin != nil && s.origin(origin)
}

func (s appTokenHTTPAuthenticatorStub) TouchAppTokenLastSeen(ctx context.Context, id string, epoch uint64) error {
	if s.touch != nil {
		return s.touch(ctx, id, epoch)
	}
	return nil
}

func (s appTokenHTTPAuthenticatorStub) AcquireMutation(
	ctx context.Context,
	id string,
	epoch uint64,
) (context.Context, error) {
	if s.acquire != nil {
		return s.acquire(ctx, id, epoch)
	}
	return ctx, nil
}

func (s appTokenHTTPAuthenticatorStub) RevalidateForCommit(ctx context.Context, id string, epoch uint64) error {
	if s.revalidate != nil {
		return s.revalidate(ctx, id, epoch)
	}
	return nil
}

func (s appTokenHTTPAuthenticatorStub) Connections() remotehost.ConnectionRegistry { return s.registry }

func TestAppTokenBrowserAdmission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, method, path, origin, credential string
		allowedOrigin                          bool
		authError                              error
		wantStatus                             int
		wantCode, wantOrigin                   string
		wantTouches                            int
	}{
		{
			name:       "Should preserve tokenless no-origin reads",
			method:     http.MethodGet,
			path:       "/api/settings/general",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Should preserve tokenless own-origin mutations",
			method:     http.MethodPatch,
			path:       "/api/settings/general",
			origin:     "http://127.0.0.1:2123",
			wantStatus: http.StatusOK,
			wantOrigin: "http://127.0.0.1:2123",
		},
		{name: "Should reject invalid app credentials without an Origin", method: http.MethodGet,
			credential: "cpz_app_invalid", authError: apptoken.ErrUnauthenticated,
			wantStatus: http.StatusUnauthorized, wantCode: "app_token_unauthenticated"},
		{
			name:          "Should accept normalized foreign-origin preflight without authentication",
			method:        http.MethodOptions,
			origin:        "HTTP://A.TEST:80/",
			allowedOrigin: true,
			wantStatus:    http.StatusNoContent,
			wantOrigin:    "HTTP://A.TEST:80/",
		},
		{name: "Should reject preflight outside the active origin union", method: http.MethodOptions,
			origin: "http://a.test", wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{name: "Should reject opaque origins even when the origin union accepts them", method: http.MethodOptions,
			origin: "null", allowedOrigin: true, wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{name: "Should reject an ungranted foreign origin without credentials", method: http.MethodGet,
			origin: "http://a.test", wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{name: "Should reject opaque actual-request origins without credentials", method: http.MethodGet,
			origin: "null", wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{
			name:          "Should require a bearer from an allowed foreign origin",
			method:        http.MethodGet,
			origin:        "http://a.test",
			allowedOrigin: true,
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "app_token_required",
			wantOrigin:    "http://a.test",
		},
		{name: "Should deny a token whose own allowlist lacks the origin", method: http.MethodGet,
			origin: "http://b.test", allowedOrigin: true, credential: "cpz_app_valid",
			wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed", wantOrigin: "http://b.test"},
		{name: "Should admit a foreign-origin mutation after bearer authorization", method: http.MethodPost,
			origin: "http://a.test", allowedOrigin: true, credential: "cpz_app_valid",
			wantStatus: http.StatusOK, wantOrigin: "http://a.test", wantTouches: 1},
		{
			name:        "Should authenticate and record a no-origin app bearer",
			method:      http.MethodPatch,
			path:        "/api/settings/general",
			credential:  "cpz_app_valid",
			wantStatus:  http.StatusOK,
			wantTouches: 1,
		},
		{name: "Should reject opaque actual-request origins", method: http.MethodGet,
			origin: "null", credential: "cpz_app_valid", allowedOrigin: true,
			wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{
			name:          "Should reject malformed foreign-origin credentials",
			method:        http.MethodGet,
			origin:        "http://a.test",
			credential:    "cpz_app_bad",
			allowedOrigin: true,
			authError:     apptoken.ErrUnauthenticated,
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "app_token_unauthenticated",
			wantOrigin:    "http://a.test",
		},
		{
			name:          "Should report revoked credentials without recording use",
			method:        http.MethodGet,
			origin:        "http://a.test",
			allowedOrigin: true,
			credential:    "cpz_app_revoked",
			authError:     apptoken.ErrRevoked,
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "app_token_revoked",
			wantOrigin:    "http://a.test",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			calls, touches := 0, 0
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(origin string) bool {
					if origin != "http://a.test" && origin != "http://b.test" {
						t.Errorf("origin lookup = %q, want normalized origin", origin)
					}
					return test.allowedOrigin
				},
				authenticate: func(context.Context, string) (apptoken.Token, error) {
					return apptoken.Token{ID: "apt_browser", RevokeEpoch: 3,
						Origins: []string{"http://a.test"}}, test.authError
				},
				touch: func(_ context.Context, id string, epoch uint64) error {
					touches++
					if id != "apt_browser" || epoch != 3 {
						t.Errorf("touch = %s/%d, want authenticated token epoch", id, epoch)
					}
					return nil
				},
			}
			router := newAppTokenTestRouter(
				t, appTokenTestHost,
				auth,
				func(_ string, path string) bool { return path == "/api/sessions" },
				nil,
			)
			path := test.path
			if path == "" {
				path = "/api/sessions"
			}
			router.Handle(test.method, path, func(c *gin.Context) {
				calls++
				c.JSON(http.StatusOK, gin.H{"admitted": true})
			})
			request := appTokenTestRequest(t, test.method, path, test.origin, test.credential)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertAppTokenHTTPResponse(t, response, test.wantStatus, test.wantCode)
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin {
				t.Fatalf("Allow-Origin = %q, want %q", got, test.wantOrigin)
			}
			if touches != test.wantTouches {
				t.Fatalf("last_seen touches = %d, want %d", touches, test.wantTouches)
			}
			if test.wantStatus == http.StatusOK {
				if calls != 1 || !strings.Contains(response.Body.String(), `"admitted":true`) {
					t.Fatalf("admitted response = calls:%d body:%s", calls, response.Body.String())
				}
				if !strings.Contains(
					response.Header().Get("Access-Control-Expose-Headers"),
					"x-vercel-ai-ui-message-stream",
				) {
					t.Fatal("stream protocol header is not exposed")
				}
			} else if calls != 0 {
				t.Fatalf("handler calls = %d, want zero", calls)
			}
			if test.wantStatus == http.StatusNoContent {
				if response.Body.Len() != 0 || response.Header().Get("Access-Control-Max-Age") != "600" ||
					!strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Authorization") ||
					!strings.Contains(response.Header().Get("Vary"), "Origin") {
					t.Fatalf("preflight = headers:%v body:%s", response.Header(), response.Body.String())
				}
			}
		})
	}
}

func TestAppTokenRouteAuthorization(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/workspaces/:workspace_id/terminals/exec"},
		{http.MethodGet, "/api/status"},
		{http.MethodPost, "/api/workspaces/resolve"},
		{http.MethodGet, "/api/fs/browse"},
		{http.MethodPatch, "/api/settings/general"},
		{http.MethodGet, "/api/vault/secrets"},
		{http.MethodPost, "/api/app-tokens"},
		{http.MethodGet, "/api/migration-notices"},
		{http.MethodGet, "/api/agent/me"},
		{http.MethodGet, "/api/unregistered"},
		{http.MethodGet, "/api/terminals"},
		{http.MethodGet, "/api/terminals/catalog-stream"},
		{http.MethodGet, "/api/worktrees"},
		{http.MethodGet, "/api/agents/aggregate"},
		{http.MethodGet, "/api/automation/suggestions"},
	} {
		t.Run("Should forbid browser access to "+route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			calls, touches := 0, 0
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(string) bool { return true },
				authenticate: func(context.Context, string) (apptoken.Token, error) {
					return apptoken.Token{
						ID:      "apt_routes",
						Origins: []string{"http://a.test"},
					}, nil
				},
				touch: func(context.Context, string, uint64) error { touches++; return nil },
			}
			router := newAppTokenTestRouter(t, appTokenTestHost, auth, func(method, path string) bool {
				return method == http.MethodGet && path == "/api/sessions"
			}, nil)
			if route.path != "/api/unregistered" {
				router.Handle(route.method, route.path, func(c *gin.Context) {
					calls++
					c.JSON(http.StatusOK, gin.H{"admitted": true})
				})
			}
			router.NoRoute(func(c *gin.Context) { calls++; c.JSON(http.StatusNotFound, gin.H{"missing": true}) })
			response := httptest.NewRecorder()
			path := strings.ReplaceAll(route.path, ":workspace_id", "ws-1")
			router.ServeHTTP(response, appTokenTestRequest(t, route.method, path, "http://a.test", "cpz_app_valid"))
			assertAppTokenHTTPResponse(t, response, http.StatusForbidden, "app_token_route_forbidden")
			if calls != 0 || touches != 0 {
				t.Fatalf("denied request side effects = handler:%d touches:%d", calls, touches)
			}
		})
	}
}

func TestAppTokenRequestLifecycle(t *testing.T) {
	t.Parallel()
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should attribute local requests and register only streams=%t", stream), func(t *testing.T) {
			t.Parallel()
			registry := remotehost.NewConnectionRegistry()
			auth := appTokenHTTPAuthenticatorStub{
				registry: registry,
				origin:   func(string) bool { return true },
				authenticate: func(context.Context, string) (apptoken.Token, error) {
					return apptoken.Token{
						ID:      "apt_lifecycle",
						Origins: []string{"http://a.test"},
					}, nil
				},
			}
			router := newAppTokenTestRouter(t, appTokenTestHost, auth, func(string, string) bool { return true }, nil)
			path := "/api/sessions"
			if stream {
				path = "/api/workspaces/ws-1/sessions/sess-1/stream"
			}
			entered, finished := make(chan struct{}), make(chan struct{})
			router.GET(path, func(c *gin.Context) {
				if token, ok := core.AppTokenFromContext(
					c.Request.Context(),
				); !ok ||
					token.ID != "apt_lifecycle" {
					t.Error("app credential was not attributed")
				}
				if _, ok := remotehost.PrincipalFromContext(c.Request.Context()); ok {
					t.Error("local app attribution acquired remote device privileges")
				}
				close(entered)
				if stream {
					<-c.Request.Context().Done()
				}
				c.Status(http.StatusOK)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			go func() {
				defer close(finished)
				router.ServeHTTP(
					httptest.NewRecorder(),
					appTokenTestRequest(t, http.MethodGet, path, "http://a.test", "cpz_app_valid").WithContext(ctx),
				)
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("request did not reach its handler")
			}
			canceled, err := registry.CancelDevice(ctx, "apt_lifecycle")
			want := 0
			if stream {
				want = 1
			}
			if err != nil || canceled != want {
				t.Fatalf("CancelDevice(app) = %d, %v, want %d", canceled, err, want)
			}
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("request did not release its stream registration")
			}
		})
	}
}

func TestAppTokenAuthenticationFailureWindows(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate origins and preserve ten failures until the sixty-second window expires", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
		limiter := remotehost.NewAuthFailureLimiter(10, time.Minute, func() time.Time { return now })
		lookups := 0
		auth := appTokenHTTPAuthenticatorStub{
			origin: func(string) bool { return true },
			authenticate: func(_ context.Context, secret string) (apptoken.Token, error) {
				lookups++
				if secret != "cpz_app_valid" {
					return apptoken.Token{}, apptoken.ErrUnauthenticated
				}
				return apptoken.Token{ID: "apt_rate",
					Origins: []string{"http://a.test", "http://b.test"}}, nil
			},
		}
		router := newAppTokenTestRouter(t, appTokenTestHost, auth, func(string, string) bool { return true }, limiter)
		router.GET("/api/sessions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"admitted": true}) })
		request := func(origin, secret string, status int, code string) {
			t.Helper()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, appTokenTestRequest(t, http.MethodGet, "/api/sessions", origin, secret))
			assertAppTokenHTTPResponse(t, response, status, code)
		}
		for range 5 {
			request("http://a.test", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		}
		request("http://a.test", "cpz_app_valid", http.StatusOK, "")
		for range 5 {
			request("http://a.test", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		}
		request("HTTP://A.TEST:80/", "cpz_app_invalid", http.StatusTooManyRequests, "app_token_rate_limited")
		if lookups != 11 {
			t.Fatalf("lookups after throttled origin = %d, want 11", lookups)
		}
		request("http://b.test", "cpz_app_valid", http.StatusOK, "")
		for range 10 {
			request("", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		}
		request("", "cpz_app_invalid", http.StatusTooManyRequests, "app_token_rate_limited")
		now = now.Add(time.Minute)
		request("http://a.test", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		request("", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
	})
}

func TestAppTokenRequestLogging(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		secret string
		err    error
		reason string
	}{
		{
			name:   "Should warn when a browser omits the required app token",
			reason: "app_token_required",
		},
		{
			name:   "Should warn about invalid credentials without exposing the secret",
			secret: "cpz_app_log_invalid_secret",
			err:    apptoken.ErrUnauthenticated,
			reason: "app_token_unauthenticated",
		},
		{
			name:   "Should warn about revoked credentials without exposing the secret",
			secret: "cpz_app_log_revoked_secret",
			err:    apptoken.ErrRevoked,
			reason: "app_token_revoked",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(string) bool { return true },
				authenticate: func(context.Context, string) (apptoken.Token, error) {
					return apptoken.Token{}, fmt.Errorf("authentication denied: %w", tc.err)
				},
			}
			server := &Server{host: "127.0.0.1:2123", appTokenAuth: auth,
				logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			router := gin.New()
			router.Use(server.appTokenAuthMiddleware())
			router.GET("/api/sessions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"admitted": true}) })
			request := appTokenTestRequest(t, http.MethodGet, "/api/sessions", "HTTP://APP.TEST:80/", tc.secret)
			request.RemoteAddr = "127.0.0.1:4000"
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertAppTokenHTTPResponse(t, response, http.StatusUnauthorized, tc.reason)
			var entry map[string]any
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatalf("decode authentication warning: %v", err)
			}
			for key, want := range map[string]string{
				"level": "WARN", "msg": "app_token.auth_failed", "origin": "http://app.test",
				"remote": request.RemoteAddr, "reason": tc.reason,
			} {
				if entry[key] != want {
					t.Errorf("warning %s = %v, want %q", key, entry[key], want)
				}
			}
			if strings.Contains(logs.String(), "cpz_app_") || strings.Contains(logs.String(), "authentication denied") {
				t.Fatalf("authentication warning exposed credential details: %s", logs.String())
			}
		})
	}
	t.Run("Should record the token identity without exposing its secret", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		auth := appTokenHTTPAuthenticatorStub{authenticate: func(context.Context, string) (apptoken.Token, error) {
			return apptoken.Token{ID: "apt_logged"}, nil
		}}
		router := gin.New()
		router.Use(requestLoggingMiddleware(logger))
		server := &Server{host: "127.0.0.1:2123", appTokenAuth: auth}
		WithAppTokenAuthLimiter(remotehost.NewAuthFailureLimiter(10, time.Minute, nil))(server)
		router.Use(server.appTokenAuthMiddleware())
		router.GET("/api/sessions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"admitted": true}) })
		response := httptest.NewRecorder()
		router.ServeHTTP(response, appTokenTestRequest(t, http.MethodGet, "/api/sessions", "", "cpz_app_log_secret"))
		assertAppTokenHTTPResponse(t, response, http.StatusOK, "")
		if !strings.Contains(logs.String(), `"app_token_id":"apt_logged"`) ||
			strings.Contains(logs.String(), "cpz_app_") {
			t.Fatalf("request log did not protect token identity: %s", logs.String())
		}
	})
}

const appTokenTestHost = "127.0.0.1:2123"

func newAppTokenTestRouter(t *testing.T, host string, auth AppTokenAuthenticator,
	allowed func(string, string) bool, limiter *remotehost.AuthFailureLimiter,
) *gin.Engine {
	t.Helper()
	if limiter == nil {
		limiter = remotehost.NewAuthFailureLimiter(10, time.Minute, nil)
	}
	router := gin.New()
	server := &Server{host: host, appTokenAuth: auth, appAccess: allowed}
	WithAppTokenAuthLimiter(limiter)(server)
	router.Use(server.appTokenAuthMiddleware())
	router.Use(corsMiddleware(host))
	router.Use(browserRequestProtectionMiddleware(host))
	return router
}

func appTokenTestRequest(t *testing.T, method, path, origin, secret string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "http://127.0.0.1:2123"+path, http.NoBody)
	if origin != "" {
		request.Header.Set("Origin", origin)
		if origin != "http://127.0.0.1:2123" {
			request.Header.Set("Sec-Fetch-Site", "cross-site")
		}
	}
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	return request
}

func assertAppTokenHTTPResponse(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("response = %d/%s, want status %d", response.Code, response.Body.String(), status)
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("credentials header unexpectedly present: %v", response.Header())
	}
	if code != "" {
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
		}
		if payload.Code != code || payload.Error == "" {
			t.Fatalf("error payload = %#v, want code %s and message", payload, code)
		}
	}
}

func TestAppTokenTaskBreadthAdmission(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"", "?workspace=ws-a", "?all_workspaces=true", "?all_workspaces=true&workspace_ids=@user"} {
		t.Run("Should preserve app access for task selector "+query, func(t *testing.T) {
			t.Parallel()
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(string) bool { return true },
				authenticate: func(context.Context, string) (apptoken.Token, error) {
					return apptoken.Token{
						ID:      "apt_breadth",
						Origins: []string{"http://a.test"},
					}, nil
				},
				touch: func(context.Context, string, uint64) error { return nil },
			}
			router := newAppTokenTestRouter(t, appTokenTestHost, auth, func(method, path string) bool {
				for _, op := range apispec.Operations() {
					if op.Method == method && op.Path == path {
						return op.AppAccess == apispec.AppAccessAllowed
					}
				}
				return false
			}, nil)
			router.GET("/api/tasks", func(c *gin.Context) {
				query, err := core.ParseTaskListQuery(c)
				if err != nil {
					core.RespondError(c, core.StatusForTaskError(err), err, false)
					return
				}
				c.JSON(http.StatusOK, query)
			})
			router.GET(
				"/api/observe/overview",
				func(c *gin.Context) { t.Error("app reached denied overview"); c.Status(http.StatusOK) },
			)
			response := httptest.NewRecorder()
			router.ServeHTTP(
				response,
				appTokenTestRequest(t, http.MethodGet, "/api/tasks"+query, "http://a.test", "cpz_app_valid"),
			)
			if response.Code != http.StatusOK {
				t.Fatalf("tasks = %d %s", response.Code, response.Body.String())
			}
			var got contract.TaskListQuery
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.AllWorkspaces != strings.Contains(query, "all_workspaces=true") {
				t.Fatalf("query = %+v", got)
			}
			response = httptest.NewRecorder()
			router.ServeHTTP(
				response,
				appTokenTestRequest(
					t,
					http.MethodGet,
					"/api/observe/overview?all_workspaces=true",
					"http://a.test",
					"cpz_app_valid",
				),
			)
			assertAppTokenHTTPResponse(t, response, http.StatusForbidden, "app_token_route_forbidden")
		})
	}
}

func TestAppTokenLoopAggregateAdmission(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/api/loops", "/api/loop-runs", "/api/loop-requests"} {
		t.Run("Should deny app tokens on "+path, func(t *testing.T) {
			t.Parallel()
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(string) bool { return true },
				authenticate: func(context.Context, string) (apptoken.Token, error) {
					return apptoken.Token{
						ID:      "apt_loops",
						Origins: []string{"http://a.test"},
					}, nil
				},
				touch: func(context.Context, string, uint64) error { return nil },
			}
			router := newAppTokenTestRouter(t, appTokenTestHost, auth, func(method, path string) bool {
				for _, op := range apispec.Operations() {
					if op.Method == method && op.Path == path {
						return op.AppAccess == apispec.AppAccessAllowed
					}
				}
				return false
			}, nil)
			router.GET(path, func(c *gin.Context) { t.Error("app reached aggregate handler"); c.Status(http.StatusOK) })
			response := httptest.NewRecorder()
			router.ServeHTTP(
				response,
				appTokenTestRequest(t, http.MethodGet, path+"?all_workspaces=true", "http://a.test", "cpz_app_valid"),
			)
			assertAppTokenHTTPResponse(t, response, http.StatusForbidden, "app_token_route_forbidden")
		})
	}
}
