package spec

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// UT-011: the removed-built-in listing is a static path beside the `{name}` extension paths, so the
// listing resolves to its own operation and the `{name}` GET stays the single-extension status read.
func TestExtensionRemovedBuiltinRouteOrdering(t *testing.T) {
	t.Parallel()
	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	t.Run("Should resolve the static removed-builtins path to the listing, not the name route", func(t *testing.T) {
		t.Parallel()

		listing := operationFor(t, doc, "/api/extensions/removed-builtins", "GET")
		if listing.OperationID != "listRemovedBuiltinExtensions" {
			t.Fatalf("removed-builtins GET operationId = %q, want listRemovedBuiltinExtensions", listing.OperationID)
		}
		status := operationFor(t, doc, "/api/extensions/{name}", "GET")
		if status.OperationID != "getExtension" {
			t.Fatalf("/api/extensions/{name} GET operationId = %q, want getExtension", status.OperationID)
		}
	})

	t.Run("Should publish the removed-built-in list shape with no path parameter", func(t *testing.T) {
		t.Parallel()

		listing := operationFor(t, doc, "/api/extensions/removed-builtins", "GET")
		if hasPathParameter(listing) {
			t.Fatalf("removed-builtins GET declares a path parameter: %#v", listing.Parameters)
		}
		schema := jsonResponseSchema(t, listing, 200)
		assertRequired(t, schema, "removed")
		items := schema.Properties["removed"].Value.Items.Value
		assertRequired(t, items, "name", "removed_at", "removed_by")
	})

	t.Run("Should document no profile selector on the global built-in operations", func(t *testing.T) {
		t.Parallel()

		for _, request := range []struct{ path, method string }{
			{"/api/extensions/removed-builtins", "GET"},
			{"/api/extensions/{name}/restore", "POST"},
		} {
			for _, ref := range operationFor(t, doc, request.path, request.method).Parameters {
				if ref != nil && ref.Value != nil && (ref.Value.Name == "profile" || ref.Value.Name == "all_profiles") {
					t.Fatalf("%s %s documents %q, but the built-in handlers read no profile", request.method,
						request.path, ref.Value.Name)
				}
			}
		}
	})

	t.Run("Should publish the restore route beside the name route without shadowing it", func(t *testing.T) {
		t.Parallel()

		restore := operationFor(t, doc, "/api/extensions/{name}/restore", "POST")
		if restore.OperationID != "restoreExtension" {
			t.Fatalf("restore POST operationId = %q, want restoreExtension", restore.OperationID)
		}
		if got := operationFor(t, doc, "/api/extensions/{name}/reload", "POST").OperationID; got == "" {
			t.Fatal("reload POST lost its operation beside restore")
		}
	})
}

func hasPathParameter(operation *openapi3.Operation) bool {
	for _, ref := range operation.Parameters {
		if ref != nil && ref.Value != nil && ref.Value.In == "path" {
			return true
		}
	}
	return false
}
