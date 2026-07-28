package hooks

import (
	"strings"
	"testing"
)

func TestAgentAuthoredContextMatcherValidationContract(t *testing.T) {
	tests := []struct {
		name  string
		event HookEvent
	}{
		{name: "Should reject workspace root for Soul snapshot resolved hooks", event: HookAgentSoulSnapshotResolved},
		{name: "Should reject workspace root for Soul mutation hooks", event: HookAgentSoulMutationAfter},
		{name: "Should reject workspace root for Heartbeat policy hooks", event: HookAgentHeartbeatPolicyResolved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if MatcherFieldAllowedForEvent(tt.event, "workspace_root") {
				t.Fatalf("MatcherFieldAllowedForEvent(%q, workspace_root) = true, want false", tt.event)
			}

			err := ValidateHookDecl(HookDecl{
				Name:    "authored-context-root",
				Event:   tt.event,
				Source:  HookSourceConfig,
				Command: "./hook.sh",
				Matcher: HookMatcher{
					AgentName:     "coder",
					WorkspaceID:   "ws-1",
					WorkspaceRoot: "/workspace/a",
				},
			})
			if err == nil {
				t.Fatal("ValidateHookDecl() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), "workspace_root") || !strings.Contains(err.Error(), string(tt.event)) {
				t.Fatalf("ValidateHookDecl() error = %q, want workspace_root detail for %q", err, tt.event)
			}
		})
	}
}
