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
			"GET /api/marketplace/search":            "searchMarketplace",
			"GET /api/marketplace/{kind}":            "browseMarketplaceKind",
			"GET /api/marketplace/{kind}/{entry_id}": "getMarketplaceEntry",
			"POST /api/marketplace/refresh":          "refreshMarketplaceCatalog",
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

		search := jsonResponseSchema(t, operationFor(t, doc, "/api/marketplace/search", "GET"), 200)
		searchKinds := propertySchema(t, search, "kinds")
		if searchKinds.Items == nil || searchKinds.Items.Value == nil {
			t.Fatal("search kinds items schema = nil")
		}
		assertRequired(t, searchKinds.Items.Value, "kind", "stale", "items")
		assertNotRequired(t, searchKinds.Items.Value, "total", "next_cursor", "error_class", "error")

		browse := jsonResponseSchema(t, operationFor(t, doc, "/api/marketplace/{kind}", "GET"), 200)
		assertRequired(t, browse, "kind", "stale", "items")
		assertNotRequired(t, browse, "total", "next_cursor", "error_class", "error")

		refresh := jsonResponseSchema(t, operationFor(t, doc, "/api/marketplace/refresh", "POST"), 200)
		refreshKinds := propertySchema(t, refresh, "kinds")
		if refreshKinds.Items == nil || refreshKinds.Items.Value == nil {
			t.Fatal("refresh kinds items schema = nil")
		}
		assertRequired(t, refreshKinds.Items.Value, "kind", "outcome", "entry_count", "stale")
		assertNotRequired(t, refreshKinds.Items.Value, "error_class")
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
				name:   "Should describe marketplace search",
				path:   "/api/marketplace/search",
				method: httpMethodGet,
				parameters: []parameterExpectation{
					{name: "q", in: openapi3.ParameterInQuery},
					{name: "limit", in: openapi3.ParameterInQuery},
					{name: "scope", in: openapi3.ParameterInQuery, enum: []string{"global", "workspace"}},
					{name: "workspace_id", in: openapi3.ParameterInQuery},
				},
				statuses: []int{200, 400, 500, 503},
			},
			{
				name:   "Should describe marketplace kind browse",
				path:   "/api/marketplace/{kind}",
				method: httpMethodGet,
				parameters: []parameterExpectation{
					{
						name: "kind", in: openapi3.ParameterInPath, required: true,
						enum: []string{"mcp", "extension", "skill", "bundle"},
					},
					{name: "q", in: openapi3.ParameterInQuery},
					{name: "limit", in: openapi3.ParameterInQuery},
					{name: "cursor", in: openapi3.ParameterInQuery},
					{name: "scope", in: openapi3.ParameterInQuery, enum: []string{"global", "workspace"}},
					{name: "workspace_id", in: openapi3.ParameterInQuery},
				},
				statuses: []int{200, 400, 404, 500, 503},
			},
			{
				name:   "Should describe marketplace entry detail",
				path:   "/api/marketplace/{kind}/{entry_id}",
				method: httpMethodGet,
				parameters: []parameterExpectation{
					{
						name: "kind", in: openapi3.ParameterInPath, required: true,
						enum: []string{"mcp", "extension", "skill", "bundle"},
					},
					{name: "entry_id", in: openapi3.ParameterInPath, required: true},
					{name: "installed_name", in: openapi3.ParameterInQuery},
					{name: "scope", in: openapi3.ParameterInQuery, enum: []string{"global", "workspace"}},
					{name: "workspace_id", in: openapi3.ParameterInQuery},
				},
				statuses: []int{200, 400, 404, 500, 503},
			},
			{
				name:   "Should describe marketplace refresh",
				path:   "/api/marketplace/refresh",
				method: httpMethodPost,
				parameters: []parameterExpectation{
					{name: "kind", in: openapi3.ParameterInQuery, enum: []string{"mcp", "extension", "skill"}},
				},
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
