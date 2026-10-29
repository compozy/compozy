package spec

import (
	"slices"
	"testing"
)

// The compatibility baseline admits only the new optional header on these two operations.
func addTerminalClientPartitionHeader(t *testing.T, baseline map[string]any) {
	t.Helper()
	paths := baseline["paths"].(map[string]any)
	for _, path := range []string{terminalPath, terminalPath + "/{id}/attach-ticket"} {
		operation := paths[path].(map[string]any)["post"].(map[string]any)
		parameters := operation["parameters"].([]any)
		last := len(parameters) - 1
		if parameters[last].(map[string]any)["name"] != "profile" {
			t.Fatalf("terminal partition addition: %s must end with the profile selector", path)
		}
		operation["parameters"] = slices.Concat(parameters[:last], []any{map[string]any{
			"in": "header", "name": "X-Compozy-Client-Workspace", "schema": map[string]any{"type": "string"},
		}}, parameters[last:])
	}
}

func TestTerminalOpenAPIPublicWireContract(t *testing.T) {
	t.Parallel()

	t.Run("Should publish the optional browser identity headers on create and attach ticket", func(t *testing.T) {
		t.Parallel()

		operationIDs := map[string]struct{}{
			"createTerminal":           {},
			"mintTerminalAttachTicket": {},
		}
		for _, header := range []string{"X-Compozy-Client-Token", "X-Compozy-Client-Workspace"} {
			seen := make(map[string]struct{}, len(operationIDs))
			for _, operation := range Operations() {
				if _, ok := operationIDs[operation.OperationID]; !ok {
					continue
				}
				for _, parameter := range operation.Parameters {
					if parameter.Name == header && parameter.In == "header" && !parameter.Required {
						seen[operation.OperationID] = struct{}{}
					}
				}
			}
			if len(seen) != len(operationIDs) {
				t.Fatalf("terminal browser identity header %s found = %v, want %v", header, seen, operationIDs)
			}
		}
	})

	t.Run("Should publish exact terminal request required property sets", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		testCases := []struct {
			name     string
			path     string
			required []string
		}{
			{name: "create", path: terminalPath, required: []string{}},
			{name: "attach ticket", path: terminalPath + "/{id}/attach-ticket", required: []string{"mode"}},
			{name: "exec", path: terminalPath + "/exec", required: []string{"command"}},
			{name: "wait", path: terminalPath + "/{id}/wait", required: []string{"until"}},
		}
		for _, testCase := range testCases {
			t.Run("Should match "+testCase.name, func(t *testing.T) {
				t.Parallel()
				path := doc.Paths.Value(testCase.path)
				if path == nil || path.Post == nil {
					t.Fatalf("POST %s operation is missing", testCase.path)
				}
				schema := jsonRequestSchema(t, path.Post)
				if !slices.Equal(schema.Required, testCase.required) {
					t.Fatalf("POST %s required = %v, want %v", testCase.path, schema.Required, testCase.required)
				}
			})
		}
	})
}
