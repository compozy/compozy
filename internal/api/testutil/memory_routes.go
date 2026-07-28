package testutil

import (
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// MemoryV2RouteKeysFromGin returns the normalized Memory v2 route keys registered on a Gin engine.
func MemoryV2RouteKeysFromGin(routes gin.RoutesInfo) []string {
	keys := make([]string, 0)
	for _, route := range routes {
		if strings.HasPrefix(route.Path, "/api/memory") ||
			(strings.HasPrefix(route.Path, "/api/workspaces/") && strings.Contains(route.Path, "/memory/")) {
			keys = append(keys, route.Method+" "+route.Path)
		}
	}
	sort.Strings(keys)
	return keys
}

// ExpectedMemoryV2RouteKeys is the shared parity contract for every transport.
func ExpectedMemoryV2RouteKeys() []string {
	keys := []string{
		"DELETE /api/memory/:filename",
		"GET /api/memory",
		"GET /api/memory/:filename",
		"GET /api/memory/config",
		"GET /api/memory/daily",
		"GET /api/memory/decisions",
		"GET /api/memory/decisions/:decision_id",
		"GET /api/memory/dreams",
		"GET /api/memory/dreams/:dream_id",
		"GET /api/memory/dreams/status",
		"GET /api/memory/extractor/failures",
		"GET /api/memory/extractor/status",
		"GET /api/memory/health",
		"GET /api/memory/history",
		"GET /api/memory/providers",
		"GET /api/memory/providers/:provider_name",
		"GET /api/memory/recall-traces/:session_id/:turn_seq",
		"GET /api/memory/scope-show",
		"GET /api/workspaces/:workspace_id/memory/sessions/:session_id/ledger",
		"PATCH /api/memory/:filename",
		"POST /api/memory",
		"POST /api/memory/ad-hoc",
		"POST /api/memory/decisions/:decision_id/revert",
		"POST /api/memory/dreams/:dream_id/retry",
		"POST /api/memory/dreams/trigger",
		"POST /api/memory/extractor/drain",
		"POST /api/memory/extractor/retry",
		"POST /api/memory/promote",
		"POST /api/memory/providers/:provider_name/disable",
		"POST /api/memory/providers/:provider_name/enable",
		"POST /api/memory/providers/select",
		"POST /api/memory/reindex",
		"POST /api/memory/reload",
		"POST /api/memory/reset",
		"POST /api/memory/search",
		"POST /api/memory/sessions/prune",
		"POST /api/memory/sessions/repair",
		"POST /api/workspaces/:workspace_id/memory/sessions/:session_id/replay",
	}
	sort.Strings(keys)
	return keys
}

// AssertMemoryV2RouteParity fails when a transport drifts from the Memory v2 route contract.
func AssertMemoryV2RouteParity(t testing.TB, got []string) {
	t.Helper()

	want := ExpectedMemoryV2RouteKeys()
	if len(got) != len(want) {
		t.Fatalf("len(memory routes) = %d, want %d\nroutes=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("memory route[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
