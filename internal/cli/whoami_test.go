package cli

import (
	"encoding/json"
	"testing"
)

func TestWhoamiReadsSandbox(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		envSessionID: "sess-1",
		envAgentID:   "agent-1",
		envAgentName: "coder",
	}
	deps := newTestDeps(t, &stubClient{})
	deps.getenv = func(key string) string {
		return values[key]
	}

	stdout, _, err := executeRootCommand(t, deps, "whoami", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}

	var decoded IdentityRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.SessionID != "sess-1" || decoded.Agent != "agent-1" || decoded.AgentName != "coder" {
		t.Fatalf("decoded = %#v, want env-backed identity", decoded)
	}
}
