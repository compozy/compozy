package spec

import (
	"slices"
	"testing"
)

// Suite: command-palette OpenAPI and transport registry.
// Invariant: every P1 command-palette and async-approval route is registered on HTTP and UDS.
// Owning layer: internal/api/spec. Canonical suite: this file.
func TestCmdPaletteOperationsSupportHTTPAndUDS(t *testing.T) {
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
}
