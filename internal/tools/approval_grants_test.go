package tools

import (
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/store"
)

func TestApprovalGrantSetRequestBuildGrant(t *testing.T) {
	t.Parallel()

	t.Run("Should build an agent-wide grant without an input digest", func(t *testing.T) {
		t.Parallel()

		grant, err := (ApprovalGrantSetRequest{
			ToolID:    ToolIDWorkspaceList,
			Decision:  ApprovalGrantAllow,
			Scope:     ApprovalGrantScopeAgent,
			AgentName: " codex ",
		}).BuildGrant(store.DefaultProfileID, " ws-1 ")
		if err != nil {
			t.Fatalf("BuildGrant() error = %v", err)
		}
		if grant.ProfileID != store.DefaultProfileID || grant.WorkspaceID != "ws-1" || grant.AgentName != "codex" ||
			grant.InputDigest != "" {
			t.Fatalf("BuildGrant() key = %#v, want agent-wide ws-1/codex with no digest", grant.ApprovalGrantKey)
		}
		if grant.ToolID != ToolIDWorkspaceList || grant.Decision != ApprovalGrantAllow {
			t.Fatalf("BuildGrant() = %#v, want workspace-list allow", grant)
		}
	})

	t.Run("Should build a tool-wide grant without an agent or input digest", func(t *testing.T) {
		t.Parallel()

		grant, err := (ApprovalGrantSetRequest{
			ToolID:   ToolIDWorkspaceList,
			Decision: ApprovalGrantReject,
			Scope:    ApprovalGrantScopeTool,
		}).BuildGrant(store.DefaultProfileID, "ws-1")
		if err != nil {
			t.Fatalf("BuildGrant() error = %v", err)
		}
		if grant.ProfileID != store.DefaultProfileID || grant.AgentName != "" || grant.InputDigest != "" {
			t.Fatalf("BuildGrant() key = %#v, want tool-wide key", grant.ApprovalGrantKey)
		}
		if grant.Decision != ApprovalGrantReject {
			t.Fatalf("BuildGrant() decision = %q, want reject", grant.Decision)
		}
	})

	t.Run("Should reject scope and agent combinations outside the management contract", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			request ApprovalGrantSetRequest
		}{
			{
				name: "missing agent",
				request: ApprovalGrantSetRequest{
					ToolID: ToolIDWorkspaceList, Decision: ApprovalGrantAllow, Scope: ApprovalGrantScopeAgent,
				},
			},
			{
				name: "agent on tool scope",
				request: ApprovalGrantSetRequest{
					ToolID: ToolIDWorkspaceList, Decision: ApprovalGrantAllow, Scope: ApprovalGrantScopeTool,
					AgentName: "codex",
				},
			},
			{
				name: "unknown scope",
				request: ApprovalGrantSetRequest{
					ToolID: ToolIDWorkspaceList, Decision: ApprovalGrantAllow, Scope: "input",
				},
			},
		}
		for _, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()

				_, err := test.request.BuildGrant(store.DefaultProfileID, "ws-1")
				if !errors.Is(err, ErrApprovalGrantInvalid) {
					t.Fatalf("BuildGrant() error = %v, want ErrApprovalGrantInvalid", err)
				}
			})
		}
	})

	t.Run("Should reject a grant without a profile", func(t *testing.T) {
		t.Parallel()

		_, err := (ApprovalGrantSetRequest{
			ToolID: ToolIDWorkspaceList, Decision: ApprovalGrantAllow, Scope: ApprovalGrantScopeTool,
		}).BuildGrant("", "ws-1")
		if err == nil || !errors.Is(err, ErrApprovalGrantInvalid) ||
			!strings.Contains(err.Error(), "profile_id is required") {
			t.Fatalf("BuildGrant(missing profile) error = %v, want profile_id is required", err)
		}
	})

	t.Run("Should use ordinary wider grants for terminal writes", func(t *testing.T) {
		t.Parallel()

		grant, err := (ApprovalGrantSetRequest{
			ToolID: ToolIDTerminalWrite, Decision: ApprovalGrantAllow, Scope: ApprovalGrantScopeAgent,
			AgentName: "codex",
		}).BuildGrant(store.DefaultProfileID, "ws-1")
		if err != nil {
			t.Fatalf("BuildGrant(terminal write) error = %v", err)
		}
		if grant.ToolID != ToolIDTerminalWrite || grant.AgentName != "codex" {
			t.Fatalf("BuildGrant(terminal write) = %#v, want agent-scoped ordinary grant", grant)
		}
	})

	t.Run("Should keep command execution grants exact", func(t *testing.T) {
		t.Parallel()

		_, err := (ApprovalGrantSetRequest{
			ToolID: ToolIDTerminalExec, Decision: ApprovalGrantAllow, Scope: ApprovalGrantScopeAgent,
			AgentName: "codex",
		}).BuildGrant(store.DefaultProfileID, "ws-1")
		if !errors.Is(err, ErrApprovalGrantInvalid) ||
			!strings.Contains(err.Error(), "requires an exact prompt-origin grant") {
			t.Fatalf("BuildGrant(terminal exec) error = %v, want exact prompt-origin refusal", err)
		}
	})
}
