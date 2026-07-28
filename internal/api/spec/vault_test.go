package spec

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestVaultRoutesAndSchemas(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	t.Run("Should register vault routes with HTTP and UDS parity", func(t *testing.T) {
		t.Parallel()

		operations := []struct {
			path   string
			method string
		}{
			{path: "/api/vault/secrets", method: "GET"},
			{path: "/api/vault/secrets/metadata", method: "GET"},
			{path: "/api/vault/secrets", method: "PUT"},
			{path: "/api/vault/secrets", method: "DELETE"},
		}

		for _, operation := range operations {
			t.Run(operation.method+" "+operation.path, func(t *testing.T) {
				t.Parallel()

				op := operationFor(t, doc, operation.path, operation.method)
				assertTagsContain(t, op, "vault")
				assertOperationTransports(t, op, TransportHTTP, TransportUDS)
			})
		}
	})

	t.Run("Should describe redacted metadata responses and write-only requests", func(t *testing.T) {
		t.Parallel()

		listVaultSecrets := operationFor(t, doc, "/api/vault/secrets", "GET")
		assertParameter(t, listVaultSecrets, "prefix", openapi3.ParameterInQuery, false)
		assertParameter(t, listVaultSecrets, "namespace", openapi3.ParameterInQuery, false)
		listSchema := jsonResponseSchema(t, listVaultSecrets, 200)
		assertRequired(t, listSchema, "secrets")
		secretListSchema := propertySchema(t, listSchema, "secrets")
		if secretListSchema.Items == nil || secretListSchema.Items.Value == nil {
			t.Fatal("expected vault secrets list to define an items schema")
		}
		assertVaultMetadataSchema(t, secretListSchema.Items.Value)

		getVaultSecret := operationFor(t, doc, "/api/vault/secrets/metadata", "GET")
		assertParameter(t, getVaultSecret, "ref", openapi3.ParameterInQuery, true)
		getSchema := jsonResponseSchema(t, getVaultSecret, 200)
		assertRequired(t, getSchema, "secret")
		assertVaultMetadataSchema(t, propertySchema(t, getSchema, "secret"))

		putVaultSecret := operationFor(t, doc, "/api/vault/secrets", "PUT")
		putSchema := jsonRequestSchema(t, putVaultSecret)
		assertRequired(t, putSchema, "ref", "secret_value")
		assertNotRequired(t, putSchema, "kind")
		putResponse := jsonResponseSchema(t, putVaultSecret, 200)
		assertVaultMetadataSchema(t, propertySchema(t, putResponse, "secret"))

		deleteVaultSecret := operationFor(t, doc, "/api/vault/secrets", "DELETE")
		assertParameter(t, deleteVaultSecret, "ref", openapi3.ParameterInQuery, true)
		assertResponseStatus(t, deleteVaultSecret, 204)
	})
}

func assertVaultMetadataSchema(t *testing.T, schema *openapi3.Schema) {
	t.Helper()

	assertRequired(t, schema, "ref", "namespace", "present", "created_at", "updated_at")
	assertNotRequired(t, schema, "kind")
	if _, ok := schema.Properties["secret_value"]; ok {
		t.Fatal("vault metadata schema must not expose secret_value")
	}
}
