package automation

import (
	"strings"
	"testing"
)

func TestExtensionTriggerRequestValidateRequiresExtPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request ExtensionTriggerRequest
		wantErr string
	}{
		{
			name: "Should accept workspace scoped ext event",
			request: ExtensionTriggerRequest{
				Event:       "ext.github.push",
				Scope:       AutomationScopeWorkspace,
				WorkspaceID: "ws-1",
				Payload:     map[string]any{"repo": "acme/api"},
			},
		},
		{
			name: "Should reject built in event names",
			request: ExtensionTriggerRequest{
				Event: "session.stopped",
				Scope: AutomationScopeGlobal,
			},
			wantErr: `must start with "ext."`,
		},
		{
			name: "Should reject surrounding whitespace",
			request: ExtensionTriggerRequest{
				Event: " ext.github.push ",
				Scope: AutomationScopeGlobal,
			},
			wantErr: "must not contain surrounding whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.request.Validate("trigger_fire")
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Validate() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}
