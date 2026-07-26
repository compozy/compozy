package cli

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/resources"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestAPIStateTransitionCommandsExposeExactLeaves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "ShouldExposeSessionApprove", args: []string{"session", "approve"}},
		{name: "ShouldExposeBridgeSecretBindingDelete", args: []string{"bridge", "secret-bindings", "delete"}},
		{name: "ShouldExposeResourceDelete", args: []string{"resource", "delete"}},
		{name: "ShouldExposeTaskDelete", args: []string{"task", "delete"}},
		{name: "ShouldExposeTaskReject", args: []string{"task", "reject"}},
		{name: "ShouldExposeSkillEnable", args: []string{"skill", "enable"}},
		{name: "ShouldExposeSkillDisable", args: []string{"skill", "disable"}},
		{name: "ShouldExposeToolApprove", args: []string{"tool", "approve"}},
		{name: "ShouldExposeWorkspaceEditAlias", args: []string{"workspace", "edit"}},
		{name: "ShouldExposeAgentHeartbeatWake", args: []string{"agent", "heartbeat", "wake"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := newRootCommand(commandDeps{})
			cmd, remaining, err := root.Find(tt.args)
			if err != nil {
				t.Fatalf("Find(%v) error = %v", tt.args, err)
			}
			if len(remaining) != 0 {
				t.Fatalf("Find(%v) remaining args = %v, want none", tt.args, remaining)
			}
			if cmd == nil {
				t.Fatalf("Find(%v) command = nil", tt.args)
			}
			if got := strings.TrimSpace(cmd.CommandPath()); got != "agh "+strings.Join(tt.args, " ") {
				t.Fatalf("command path = %q, want %q", got, "agh "+strings.Join(tt.args, " "))
			}
		})
	}
}

