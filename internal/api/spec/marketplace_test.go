package spec

import (
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestMarketplaceOperations(t *testing.T) {
	t.Parallel()

	operations := make(map[string]OperationSpec)
	for _, operation := range Operations() {
		operations[operation.Method+" "+operation.Path] = operation
	}

	t.Run("Should expose the marketplace namespace operations", func(t *testing.T) {
		t.Parallel()

		expected := map[string]string{
			"GET /api/marketplace":                    "listMarketplace",
			"GET /api/marketplace/entries/{entry_id}": "getMarketplaceCatalogEntry",
			"POST /api/marketplace/refresh":           "refreshMarketplaceCatalog",
		}
		for key, operationID := range expected {
			operation, ok := operations[key]
			if !ok {
				t.Fatalf("expected marketplace operation %s", key)
			}
			if operation.OperationID != operationID {
				t.Fatalf("%s operation ID = %q, want %q", key, operation.OperationID, operationID)
			}
			if len(operation.Transports) != 2 ||
				!slices.Contains(operation.Transports, TransportHTTP) ||
				!slices.Contains(operation.Transports, TransportUDS) {
				t.Fatalf(
					"%s transports = %v, want HTTP and UDS",
					key,
					operation.Transports,
				)
			}
		}
	})

	t.Run("Should omit hard deleted marketplace operations", func(t *testing.T) {
		t.Parallel()

		for _, key := range []string{
			"GET /api/marketplace/search",
			"GET /api/marketplace/{kind}",
			"GET /api/marketplace/{kind}/{entry_id}",
			"GET /api/skills/marketplace/search",
			"GET /api/skills/marketplace/info",
			"GET /api/extensions/marketplace",
		} {
			_, ok := operations[key]
			if ok {
				t.Fatalf("legacy marketplace operation should be absent: %s", key)
			}
		}
	})

	t.Run("Should expose truthful stale state on browse and refresh responses", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}

		browse := jsonResponseSchema(t, operationFor(t, doc, "/api/marketplace", "GET"), 200)
		assertRequired(t, browse, "total", "revision", "sources", "stale", "items")
		assertNotRequired(t, browse, "next_cursor", "error_class", "error")

		refresh := jsonResponseSchema(t, operationFor(t, doc, "/api/marketplace/refresh", "POST"), 200)
		refreshSources := propertySchema(t, refresh, "sources")
		if refreshSources.Items == nil || refreshSources.Items.Value == nil {
			t.Fatal("refresh sources items schema = nil")
		}
		assertRequired(t, refreshSources.Items.Value, "source", "outcome", "entry_count", "stale")
		assertNotRequired(t, refreshSources.Items.Value, "error_class")
	})

	t.Run("Should own marketplace parameters and response statuses", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}

		type parameterExpectation struct {
			name     string
			in       string
			required bool
			enum     []string
		}
		tests := []struct {
			name       string
			path       string
			method     string
			parameters []parameterExpectation
			statuses   []int
		}{

			{
				name:   "Should describe marketplace catalog browse",
				path:   "/api/marketplace",
				method: httpMethodGet,
				parameters: []parameterExpectation{
					{name: "q", in: openapi3.ParameterInQuery},
					{name: "limit", in: openapi3.ParameterInQuery},
					{name: "cursor", in: openapi3.ParameterInQuery},
					{name: "scope", in: openapi3.ParameterInQuery, enum: []string{"global", "profile", "workspace"}},
					{name: "profile", in: openapi3.ParameterInQuery},
					{name: "workspace_id", in: openapi3.ParameterInQuery},
				},
				statuses: []int{200, 400, 409, 500, 503},
			},
			{
				name:   "Should describe marketplace entry detail",
				path:   "/api/marketplace/entries/{entry_id}",
				method: httpMethodGet,
				parameters: []parameterExpectation{
					{name: "entry_id", in: openapi3.ParameterInPath, required: true},
					{name: "source", in: openapi3.ParameterInQuery},
					{name: "installed_name", in: openapi3.ParameterInQuery},
					{name: "scope", in: openapi3.ParameterInQuery, enum: []string{"global", "profile", "workspace"}},
					{name: "profile", in: openapi3.ParameterInQuery},
					{name: "workspace_id", in: openapi3.ParameterInQuery},
				},
				statuses: []int{200, 400, 404, 409, 500, 503},
			},
			{
				name:     "Should describe marketplace refresh",
				path:     "/api/marketplace/refresh",
				method:   httpMethodPost,
				statuses: []int{200, 400, 403, 500, 503},
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				operation := operationFor(t, doc, tc.path, tc.method)
				if len(operation.Parameters) != len(tc.parameters) {
					t.Fatalf("parameters = %d, want %d", len(operation.Parameters), len(tc.parameters))
				}
				for _, parameter := range tc.parameters {
					assertParameter(t, operation, parameter.name, parameter.in, parameter.required)
					if len(parameter.enum) > 0 {
						assertParameterEnumValues(t, operation, parameter.name, parameter.enum...)
					}
				}
				for _, status := range tc.statuses {
					assertResponseStatus(t, operation, status)
				}
			})
		}
	})
}
