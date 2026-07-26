package httpapi

import (
	"context"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/core"
	apispec "github.com/compozy/agh/internal/api/spec"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/gin-gonic/gin"
)

var _ core.CoordinatorRoleResolver = httpapiCoordinatorRoleResolverFunc(nil)

func TestHTTPAgentKernelRoutesMatchDocumentedSpecOperations(t *testing.T) {
	t.Run("Should register every HTTP agent operation in the spec", func(t *testing.T) {
		t.Parallel()

		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))

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
			func(_ context.Context, workspaceID string) (aghconfig.ResolvedCoordinatorRole, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ResolveCoordinatorRole() workspaceID = %q, want ws-1", workspaceID)
				}
				return aghconfig.ResolvedCoordinatorRole{AgentName: "coordinator"}, nil
			},
		)

		server := &Server{}
		WithCoordinatorRole(resolver)(server)
		handlers := newHandlers(server.handlerConfig(nil))
		if handlers.CoordinatorRole == nil {
			t.Fatal("handlers.CoordinatorRole is nil, want configured resolver")
		}

		cfg, err := handlers.CoordinatorRole.ResolveCoordinatorRole(context.Background(), "ws-1")
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
	sort.Strings(filtered)
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
	sort.Strings(routes)
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

type httpapiCoordinatorRoleResolverFunc func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error)

func (f httpapiCoordinatorRoleResolverFunc) ResolveCoordinatorRole(
	ctx context.Context,
	workspaceID string,
) (aghconfig.ResolvedCoordinatorRole, error) {
	return f(ctx, workspaceID)
}
