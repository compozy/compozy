package daemon

import (
	"encoding/json"
	"errors"
	"testing"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDaemonNativeToolApprovalGrants(t *testing.T) {
	t.Parallel()

	t.Run("Should set explicit agent-wide and tool-wide grants", func(t *testing.T) {
		t.Parallel()

		grantStore := &recordingApprovalGrantStore{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ApprovalGrants: grantStore,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{WorkspaceID: "ws-a", AgentName: "codex"}

		result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolApprovalsSet,
			Input: json.RawMessage(
				`{"tool_id":"agh__approval_probe","decision":"allow","scope":"agent","agent_name":"codex"}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_approvals_set agent) error = %v", err)
		}
		var set nativeToolApprovalSetResponse
		if err := json.Unmarshal(result.Structured, &set); err != nil {
			t.Fatalf("Unmarshal(agent set result) error = %v", err)
		}
		if set.Grant.WorkspaceID != "ws-a" || set.Grant.AgentName != "codex" || set.Grant.InputDigest != "" {
			t.Fatalf("agent set result = %#v, want ws-a/codex with no digest", set)
		}

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolApprovalsSet,
			Input: json.RawMessage(
				`{"tool_id":"agh__approval_probe","decision":"reject","scope":"tool"}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_approvals_set tool) error = %v", err)
		}
		grants, err := grantStore.ListApprovalGrants(t.Context(), "ws-a")
		if err != nil {
			t.Fatalf("ListApprovalGrants() error = %v", err)
		}
		if len(grants) != 2 || grants[1].AgentName != "" || grants[1].InputDigest != "" {
			t.Fatalf("stored grants = %#v, want agent-wide and tool-wide keys", grants)
		}
	})

	t.Run("Should list and revoke only the caller workspace grant", func(t *testing.T) {
		t.Parallel()

		grantStore := &recordingApprovalGrantStore{grants: []toolspkg.ApprovalGrant{
			materializedApprovalGrant("grant-a", toolspkg.ApprovalGrantKey{
				WorkspaceID: "ws-a",
				AgentName:   "codex",
				ToolID:      "agh__approval_probe",
				InputDigest: "sha256:abc",
			}, toolspkg.ApprovalGrantAllow),
			materializedApprovalGrant("grant-b", toolspkg.ApprovalGrantKey{
				WorkspaceID: "ws-b",
				ToolID:      "agh__approval_probe",
			}, toolspkg.ApprovalGrantReject),
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ApprovalGrants: grantStore,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{WorkspaceID: "ws-a", AgentName: "codex"}

		result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolApprovalsList,
			Input:  json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_approvals_list) error = %v", err)
		}
		var listed nativeToolApprovalListResponse
		if err := json.Unmarshal(result.Structured, &listed); err != nil {
			t.Fatalf("Unmarshal(list result) error = %v", err)
		}
		if listed.Total != 1 || len(listed.Grants) != 1 || listed.Grants[0].ID != "grant-a" {
			t.Fatalf("list result = %#v, want grant-a only", listed)
		}

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolApprovalsRevoke,
			Input:  json.RawMessage(`{"id":"grant-a"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_approvals_revoke) error = %v", err)
		}
		remaining, err := grantStore.ListApprovalGrants(t.Context(), "ws-a")
		if err != nil || len(remaining) != 0 {
			t.Fatalf("workspace A grants after revoke = %#v, %v, want empty", remaining, err)
		}
		foreign, err := grantStore.ListApprovalGrants(t.Context(), "ws-b")
		if err != nil || len(foreign) != 1 || foreign[0].ID != "grant-b" {
			t.Fatalf("workspace B grants after revoke = %#v, %v, want grant-b", foreign, err)
		}
	})

	t.Run("Should reject workspace overrides before reading grants", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ApprovalGrants: &recordingApprovalGrantStore{},
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{WorkspaceID: "ws-a"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolApprovalsSet,
			Input: json.RawMessage(
				`{"workspace_id":"ws-b","tool_id":"agh__approval_probe","decision":"allow","scope":"tool"}`,
			),
		})
		if !errors.Is(err, toolspkg.ErrToolDenied) {
			t.Fatalf("Registry.Call(workspace override) error = %v, want ErrToolDenied", err)
		}
	})

	t.Run("Should return a typed not found error without exposing ownership", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ApprovalGrants: &recordingApprovalGrantStore{},
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{WorkspaceID: "ws-a"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolApprovalsRevoke,
			Input:  json.RawMessage(`{"id":"foreign-or-missing"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Registry.Call(missing revoke) error = %v, want ErrToolNotFound", err)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeNotFound {
			t.Fatalf("missing revoke error = %#v, want tool_not_found envelope", err)
		}
	})
}