func TestAPIStateTransitionCommandsCallDaemonClient(t *testing.T) {
	t.Parallel()

	t.Run("ShouldApproveSessionThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			approveSessionFn: func(_ context.Context, id string, request SessionApprovalRequest) (SessionApprovalRecord, error) {
				if id != "sess-1" {
					t.Fatalf("session id = %q, want sess-1", id)
				}
				if request.RequestID != "req-1" || request.Decision != "allow-once" {
					t.Fatalf("approval request = %#v", request)
				}
				return SessionApprovalRecord{Status: "approved"}, nil
			},
		}
		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"session",
			"approve",
			"sess-1",
			"--request-id",
			"req-1",
			"--decision",
			"allow-once",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("session approve error = %v; stderr=%s", err, stderr)
		}
		var payload struct {
			SessionID string `json:"session_id"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode stdout %q: %v", stdout, err)
		}
		if payload.SessionID != "sess-1" || payload.Status != "approved" {
			t.Fatalf("session approval payload = %#v", payload)
		}
	})

	t.Run("ShouldDeleteBridgeSecretBindingThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			deleteBridgeSecretBindingFn: func(_ context.Context, id string, bindingName string) error {
				if id != "bridge-1" || bindingName != "bot-token" {
					t.Fatalf("delete binding = %q/%q, want bridge-1/bot-token", id, bindingName)
				}
				return nil
			},
		}
		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"bridge",
			"secret-bindings",
			"delete",
			"bridge-1",
			"bot-token",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("bridge secret binding delete error = %v; stderr=%s", err, stderr)
		}
		var payload struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode stdout %q: %v", stdout, err)
		}
		if payload.Status != "deleted" {
			t.Fatalf("delete payload = %#v", payload)
		}
	})

	t.Run("ShouldDeleteResourceThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			deleteResourceFn: func(_ context.Context, kind string, id string, request ResourceDeleteRequest) error {
				if kind != "agent" || id != "general" || request.ExpectedVersion != 7 {
					t.Fatalf("delete resource = %q/%q %#v, want agent/general version 7", kind, id, request)
				}
				return nil
			},
		}
		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"resource",
			"delete",
			"agent",
			"general",
			"--expected-version",
			"7",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("resource delete error = %v; stderr=%s", err, stderr)
		}
		var payload struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode stdout %q: %v", stdout, err)
		}
		if payload.Status != "deleted" {
			t.Fatalf("delete payload = %#v", payload)
		}
	})

	t.Run("ShouldPutResourceThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			putResourceFn: func(_ context.Context, kind string, id string, request ResourcePutRequest) (ResourceRecord, error) {
				if kind != "agent" || id != "general" {
					t.Fatalf("put resource = %q/%q, want agent/general", kind, id)
				}
				if request.Scope.Kind != resources.ResourceScopeKindWorkspace || request.Scope.ID != "ws-1" {
					t.Fatalf("resource scope = %#v, want workspace ws-1", request.Scope)
				}
				if string(request.Spec) != `{"name":"general"}` {
					t.Fatalf("resource spec = %s, want compact general spec", request.Spec)
				}
				return ResourceRecord{
					Kind:    resources.ResourceKind("agent"),
					ID:      "general",
					Version: 1,
					Scope: resources.ResourceScope{
						Kind: resources.ResourceScopeKindWorkspace,
						ID:   "ws-1",
					},
					Spec:      request.Spec,
					CreatedAt: time.Unix(1, 0).UTC(),
					UpdatedAt: time.Unix(2, 0).UTC(),
				}, nil
			},
		}
		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"resource",
			"put",
			"agent",
			"general",
			"--scope",
			"workspace",
			"--scope-id",
			"ws-1",
			"--spec",
			`{"name":"general"}`,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("resource put error = %v; stderr=%s", err, stderr)
		}
		var payload struct {
			Record ResourceRecord `json:"record"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode stdout %q: %v", stdout, err)
		}
		if payload.Record.ID != "general" || payload.Record.Version != 1 {
			t.Fatalf("put payload = %#v", payload)
		}
	})

	t.Run("ShouldEnableAndDisableSkillsThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		var calls []string
		client := &stubClient{
			enableSkillFn: func(_ context.Context, name string, query SkillQuery) (SkillActionRecord, error) {
				calls = append(calls, "enable:"+name+":"+query.Workspace)
				return SkillActionRecord{OK: true}, nil
			},
			disableSkillFn: func(_ context.Context, name string, query SkillQuery) (SkillActionRecord, error) {
				calls = append(calls, "disable:"+name+":"+query.Workspace)
				return SkillActionRecord{OK: true}, nil
			},
		}
		deps := newTestDeps(t, client)
		if _, stderr, err := executeRootCommand(
			t,
			deps,
			"skill",
			"enable",
			"qa",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("skill enable error = %v; stderr=%s", err, stderr)
		}
		if _, stderr, err := executeRootCommand(
			t,
			deps,
			"skill",
			"disable",
			"qa",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("skill disable error = %v; stderr=%s", err, stderr)
		}
		if want := []string{"enable:qa:ws-1", "disable:qa:ws-1"}; !reflect.DeepEqual(calls, want) {
			t.Fatalf("skill calls = %#v, want %#v", calls, want)
		}
	})

	t.Run("ShouldApproveToolThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			createToolApprovalFn: func(_ context.Context, id string, request ToolApprovalRequest) (ToolApprovalRecord, error) {
				if id != toolspkg.ToolIDToolInfo.String() {
					t.Fatalf("tool id = %q, want %q", id, toolspkg.ToolIDToolInfo)
				}
				if request.SessionID != "sess-1" || request.WorkspaceID != "ws-1" || request.AgentName != "general" {
					t.Fatalf("tool approval scope = %#v", request)
				}
				if string(request.Input) != `{"tool_id":"agh__skill_view"}` {
					t.Fatalf("tool approval input = %s", request.Input)
				}
				return ToolApprovalRecord{
					ApprovalToken: "agh_tool_approval_test",
					ToolID:        toolspkg.ToolIDToolInfo,
					InputDigest:   "sha256:test",
					ExpiresAt:     time.Unix(3, 0).UTC(),
				}, nil
			},
		}
		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"approve",
			toolspkg.ToolIDToolInfo.String(),
			"--session",
			"sess-1",
			"--workspace",
			"ws-1",
			"--agent",
			"general",
			"--input",
			`{"tool_id":"agh__skill_view"}`,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool approve error = %v; stderr=%s", err, stderr)
		}
		var payload struct {
			Approval ToolApprovalRecord `json:"approval"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode stdout %q: %v", stdout, err)
		}
		if payload.Approval.ApprovalToken != "agh_tool_approval_test" {
			t.Fatalf("tool approval payload = %#v", payload)
		}
	})

	t.Run("ShouldDeleteAndRejectTasksThroughUDSClient", func(t *testing.T) {
		t.Parallel()

		var calls []string
		client := &stubClient{
			deleteTaskFn: func(_ context.Context, id string) error {
				calls = append(calls, "delete:"+id)
				return nil
			},
			rejectTaskFn: func(_ context.Context, id string) (TaskRecord, error) {
				calls = append(calls, "reject:"+id)
				return TaskRecord{ID: id, ApprovalState: taskpkg.ApprovalStateRejected}, nil
			},
		}
		deps := newTestDeps(t, client)
		if _, stderr, err := executeRootCommand(t, deps, "task", "delete", "task-1", "-o", "json"); err != nil {
			t.Fatalf("task delete error = %v; stderr=%s", err, stderr)
		}
		if _, stderr, err := executeRootCommand(t, deps, "task", "reject", "task-1", "-o", "json"); err != nil {
			t.Fatalf("task reject error = %v; stderr=%s", err, stderr)
		}
		if want := []string{"delete:task-1", "reject:task-1"}; !reflect.DeepEqual(calls, want) {
			t.Fatalf("task calls = %#v, want %#v", calls, want)
		}
	})
}
