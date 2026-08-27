// Suite: Integrated terminal OpenAPI contract.
// Invariant: every terminal route ships on HTTP and UDS with its exact profile scope, browser identity, and WS response.
// Boundary IN: terminal paths, operation ids, transports, profile and identity parameters, and response statuses.
// Boundary OUT: handler behavior belongs to internal/api/core and user journeys belong to web/e2e.
package spec

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestTerminalOpenAPIContract(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the complete terminal route tranche on HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		expected := map[string]string{
			http.MethodGet + " " + terminalPath + "/stream":                                   "streamTerminalCatalog",
			http.MethodGet + " " + terminalPath:                                               "listTerminals",
			http.MethodPost + " " + terminalPath:                                              "createTerminal",
			http.MethodGet + " " + terminalPath + "/{id}":                                     "getTerminal",
			http.MethodDelete + " " + terminalPath + "/{id}":                                  "deleteTerminal",
			http.MethodPost + " " + terminalPath + "/{id}/attach-ticket":                      "mintTerminalAttachTicket",
			http.MethodGet + " " + terminalPath + "/{id}/stream":                              "streamTerminal",
			http.MethodPost + " " + terminalPath + "/exec":                                    "execTerminal",
			http.MethodGet + " " + terminalPath + "/{id}/read":                                "readTerminal",
			http.MethodPost + " " + terminalPath + "/{id}/signal":                             "signalTerminal",
			http.MethodPost + " " + terminalPath + "/{id}/wait":                               "waitTerminal",
			http.MethodGet + " " + terminalPath + "/input-requests":                           "listTerminalInputRequests",
			http.MethodPost + " " + terminalPath + "/{id}/input-requests/{request_id}/answer": "answerTerminalInputRequest",
			http.MethodPost + " " + terminalPath + "/{id}/input-requests/{request_id}/reject": "rejectTerminalInputRequest",
			http.MethodPost + " " + terminalPath + "/{id}/recording":                          "controlTerminalRecording",
			http.MethodGet + " " + terminalPath + "/journal":                                  "queryTerminalJournal",
			http.MethodGet + " " + terminalPath + "/recordings/{id}":                          "downloadTerminalRecording",
			http.MethodGet + " " + terminalPath + "/artifacts/{id}":                           "downloadTerminalArtifact",
		}
		seen := make(map[string]struct{}, len(expected))
		for _, operation := range Operations() {
			if !strings.HasPrefix(operation.Path, terminalPath) {
				continue
			}
			key := operation.Method + " " + operation.Path
			if operation.OperationID != expected[key] {
				t.Fatalf("%s operation id = %q, want %q", key, operation.OperationID, expected[key])
			}
			if !slices.Equal(operation.Transports, []Transport{TransportHTTP, TransportUDS}) {
				t.Fatalf("%s transports = %v, want HTTP and UDS", key, operation.Transports)
			}
			seen[key] = struct{}{}
		}
		if len(seen) != len(expected) {
			t.Fatalf("terminal operations found = %d, want %d; seen = %v", len(seen), len(expected), seen)
		}
	})

	t.Run("Should keep aggregate selectors and the ticket-bound stream distinct", func(t *testing.T) {
		t.Parallel()

		aggregates := map[string]struct{}{
			http.MethodGet + " " + terminalPath:                     {},
			http.MethodGet + " " + terminalPath + "/input-requests": {},
			http.MethodGet + " " + terminalPath + "/journal":        {},
		}
		for _, operation := range Operations() {
			if !strings.HasPrefix(operation.Path, terminalPath) {
				continue
			}
			key := operation.Method + " " + operation.Path
			hasProfile := operationHasParameter(operation, specProfileKey)
			hasAggregate := operationHasParameter(operation, "all_profiles")
			if operation.Path == terminalPath+"/{id}/stream" {
				if hasProfile || hasAggregate {
					t.Fatalf("%s exposes profile selectors despite ticket binding", key)
				}
				if !operationHasResponse(operation, http.StatusSwitchingProtocols) {
					t.Fatalf("%s omits the 101 upgrade response", key)
				}
				continue
			}
			_, aggregate := aggregates[key]
			if !hasProfile || hasAggregate != aggregate {
				t.Fatalf("%s profile/all_profiles = %t/%t, aggregate = %t", key, hasProfile, hasAggregate, aggregate)
			}
		}
	})

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
}

func operationHasParameter(operation OperationSpec, name string) bool {
	for _, parameter := range operation.Parameters {
		if parameter.Name == name {
			return true
		}
	}
	return false
}

func operationHasResponse(operation OperationSpec, status int) bool {
	for _, response := range operation.Responses {
		if response.Status == status {
			return true
		}
	}
	return false
}
