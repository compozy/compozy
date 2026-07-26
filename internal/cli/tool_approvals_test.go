package cli

import (
	"context"
	"testing"
	"time"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestToolApprovalGrantCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should set an agent-wide approval as structured JSON", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			setToolApprovalGrantFn: func(
				_ context.Context,
				workspaceID string,
				request ToolApprovalGrantSetRequest,
			) (ToolApprovalGrantRecord, error) {
				if workspaceID != "ws-1" || request.ToolID != "agh__approval_probe" {
					t.Fatalf("SetToolApprovalGrant workspace/tool = %q/%q", workspaceID, request.ToolID)
				}
				if request.Decision != toolspkg.ApprovalGrantAllow ||
					request.Scope != toolspkg.ApprovalGrantScopeAgent || request.AgentName != "codex" {
					t.Fatalf("SetToolApprovalGrant request = %#v, want agent-wide allow", request)
				}
				grant := toolApprovalGrantCLIRecord()
				grant.InputDigest = ""
				return grant, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"approvals",
			"set",
			"agh__approval_probe",
			"--workspace",
			"ws-1",
			"--decision",
			"allow",
			"--scope",
			"agent",
			"--agent",
			"codex",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool approvals set error = %v", err)
		}
		var payload ToolApprovalGrantRecord
		decodeJSONOutput(t, stdout, &payload)
		if payload.ID != "grant-1" || payload.AgentName != "codex" || payload.InputDigest != "" {
			t.Fatalf("tool approvals set payload = %#v, want agent-wide grant", payload)
		}
	})

	t.Run("Should reject an agent on tool-wide scope before calling the daemon", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, &stubClient{}),
			"tool",
			"approvals",
			"set",
			"agh__approval_probe",
			"--workspace",
			"ws-1",
			"--decision",
			"allow",
			"--scope",
			"tool",
			"--agent",
			"codex",
		)
		if err == nil {
			t.Fatal("tool approvals set with tool scope and agent error = nil")
		}
	})

	t.Run("Should list remembered approvals as structured JSON", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listToolApprovalGrantsFn: func(
				_ context.Context,
				workspaceID string,
			) (ToolApprovalGrantListRecord, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ListToolApprovalGrants workspace_id = %q, want ws-1", workspaceID)
				}
				grant := toolApprovalGrantCLIRecord()
				return ToolApprovalGrantListRecord{Grants: []ToolApprovalGrantRecord{grant}, Total: 1}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"approvals",
			"list",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool approvals list error = %v", err)
		}
		var payload ToolApprovalGrantListRecord
		decodeJSONOutput(t, stdout, &payload)
		if payload.Total != 1 || len(payload.Grants) != 1 || payload.Grants[0].ID != "grant-1" {
			t.Fatalf("tool approvals list payload = %#v", payload)
		}
	})

	t.Run("Should revoke one remembered approval as structured JSON", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			revokeToolApprovalGrantFn: func(_ context.Context, workspaceID, id string) error {
				if workspaceID != "ws-1" || id != "grant-1" {
					t.Fatalf("RevokeToolApprovalGrant(%q, %q), want ws-1/grant-1", workspaceID, id)
				}
				return nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"approvals",
			"revoke",
			"grant-1",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool approvals revoke error = %v", err)
		}
		var payload struct {
			RevokedID   string `json:"revoked_id"`
			WorkspaceID string `json:"workspace_id"`
		}
		decodeJSONOutput(t, stdout, &payload)
		if payload.RevokedID != "grant-1" || payload.WorkspaceID != "ws-1" {
			t.Fatalf("tool approvals revoke payload = %#v", payload)
		}
	})

	t.Run("Should require an explicit workspace", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), "tool", "approvals", "list")
		if err == nil {
			t.Fatal("tool approvals list without workspace error = nil")
		}
	})
}

func toolApprovalGrantCLIRecord() ToolApprovalGrantRecord {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	return ToolApprovalGrantRecord{
		ID:          "grant-1",
		WorkspaceID: "ws-1",
		AgentName:   "codex",
		ToolID:      toolspkg.ToolID("agh__approval_probe"),
		InputDigest: "sha256:abc",
		Decision:    toolspkg.ApprovalGrantAllow,
		CreatedAt:   now,
		LastUsedAt:  now,
	}
}
