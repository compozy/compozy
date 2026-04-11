package kinds

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReusableAgentLifecyclePayloadJSONPreservesMeaningfulZeroValues(t *testing.T) {
	t.Parallel()

	payload := ReusableAgentLifecyclePayload{
		Stage:          ReusableAgentLifecycleStageNestedCompleted,
		AgentName:      "child",
		NestedDepth:    0,
		MaxNestedDepth: 0,
		Success:        false,
		Blocked:        false,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal reusable agent payload: %v", err)
	}
	jsonText := string(raw)
	for _, want := range []string{
		`"nested_depth":0`,
		`"max_nested_depth":0`,
		`"success":false`,
		`"blocked":false`,
	} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("expected payload JSON to contain %s, got %s", want, jsonText)
		}
	}
}
