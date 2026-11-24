package httpapi

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	apispec "github.com/compozy/compozy/internal/api/spec"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/gin-gonic/gin"
)

// UT-039. Safety Invariants 5, 11 and 20.
// The HTTP route table owns scope enforcement and remote rejection before the SPA.
func TestRemoteOperatorRoutes(t *testing.T) {
	t.Run("Should require a declared scope on every mounted operator route", func(t *testing.T) {
		t.Parallel()
		router := newRemoteOperatorRouter(newHandlers(nil), false)
		routes := router.Routes()
		if len(routes) == 0 {
			t.Fatal("the tunnel has no operator routes")
		}
		for _, route := range routes {
			parts := strings.Split(route.Path, "/")
			for i, part := range parts {
				if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
					parts[i] = "fixture"
				}
			}
			path := strings.Join(parts, "/")
			if err := remotehost.OperatorRouteRejection(path, false); err != nil {
				t.Fatalf("registered a forbidden route %s %s", route.Method, route.Path)
			}
			if route.Path == "/api/tools/:id/invoke" {
				path = "/api/tools/compozy__session_list/invoke"
			}
			payload := expectRemoteRouteError(
				t,
				router,
				route.Method,
				path,
				http.StatusForbidden,
				"remote_scope_denied",
			)
			if !remoteproto.Scope(payload.Details["scope"]).Valid() {
				t.Fatalf("scope for %s %s is missing or invalid: %+v", route.Method, route.Path, payload)
			}
		}
	})

	t.Run("Should reject excluded routes before the SPA can serve them", func(t *testing.T) {
		t.Parallel()
		for _, browser := range []bool{false, true} {
			router := newRemoteOperatorRouter(newHandlers(nil), browser)
			router.NoRoute(func(c *gin.Context) { c.String(http.StatusOK, "SPA fixture") })
			expectRemoteRouteError(t, router, http.MethodGet, "/api/account/x", http.StatusNotFound, "route_not_remote")
			expectRemoteRouteError(t, router, http.MethodGet, "/api/computers", http.StatusNotFound, "route_not_remote")
			expectRemoteRouteError(t, router, http.MethodPost, "/api/agent/tasks/claim-next",
				http.StatusNotFound, "route_not_remote")
			code := "remote_bridge_not_transitive"
			if browser {
				code = "remote_bridge_browser_denied"
			}
			expectRemoteRouteError(t, router, http.MethodGet, "/c/cmp_x/api/sessions", http.StatusForbidden, code)
		}
	})

	t.Run("Should panic at registration when a remote group lacks a scope", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if recovered := recover(); recovered != "invariant: remote route group requires a declared scope" {
				t.Fatalf("registration panic = %v", recovered)
			}
		}()
		NewRemoteRoutes(func(routes *RemoteRoutes) {
			routes.Group("/api/fixture", "").GET("", func(c *gin.Context) { c.Status(http.StatusOK) })
		})
	})
}

func expectRemoteRouteError(
	t *testing.T, handler http.Handler, method, path string, status int, code string,
) contract.ErrorPayload {
	t.Helper()
	reply := httptest.NewRecorder()
	handler.ServeHTTP(reply, httptest.NewRequest(method, path, nil))
	var payload contract.ErrorPayload
	if err := json.Unmarshal(reply.Body.Bytes(), &payload); err != nil || reply.Code != status || payload.Code != code {
		t.Fatalf("%s %s = HTTP %d %s, want HTTP %d %s", method, path, reply.Code, reply.Body, status, code)
	}
	return payload
}

