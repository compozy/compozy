package spec

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestRolesRoutesAndSchemas(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	t.Run("Should register list and detail on HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		list := operationFor(t, doc, "/api/roles", "GET")
		assertOperationTransports(t, list, TransportHTTP, TransportUDS)
		assertParameter(t, list, "workspace", openapi3.ParameterInQuery, false)
		for _, status := range []int{200, 404, 410, 500, 503} {
			assertResponseStatus(t, list, status)
		}

		detail := operationFor(t, doc, "/api/roles/{role}", "GET")
		assertOperationTransports(t, detail, TransportHTTP, TransportUDS)
		assertParameter(t, detail, "role", openapi3.ParameterInPath, true)
		assertParameter(t, detail, "workspace", openapi3.ParameterInQuery, false)
		for _, status := range []int{200, 404, 410, 500, 503} {
			assertResponseStatus(t, detail, status)
		}
	})

	t.Run("Should describe the complete role projection", func(t *testing.T) {
		t.Parallel()

		list := operationFor(t, doc, "/api/roles", "GET")
		response := jsonResponseSchema(t, list, 200)
		assertRequired(t, response, "roles")
		roles := propertySchema(t, response, "roles")
		if roles.Items == nil || roles.Items.Value == nil {
			t.Fatal("roles schema items are missing")
		}
		role := roles.Items.Value
		assertRequired(
			t,
			role,
			"role",
			"enabled",
			"resolution_mode",
			"agent",
			"provider",
			"model",
			"reasoning_effort",
			"fallback_chain",
			"provenance",
			"diagnostics",
		)
		assertEnumValues(t, propertySchema(t, role, "resolution_mode"), "builtin", "catalog", "inherit")

		detail := operationFor(t, doc, "/api/roles/{role}", "GET")
		detailResponse := jsonResponseSchema(t, detail, 200)
		assertRequired(t, detailResponse, "role")
	})
}
