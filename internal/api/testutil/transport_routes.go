package testutil

import (
	"slices"
	"sort"
	"strings"

	apispec "github.com/compozy/compozy/internal/api/spec"
	"github.com/gin-gonic/gin"
)

// IsProfileRoute matches only the profile collection and its descendants.
func IsProfileRoute(path string) bool {
	return path == "/api/profiles" || strings.HasPrefix(path, "/api/profiles/")
}

// RoutesFromEngine returns normalized, stable routes selected by a path predicate.
func RoutesFromEngine(routes gin.RoutesInfo, matches func(string) bool) []string {
	filtered := make([]string, 0)
	for _, route := range routes {
		if matches(route.Path) {
			filtered = append(filtered, route.Method+" "+route.Path)
		}
	}
	sort.Strings(filtered)
	return filtered
}

// DocumentedRoutesForTransport returns normalized routes selected from the public spec.
func DocumentedRoutesForTransport(transport apispec.Transport, matches func(string) bool) []string {
	routes := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if slices.Contains(operation.Transports, transport) && matches(operation.Path) {
			routes = append(routes, operation.Method+" "+normalizeSpecRoutePath(operation.Path))
		}
	}
	sort.Strings(routes)
	return routes
}

// ProfileRoutesFromEngine returns normalized, stable profile routes registered by a transport.
func ProfileRoutesFromEngine(routes gin.RoutesInfo) []string {
	return RoutesFromEngine(routes, IsProfileRoute)
}

// DocumentedProfileRoutesForTransport returns the normalized profile routes in the public spec.
func DocumentedProfileRoutesForTransport(transport apispec.Transport) []string {
	return DocumentedRoutesForTransport(transport, IsProfileRoute)
}

func normalizeSpecRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			parts[index] = ":" + part[1:len(part)-1]
		}
	}
	return strings.Join(parts, "/")
}
