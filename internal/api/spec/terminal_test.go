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

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
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

	t.Run("Should publish the exhaustive status matrix for every terminal operation", func(t *testing.T) {
		t.Parallel()

		common := []int{400, 401, 403, 409, 500, 503}
		expected := map[string][]int{
			"streamTerminalCatalog":      append([]int{200, 422}, common...),
			"listTerminals":              append([]int{200, 422}, common...),
			"createTerminal":             append([]int{201, 422}, common...),
			"getTerminal":                append([]int{200, 404, 410}, common...),
			"deleteTerminal":             append([]int{200, 404, 410}, common...),
			"mintTerminalAttachTicket":   append([]int{201, 404, 422}, common...),
			"streamTerminal":             append([]int{101}, common...),
			"execTerminal":               append([]int{200, 202, 422}, common...),
			"readTerminal":               append([]int{200, 404, 410, 422}, common...),
			"signalTerminal":             append([]int{200, 404, 422}, common...),
			"waitTerminal":               append([]int{200, 404, 410, 422}, common...),
			"listTerminalInputRequests":  append([]int{200, 422}, common...),
			"answerTerminalInputRequest": append([]int{200, 404}, common...),
			"rejectTerminalInputRequest": append([]int{200, 404}, common...),
			"controlTerminalRecording":   append([]int{200, 404, 422}, common...),
			"queryTerminalJournal":       append([]int{200, 422}, common...),
			"downloadTerminalRecording":  append([]int{200, 404}, common...),
			"downloadTerminalArtifact":   append([]int{200, 404}, common...),
		}
		for _, operation := range Operations() {
			want, ok := expected[operation.OperationID]
			if !ok {
				continue
			}
			got := make([]int, 0, len(operation.Responses))
			for _, response := range operation.Responses {
				got = append(got, response.Status)
			}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("%s response statuses = %v, want %v", operation.OperationID, got, want)
			}
			delete(expected, operation.OperationID)
		}
		if len(expected) != 0 {
			t.Fatalf("terminal status matrix operations not found = %v", expected)
		}
	})

	t.Run("Should constrain every public terminal sequence cursor to decimal uint64", func(t *testing.T) {
		t.Parallel()

		expected := map[string]string{
			"streamTerminal": "after_seq",
			"readTerminal":   "since_seq",
		}
		for _, operation := range Operations() {
			parameterName, ok := expected[operation.OperationID]
			if !ok {
				continue
			}
			for _, parameter := range operation.Parameters {
				if parameter.Name != parameterName {
					continue
				}
				if parameter.Pattern != terminalpkg.DecimalUint64Pattern || parameter.MaxLength == nil ||
					*parameter.MaxLength != terminalpkg.DecimalUint64MaxLength {
					t.Fatalf("%s %s schema = pattern %q maxLength %v", operation.OperationID, parameterName,
						parameter.Pattern, parameter.MaxLength)
				}
				delete(expected, operation.OperationID)
				break
			}
		}
		if len(expected) != 0 {
			t.Fatalf("terminal cursor parameters not found = %v", expected)
		}

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		read := doc.Paths.Value(terminalPath + "/{id}/read")
		if read == nil || read.Get == nil {
			t.Fatal("GET terminal read operation is missing")
		}
		seq := propertySchema(t, jsonResponseSchema(t, read.Get, http.StatusOK), "seq")
		if seq.Pattern != terminalpkg.DecimalUint64Pattern || seq.MaxLength == nil ||
			*seq.MaxLength != terminalpkg.DecimalUint64MaxLength {
			t.Fatalf("terminal read seq schema = pattern %q maxLength %v", seq.Pattern, seq.MaxLength)
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
				for _, response := range operation.Responses {
					if response.Status == http.StatusSwitchingProtocols {
						if response.Body != nil || !strings.Contains(response.Description, "binary") ||
							!strings.Contains(response.Description, "compozy.terminal.v2") ||
							!strings.Contains(response.Description, "PRESENCE=0x09") ||
							!strings.Contains(response.Description, "REDACTED_INPUT=0x0A") ||
							!strings.Contains(response.Description, "RELEASE=0x07") {
							t.Fatalf("%s 101 response = %#v, want bodyless binary opcode contract", key, response)
						}
						continue
					}
					if response.Status >= http.StatusBadRequest {
						if _, ok := response.Body.(contract.TerminalErrorResponse); !ok {
							t.Fatalf("%s error response %d body = %T", key, response.Status, response.Body)
						}
					}
				}
				continue
			}
			_, aggregate := aggregates[key]
			if !hasProfile || hasAggregate != aggregate {
				t.Fatalf("%s profile/all_profiles = %t/%t, aggregate = %t", key, hasProfile, hasAggregate, aggregate)
			}
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
