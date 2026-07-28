package spec

import (
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestResourceOperationsSupportHTTPAndUDS(t *testing.T) {
	t.Parallel()

	t.Run("Should expose resource operations on HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		want := map[string]string{
			"GET /api/resources":                "listResources",
			"GET /api/resources/{kind}":         "listResourcesByKind",
			"GET /api/resources/{kind}/{id}":    "getResource",
			"PUT /api/resources/{kind}/{id}":    "putResource",
			"DELETE /api/resources/{kind}/{id}": "deleteResource",
		}

		seen := make(map[string]OperationSpec, len(want))
		for _, op := range Operations() {
			key := op.Method + " " + op.Path
			if _, ok := want[key]; ok {
				seen[key] = op
			}
		}

		if len(seen) != len(want) {
			t.Fatalf("resource operations found = %d, want %d", len(seen), len(want))
		}

		for key, op := range seen {
			if op.OperationID != want[key] {
				t.Fatalf("%s operation_id = %q, want %q", key, op.OperationID, want[key])
			}
			if !slices.Equal(op.Transports, []Transport{TransportHTTP, TransportUDS}) {
				t.Fatalf("%s transports = %#v, want [http uds]", key, op.Transports)
			}
		}
	})
}

func TestDocumentDescribesResourceCRUDSchemas(t *testing.T) {
	t.Parallel()

	t.Run("Should describe resource request and response envelopes", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}

		listResources := operationFor(t, doc, "/api/resources", "GET")
		assertParameter(t, listResources, "kind", "query", false)
		assertParameter(t, listResources, "scope_kind", "query", false)
		assertParameter(t, listResources, "limit", "query", false)
		listResourcesSchema := jsonResponseSchema(t, listResources, 200)
		assertRequired(t, listResourcesSchema, "records")
		recordsSchema := propertySchema(t, listResourcesSchema, "records")
		if recordsSchema.Items == nil || recordsSchema.Items.Value == nil {
			t.Fatal("resources list schema missing record items")
		}
		assertResourceRecordPayloadSchema(t, recordsSchema.Items.Value)

		getResource := operationFor(t, doc, "/api/resources/{kind}/{id}", "GET")
		getResourceSchema := jsonResponseSchema(t, getResource, 200)
		assertRequired(t, getResourceSchema, "record")
		assertResourceRecordPayloadSchema(t, propertySchema(t, getResourceSchema, "record"))

		putResource := operationFor(t, doc, "/api/resources/{kind}/{id}", "PUT")
		putSchema := jsonRequestSchema(t, putResource)
		assertRequired(t, putSchema, "scope", "spec")
		assertNotRequired(t, putSchema, "expected_version")
		scopeSchema := propertySchema(t, putSchema, "scope")
		assertEnumValues(t, propertySchema(t, scopeSchema, "kind"), "global", "workspace")

		deleteResource := operationFor(t, doc, "/api/resources/{kind}/{id}", "DELETE")
		deleteSchema := jsonRequestSchema(t, deleteResource)
		assertRequired(t, deleteSchema, "expected_version")
		assertNotRequired(t, deleteSchema, "scope", "spec")
	})
}

func assertResourceRecordPayloadSchema(t *testing.T, schema *openapi3.Schema) {
	t.Helper()

	assertRequired(t,
		schema,
		"kind",
		"id",
		"version",
		"scope",
		"owner",
		"source",
		"spec",
		"created_at",
		"updated_at",
	)
}
