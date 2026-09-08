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

func TestTerminalOpenAPISchemaContract(t *testing.T) {
	t.Parallel()

	t.Run("Should publish exact frozen domain codes beside a tolerant transport envelope", func(t *testing.T) {
		t.Parallel()

		want := frozenTerminalErrorCodes()
		if len(want) != 28 {
			t.Fatalf("frozen terminal error code count = %d, want 28", len(want))
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
		assertSchemaIncludesType(t, code, "string")
		if len(code.Enum) != 0 {
			t.Fatalf("TerminalErrorDetail.code enum = %v, want tolerant string", code.Enum)
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
			{
				name: "mode", typeOf: reflect.TypeFor[contract.TerminalMode](),
				want: contract.TerminalModeValues(),
			},
			{
				name:   "actor kind",
				typeOf: reflect.TypeFor[contract.TerminalActorKind](),
				want:   contract.TerminalActorKindValues(),
			},
			{
				name:   "signal",
				typeOf: reflect.TypeFor[contract.TerminalSignal](),
				want:   contract.TerminalSignalValues(),
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
			{
				name:   "input resolution outcome",
				typeOf: reflect.TypeFor[contract.TerminalInputResolutionOutcome](),
				want:   contract.TerminalInputResolutionOutcomeValues(),
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
		for _, removed := range []string{"lease", "controller"} {
			if terminal.Properties[removed] != nil {
				t.Fatalf("terminal projection still publishes the removed %q property", removed)
			}
		}
		for registered := range schemaEnumValues {
			if registered.Name() == "TerminalLeaseState" {
				t.Fatalf("removed TerminalLeaseState enum is still registered as %v", registered)
			}
		}

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

		requests := doc.Paths.Value(terminalPath + "/input-requests")
		if requests == nil || requests.Get == nil {
			t.Fatal("GET terminal input requests operation is missing")
		}
		resolved := propertySchema(
			t,
			jsonResponseSchema(t, requests.Get, http.StatusOK),
			"resolved",
		).Items.Value
		assertEnumValues(t, propertySchema(t, resolved, "outcome"), "answered", "rejected", "superseded", "expired")
	})

	t.Run("Should reject an unknown resolved input outcome at the public DTO boundary", func(t *testing.T) {
		t.Parallel()

		_, err := contract.TerminalInputRequestsResponseFromDomain(nil, []terminalpkg.ResolvedInputRequest{{
			Outcome: terminalpkg.InputResolutionOutcome("future"),
		}})
		if err == nil || !strings.Contains(err.Error(), "unknown input resolution outcome") {
			t.Fatalf(
				"TerminalInputRequestsResponseFromDomain() error = %v, want unknown outcome rejection",
				err,
			)
		}
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
		"generation_fenced",
		"approval_rejected",
		"ticket_invalid",
		"ticket_expired",
		"input_request_not_found",
		"input_request_already_answered",
		"input_request_superseded",
		"input_request_limit_reached",
		"input_request_requires_hidden_input",
		"recording_already_started",
		"recording_not_active",
		"recording_unavailable",
		"slow_consumer",
		"journal_unavailable",
	}
}
