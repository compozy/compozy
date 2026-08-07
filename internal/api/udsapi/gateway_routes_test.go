package udsapi

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/core"
	apispec "github.com/compozy/compozy/internal/api/spec"
	"github.com/gin-gonic/gin"
)

type gatewayUDSServiceStub struct {
	core.GatewayService
}

func TestGatewayUDSRoutesMatchDocumentedOperations(t *testing.T) {
	t.Parallel()
	t.Run("Should equal the documented gateway UDS operation set [IT-083]", func(t *testing.T) {
		t.Parallel()
		assertGatewayUDSRoutesMatchDocumentedOperations(t)
	})
}

func assertGatewayUDSRoutesMatchDocumentedOperations(t *testing.T) {
	t.Helper()
	handlers := newHandlers(&handlerConfig{gateway: gatewayUDSServiceStub{}})
	router := gin.New()
	RegisterRoutes(router, handlers)
	actual := make([]string, 0)
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/gateway") {
			continue
		}
		path := strings.ReplaceAll(strings.ReplaceAll(route.Path, ":name", "{name}"), ":id", "{id}")
		actual = append(actual, route.Method+" "+path)
	}
	sort.Strings(actual)

	want := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if slices.Contains(operation.Transports, apispec.TransportUDS) &&
			strings.HasPrefix(operation.Path, "/api/gateway") {
			want = append(want, operation.Method+" "+operation.Path)
		}
	}
	sort.Strings(want)
	if !slices.Equal(actual, want) {
		t.Fatalf("UDS gateway routes = %v, want documented operations %v", actual, want)
	}
}
