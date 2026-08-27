// Suite: Integrated terminal OpenAPI contract.
// Invariant: every terminal route ships on HTTP and UDS with its exact profile scope, browser identity, and WS response.
// Boundary IN: terminal paths, operation ids, transports, profile and identity parameters, and response statuses.
// Boundary OUT: handler behavior belongs to internal/api/core and user journeys belong to web/e2e.
package spec

import (
	"net/http"
	"reflect"
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
							!strings.Contains(response.Description, "PRESENCE=0x09") ||
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

	t.Run("Should publish the exact frozen terminal error code enum", func(t *testing.T) {
		t.Parallel()

		want := frozenTerminalErrorCodes()
		if len(want) != 31 {
			t.Fatalf("frozen terminal error code count = %d, want 31", len(want))
		}
		if got := contract.TerminalErrorCodeValues(); !slices.Equal(got, want) {
			t.Fatalf("TerminalErrorCodeValues() = %v, want %v", got, want)
		}
		registered := schemaEnumValues[reflect.TypeFor[contract.TerminalErrorCode]()]
		if !slices.Equal(registered, want) {
			t.Fatalf("registered TerminalErrorCode enum = %v, want %v", registered, want)
		}

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		exec := doc.Paths.Value(terminalPath + "/exec")
		if exec == nil || exec.Post == nil {
			t.Fatal("POST terminal exec operation is missing")
		}
		detail := propertySchema(t, jsonResponseSchema(t, exec.Post, http.StatusUnprocessableEntity), "error")
		code := propertySchema(t, detail, "code")
		assertEnumValues(t, code, want...)
		if len(code.Enum) != 31 {
			t.Fatalf("TerminalErrorDetail.code enum count = %d, want 31", len(code.Enum))
		}

		for _, values := range [][]string{toolErrorCodeValues(), toolReasonCodeValues()} {
			seen := make(map[string]int, len(values))
			for _, value := range values {
				seen[value]++
			}
			for _, terminalCode := range want {
				if seen[terminalCode] != 1 {
					t.Errorf(
						"terminal code %q occurs %d times in tool enum, want once",
						terminalCode,
						seen[terminalCode],
					)
				}
			}
		}
	})

	t.Run("Should publish exact closed terminal projection enums", func(t *testing.T) {
		t.Parallel()

		registeredEnums := []struct {
			name   string
			typeOf reflect.Type
			want   []string
		}{
			{name: "mode", typeOf: reflect.TypeFor[terminalpkg.Mode](), want: []string{"pty", "pipe"}},
			{
				name:   "lease state",
				typeOf: reflect.TypeFor[terminalpkg.LeaseState](),
				want:   []string{"human_owned", "agent_owned", "available"},
			},
			{
				name:   "actor kind",
				typeOf: reflect.TypeFor[terminalpkg.ActorKind](),
				want:   []string{"human", "agent", "system"},
			},
			{
				name:   "signal",
				typeOf: reflect.TypeFor[terminalpkg.Signal](),
				want:   []string{"INT", "TERM", "KILL", "HUP"},
			},
			{
				name:   "run state",
				typeOf: reflect.TypeFor[contract.TerminalState](),
				want:   contract.TerminalStateValues(),
			},
			{
				name:   "exit cause",
				typeOf: reflect.TypeFor[contract.TerminalExitCause](),
				want:   contract.TerminalExitCauseValues(),
			},
			{
				name:   "command detection",
				typeOf: reflect.TypeFor[contract.TerminalCommandDetection](),
				want:   contract.TerminalCommandDetectionValues(),
			},
			{
				name:   "command approval",
				typeOf: reflect.TypeFor[contract.TerminalCommandApproval](),
				want:   contract.TerminalCommandApprovalValues(),
			},
			{
				name:   "attach mode",
				typeOf: reflect.TypeFor[contract.TerminalAttachMode](),
				want:   contract.TerminalAttachModeValues(),
			},
			{
				name:   "recording action",
				typeOf: reflect.TypeFor[contract.TerminalRecordingAction](),
				want:   contract.TerminalRecordingActionValues(),
			},
			{
				name:   "recording state",
				typeOf: reflect.TypeFor[contract.TerminalRecordingState](),
				want:   contract.TerminalRecordingStateValues(),
			},
			{
				name:   "input reject outcome",
				typeOf: reflect.TypeFor[contract.TerminalInputRejectOutcome](),
				want:   contract.TerminalInputRejectOutcomeValues(),
			},
		}
		for _, enum := range registeredEnums {
			t.Run("Should register "+enum.name, func(t *testing.T) {
				t.Parallel()
				got := schemaEnumValues[enum.typeOf]
				if !slices.Equal(got, enum.want) {
					t.Fatalf("registered %s enum = %v, want %v", enum.name, got, enum.want)
				}
				if len(got) != len(enum.want) {
					t.Fatalf("registered %s enum count = %d, want %d", enum.name, len(got), len(enum.want))
				}
			})
		}

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		list := doc.Paths.Value(terminalPath)
		if list == nil || list.Get == nil {
			t.Fatal("GET terminal list operation is missing")
		}
		terminal := propertySchema(t, jsonResponseSchema(t, list.Get, http.StatusOK), "terminals").Items.Value
		assertEnumValues(t, propertySchema(t, terminal, "mode"), "pty", "pipe")
		assertEnumValues(t, propertySchema(t, terminal, "state"), "running", "exited")
		assertEnumValues(t, propertySchema(t, terminal, "lease"), "human_owned", "agent_owned", "available")
		assertEnumValues(
			t,
			propertySchema(t, propertySchema(t, terminal, "controller"), "kind"),
			"human",
			"agent",
			"system",
		)
		assertEnumValues(
			t,
			propertySchema(t, propertySchema(t, terminal, "exit"), "cause"),
			"exited",
			"signaled",
			"unknown",
		)
		assertEnumValues(
			t,
			propertySchema(t, propertySchema(t, terminal, "exit"), "signal"),
			"INT",
			"TERM",
			"KILL",
			"HUP",
		)

		journal := doc.Paths.Value(terminalPath + "/journal")
		if journal == nil || journal.Get == nil {
			t.Fatal("GET terminal journal operation is missing")
		}
		entry := propertySchema(t, jsonResponseSchema(t, journal.Get, http.StatusOK), "entries").Items.Value
		assertEnumValues(t, propertySchema(t, propertySchema(t, entry, "actor"), "kind"), "human", "agent", "system")
		assertEnumValues(t, propertySchema(t, entry, "exit_cause"), "exited", "signaled", "unknown")
		assertEnumValues(t, propertySchema(t, entry, "signal"), "INT", "TERM", "KILL", "HUP")
		assertEnumValues(t, propertySchema(t, entry, "detected_by"), "exact", "marker", "idle")
		assertEnumValues(
			t,
			propertySchema(t, entry, "approval"),
			"approved_once",
			"approved_always",
			"allowlisted",
			"human",
			"none",
		)

		attach := doc.Paths.Value(terminalPath + "/{id}/attach-ticket")
		if attach == nil || attach.Post == nil {
			t.Fatal("POST terminal attach ticket operation is missing")
		}
		assertEnumValues(t, propertySchema(t, jsonRequestSchema(t, attach.Post), "mode"), "read", "write")

		recording := doc.Paths.Value(terminalPath + "/{id}/recording")
		if recording == nil || recording.Post == nil {
			t.Fatal("POST terminal recording operation is missing")
		}
		assertEnumValues(t, propertySchema(t, jsonRequestSchema(t, recording.Post), "action"), "start", "stop")
		assertEnumValues(
			t,
			propertySchema(
				t,
				propertySchema(t, jsonResponseSchema(t, recording.Post, http.StatusOK), "recording"),
				"state",
			),
			"recording",
			"saved",
		)

		signal := doc.Paths.Value(terminalPath + "/{id}/signal")
		if signal == nil || signal.Post == nil {
			t.Fatal("POST terminal signal operation is missing")
		}
		assertEnumValues(
			t,
			propertySchema(t, jsonRequestSchema(t, signal.Post), "signal"),
			"INT",
			"TERM",
			"KILL",
			"HUP",
		)

		reject := doc.Paths.Value(terminalPath + "/{id}/input-requests/{request_id}/reject")
		if reject == nil || reject.Post == nil {
			t.Fatal("POST terminal input rejection operation is missing")
		}
		assertEnumValues(
			t,
			propertySchema(t, jsonResponseSchema(t, reject.Post, http.StatusOK), "outcome"),
			"rejected",
		)
	})
}

func frozenTerminalErrorCodes() []string {
	return []string{
		"terminal_not_found",
		"profile_selection_conflict",
		"profile_session_conflict",
		"terminal_requires_workspace",
		"profile_archived",
		"profile_unavailable",
		"terminal_limit_reached",
		"subscriber_limit_reached",
		"terminal_exited",
		"terminal_expired",
		"terminal_interactive_unavailable",
		"terminal_not_interactive",
		"invalid_cwd",
		"timeout_out_of_range",
		"write_owner_held",
		"lease_revoked",
		"generation_fenced",
		"typing_grant_rejected",
		"approval_rejected",
		"ticket_invalid",
		"ticket_expired",
		"input_request_not_found",
		"input_request_already_answered",
		"input_request_superseded",
		"input_request_limit_reached",
		"input_answer_requires_write",
		"recording_already_started",
		"recording_not_active",
		"recording_unavailable",
		"slow_consumer",
		"journal_unavailable",
	}
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
