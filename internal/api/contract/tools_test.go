package contract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/tools"
)

func TestToolContractsMarshalCanonicalIDsAndStructuredErrors(t *testing.T) {
	t.Parallel()

	t.Run("ShouldMarshalCanonicalIDsAndStructuredErrorsWithoutSecrets", func(t *testing.T) {
		t.Parallel()

		payload := ToolResponse{Tool: ToolPayload{
			Descriptor: ToolDescriptorPayload{
				ToolID:       tools.ToolIDSkillView,
				FriendlyVerb: "Reading",
				Preview:      "arg:name",
				Description:  "Read one skill",
				InputSchema:  json.RawMessage(`{"type":"object"}`),
				Backend: ToolBackendRefPayload{
					Kind:       tools.BackendNativeGo,
					NativeName: "skill_view",
				},
				Source: ToolSourceRefPayload{
					Kind:  tools.SourceBuiltin,
					Owner: "agh",
				},
				Visibility: tools.VisibilityModel,
				Risk:       tools.RiskRead,
				ReadOnly:   true,
				Toolsets:   []tools.ToolsetID{"agh__catalog"},
			},
			Availability: ToolAvailabilityPayload{
				Registered: true,
				Enabled:    true,
				Available:  true,
				Authorized: true,
				Executable: true,
			},
			Decision: ToolPolicyDecisionPayload{
				VisibleToOperator: true,
				VisibleToSession:  true,
				Callable:          true,
			},
		}}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(ToolResponse) error = %v", err)
		}
		encoded := string(data)
		if !strings.Contains(encoded, `"tool_id":"agh__skill_view"`) {
			t.Fatalf("encoded payload missing canonical tool_id: %s", encoded)
		}
		if !strings.Contains(encoded, `"kind":"native_go"`) {
			t.Fatalf("encoded payload missing backend kind: %s", encoded)
		}
		if !strings.Contains(encoded, `"friendly_verb":"Reading"`) ||
			!strings.Contains(encoded, `"preview":"arg:name"`) {
			t.Fatalf("encoded payload missing presentation metadata: %s", encoded)
		}
		if strings.Contains(encoded, "approval_token") || strings.Contains(encoded, "refresh_token") {
			t.Fatalf("encoded descriptor leaked token-shaped fields: %s", encoded)
		}

		errorData, err := json.Marshal(ToolErrorResponse{Error: ToolErrorPayload{
			Code:        tools.ErrorCodeApprovalRequired,
			Message:     "approval required",
			ToolID:      tools.ToolIDSkillView,
			ReasonCodes: []tools.ReasonCode{tools.ReasonApprovalTokenMissing},
			Layer:       "approval",
		}})
		if err != nil {
			t.Fatalf("json.Marshal(ToolErrorResponse) error = %v", err)
		}
		if !strings.Contains(string(errorData), `"code":"tool_approval_required"`) ||
			!strings.Contains(string(errorData), `"reason_codes":["approval_token_missing"]`) {
			t.Fatalf("structured error missing code or reason: %s", errorData)
		}
		if strings.Contains(string(errorData), "approval-token-secret") {
			t.Fatalf("structured error leaked approval token: %s", errorData)
		}
	})
}
