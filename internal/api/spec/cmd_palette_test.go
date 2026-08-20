package spec

import (
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// Suite: command-palette OpenAPI and transport registry.
// Invariant: every P1 command-palette and async-approval route is registered on HTTP and UDS.
// Owning layer: internal/api/spec. Canonical suite: this file.
func TestCmdPaletteOperationsSupportHTTPAndUDS(t *testing.T) {
	t.Parallel()

	t.Run("Should register command-palette routes on HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		want := map[string]string{
			"GET /api/cmd-palette/commands":                        "listCmdPaletteCommands",
			"GET /api/cmd-palette/clients":                         "listCmdPaletteClients",
			"POST /api/cmd-palette/commands/{id}/invoke":           "invokeCmdPaletteCommand",
			"POST /api/cmd-palette/views/{id}/open":                "openCmdPaletteViewSession",
			"GET /api/cmd-palette/view-sessions/{session}/stream":  "streamCmdPaletteViewSession",
			"POST /api/cmd-palette/view-sessions/{session}/events": "admitCmdPaletteViewSessionEvent",
			"DELETE /api/cmd-palette/view-sessions/{session}":      "closeCmdPaletteViewSession",
			"GET /api/cmd-palette/stream":                          "streamCmdPalette",
			"GET /api/tools/approvals/{id}":                        "getPendingToolApproval",
			"POST /api/tools/approvals/{id}/cancel":                "cancelPendingToolApproval",
		}
		seen := make(map[string]OperationSpec, len(want))
		for _, operation := range Operations() {
			key := operation.Method + " " + operation.Path
			if _, exists := want[key]; exists {
				seen[key] = operation
			}
		}
		if len(seen) != len(want) {
			t.Fatalf("command-palette operations found = %d, want %d; seen = %v", len(seen), len(want), seen)
		}
		for key, operation := range seen {
			if operation.OperationID != want[key] {
				t.Fatalf("%s operation ID = %q, want %q", key, operation.OperationID, want[key])
			}
			if !slices.Equal(operation.Transports, []Transport{TransportHTTP, TransportUDS}) {
				t.Fatalf("%s transports = %v, want [http uds] [IT-022]", key, operation.Transports)
			}
		}
	})

	t.Run("Should publish the client attachment header on invoke and view-session operations", func(t *testing.T) {
		t.Parallel()

		want := map[string]string{
			"invokeCmdPaletteCommand":         "POST /api/cmd-palette/commands/{id}/invoke",
			"openCmdPaletteViewSession":       "POST /api/cmd-palette/views/{id}/open",
			"admitCmdPaletteViewSessionEvent": "POST /api/cmd-palette/view-sessions/{session}/events",
			"closeCmdPaletteViewSession":      "DELETE /api/cmd-palette/view-sessions/{session}",
		}
		seen := make(map[string]OperationSpec, len(want))
		for _, operation := range Operations() {
			if _, exists := want[operation.OperationID]; exists {
				seen[operation.OperationID] = operation
			}
		}
		if len(seen) != len(want) {
			t.Fatalf("attachment operations found = %d, want %d; seen = %v", len(seen), len(want), seen)
		}
		for operationID, operation := range seen {
			if !cmdPalettePublishesClientAttachmentHeader(operation) {
				t.Fatalf(
					"%s (%s) missing required X-Compozy-Client-Token header",
					operationID, want[operationID],
				)
			}
		}
	})
}

func cmdPalettePublishesClientAttachmentHeader(operation OperationSpec) bool {
	for _, parameter := range operation.Parameters {
		if parameter.Name == "X-Compozy-Client-Token" &&
			parameter.In == openapi3.ParameterInHeader &&
			parameter.Required {
			return true
		}
	}
	return false
}
