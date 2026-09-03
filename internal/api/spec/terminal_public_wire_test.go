package spec

import (
	"slices"
	"testing"
)

func TestTerminalOpenAPIPublicWireContract(t *testing.T) {
	t.Parallel()

	t.Run("Should publish the optional browser identity token on create and attach ticket", func(t *testing.T) {
		t.Parallel()

		operationIDs := map[string]struct{}{
			"createTerminal":           {},
			"mintTerminalAttachTicket": {},
		}
		identityHeader := terminalClientIdentityHeaderParam()
		seen := make(map[string]struct{}, len(operationIDs))
		for _, operation := range Operations() {
			if _, ok := operationIDs[operation.OperationID]; !ok {
				continue
			}
			for _, parameter := range operation.Parameters {
				if parameter.Name == identityHeader.Name && parameter.In == "header" && !parameter.Required {
					seen[operation.OperationID] = struct{}{}
				}
			}
		}
		if len(seen) != len(operationIDs) {
			t.Fatalf("terminal browser identity headers found = %v, want %v", seen, operationIDs)
		}
	})

	t.Run("Should preserve native terminal codes in public tool errors", func(t *testing.T) {
		t.Parallel()
		for _, code := range frozenTerminalErrorCodes() {
			if !slices.Contains(toolErrorCodeValues(), code) {
				t.Errorf("ToolError.code enum omits %q", code)
			}
			if !slices.Contains(toolReasonCodeValues(), code) {
				t.Errorf("ToolError.reason_codes enum omits %q", code)
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