func TestPairedGatewayProfileWrites(t *testing.T) {
	// [IT-061] The legacy gateway listener owns the canonical profile write refusal.
	t.Run("Should preserve the paired gateway's canonical profile error", func(t *testing.T) {
		t.Parallel()
		router := gin.New()
		api := operatorRouteGroups{router: router.Group("/api")}
		registerProfileRoutes(api, newHandlers(nil), false)
		reply := httptest.NewRecorder()
		router.ServeHTTP(reply, httptest.NewRequest(http.MethodPost, "/api/profiles", nil))
		var payload contract.ProfileErrorPayload
		if err := json.Unmarshal(reply.Body.Bytes(), &payload); err != nil || reply.Code != http.StatusForbidden ||
			payload.Error.Code != "profile_remote_management_forbidden" || payload.Error.Action == "" {
			t.Fatalf("paired profile write = HTTP %d %s", reply.Code, reply.Body)
		}
	})
}

var _ core.CoordinatorRoleResolver = httpapiCoordinatorRoleResolverFunc(nil)

func TestHTTPAgentKernelRoutesMatchDocumentedSpecOperations(t *testing.T) {
	t.Run("Should register every HTTP agent operation in the spec", func(t *testing.T) {
		t.Parallel()

		engine := newTestRouter(t, newTestHandlers(t, &stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))

		got := registeredHTTPAgentKernelRoutes(engine.Routes())
		want := documentedHTTPAgentKernelRoutes()
		if !slices.Equal(got, want) {
			t.Fatalf("HTTP agent routes = %v, want documented agent routes %v", got, want)
		}
	})
}

func TestServerHandlerConfigIncludesCoordinatorRole(t *testing.T) {
	t.Run("Should carry coordinator resolver into handlers", func(t *testing.T) {
		t.Parallel()

		resolver := httpapiCoordinatorRoleResolverFunc(
			func(_ context.Context, workspaceID string) (compozyconfig.ResolvedCoordinatorRole, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ResolveCoordinatorRole() workspaceID = %q, want ws-1", workspaceID)
				}
				return compozyconfig.ResolvedCoordinatorRole{AgentName: "coordinator"}, nil
			},
		)

		server := &Server{}
		WithCoordinatorRole(resolver)(server)
		handlers := newHandlers(server.handlerConfig(nil))
		if handlers.CoordinatorRole == nil {
			t.Fatal("handlers.CoordinatorRole is nil, want configured resolver")
		}

		cfg, err := handlers.CoordinatorRole.ResolveCoordinatorRole(t.Context(), "ws-1")
		if err != nil {
			t.Fatalf("ResolveCoordinatorRole() error = %v", err)
		}
		if got, want := cfg.AgentName, "coordinator"; got != want {
			t.Fatalf("CoordinatorRole.AgentName = %q, want %q", got, want)
		}
	})
}

func registeredHTTPAgentKernelRoutes(routes gin.RoutesInfo) []string {
	filtered := make([]string, 0)
	for _, route := range routes {
		if isHTTPAgentKernelRoute(route.Path) {
			filtered = append(filtered, route.Method+" "+route.Path)
		}
	}
	slices.Sort(filtered)
	return filtered
}

func documentedHTTPAgentKernelRoutes() []string {
	routes := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if !slices.Contains(operation.Transports, apispec.TransportHTTP) {
			continue
		}
		if !isHTTPAgentKernelRoute(operation.Path) {
			continue
		}
		routes = append(routes, operation.Method+" "+normalizeHTTPAgentSpecRoutePath(operation.Path))
	}
	slices.Sort(routes)
	return routes
}

func isHTTPAgentKernelRoute(routePath string) bool {
	return routePath == "/api/agent" || strings.HasPrefix(routePath, "/api/agent/")
}

func normalizeHTTPAgentSpecRoutePath(routePath string) string {
	parts := strings.Split(routePath, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			parts[index] = ":" + part[1:len(part)-1]
		}
	}
	return strings.Join(parts, "/")
}

type httpapiCoordinatorRoleResolverFunc func(context.Context, string) (compozyconfig.ResolvedCoordinatorRole, error)

func (f httpapiCoordinatorRoleResolverFunc) ResolveCoordinatorRole(
	ctx context.Context,
	workspaceID string,
) (compozyconfig.ResolvedCoordinatorRole, error) {
	return f(ctx, workspaceID)
}
