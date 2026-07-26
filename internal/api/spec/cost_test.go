package spec

import (
	"net/http"
	"testing"

	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/getkin/kin-openapi/openapi3"
)

// Suite: truthful cost wire contract
// Invariant: session and task cost projections expose the same closed status/source enums.
// Boundary IN: reflected API contract structs.
// Boundary OUT: OpenAPI, generated SDK types, HTTP, and UDS consumers.
func TestCostContractsExposeClosedProvenanceEnums(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	usageResponse := jsonResponseSchema(
		t,
		operationFor(t, doc, "/api/workspaces/{workspace_id}/sessions/{session_id}/usage", http.MethodGet),
		http.StatusOK,
	)
	assertCostEnums(t, propertySchema(t, usageResponse, "usage"))

	runResponse := jsonResponseSchema(
		t,
		operationFor(t, doc, "/api/task-runs/{id}", http.MethodGet),
		http.StatusOK,
	)
	runDetail := propertySchema(t, runResponse, "run")
	assertCostEnums(t, propertySchema(t, runDetail, "summary"))
}

func assertCostEnums(t *testing.T, schema *openapi3.Schema) {
	t.Helper()

	assertEnumValues(t, propertySchema(t, schema, "cost_status"), modelcatalog.CostStatusValues()...)
	assertEnumValues(t, propertySchema(t, schema, "cost_source"), modelcatalog.CostSourceValues()...)
}
