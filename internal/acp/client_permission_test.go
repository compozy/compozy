package acp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestPermissionPolicyModes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policies := map[string]struct {
		mode       compozyconfig.PermissionMode
		readOK     bool
		writeOK    bool
		terminalOK bool
	}{
		"deny-all": {
			mode:       compozyconfig.PermissionModeDenyAll,
			readOK:     false,
			writeOK:    false,
			terminalOK: false,
		},
		"approve-reads": {
			mode:       compozyconfig.PermissionModeApproveReads,
			readOK:     true,
			writeOK:    false,
			terminalOK: false,
		},
		"approve-all": {
			mode:       compozyconfig.PermissionModeApproveAll,
			readOK:     true,
			writeOK:    true,
			terminalOK: true,
		},
	}

	for name, tc := range policies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			policy, err := newPermissionPolicy(tc.mode, root)
			if err != nil {
				t.Fatalf("newPermissionPolicy() error = %v", err)
			}

			assertPermissionResult(t, policy.authorize(permissionReadTextFile), tc.readOK)
			assertPermissionResult(t, policy.authorize(permissionWriteTextFile), tc.writeOK)
			assertPermissionResult(t, policy.authorize(permissionCreateTerminal), tc.terminalOK)
		})
	}
}

func TestPermissionPolicyResolvePathSandbox(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveAll, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy() error = %v", err)
	}

	insideFile := filepath.Join(root, "nested", "file.txt")
	resolvedInside, err := policy.resolvePath(insideFile)
	if err != nil {
		t.Fatalf("resolvePath(%q) error = %v", insideFile, err)
	}
	if !strings.HasSuffix(resolvedInside, filepath.Join("nested", "file.txt")) {
		t.Fatalf(
			"resolvePath(%q) = %q, want suffix %q",
			insideFile,
			resolvedInside,
			filepath.Join("nested", "file.txt"),
		)
	}

	if _, err := policy.resolvePath(filepath.Join(root, "..", "escape.txt")); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("resolvePath(outside) error = %v, want ErrPathOutsideWorkspace", err)
	}
}

func TestDriverApprovePermissionValidationAndForwarding(t *testing.T) {
	t.Parallel()

	driver := New(WithPermissionTimeout(123 * time.Millisecond))
	if driver.permissionWait != 123*time.Millisecond {
		t.Fatalf("permissionWait = %v, want %v", driver.permissionWait, 123*time.Millisecond)
	}

	proc := newDirectProcess(t, compozyconfig.PermissionModeDenyAll)
	requestID, pending := proc.registerPendingPermission("turn-1", acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{ToolCallId: "tool-1"},
	})

	if err := driver.ApprovePermission(context.Background(), proc, ApproveRequest{
		RequestID: requestID,
		Decision:  string(decisionAllowOnce),
	}); err != nil {
		t.Fatalf("ApprovePermission() error = %v", err)
	}
	select {
	case decision := <-pending.response:
		if decision != decisionAllowOnce {
			t.Fatalf("pending response = %q, want %q", decision, decisionAllowOnce)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending permission response")
	}

	if err := driver.ApprovePermission(context.Background(), nil, ApproveRequest{
		RequestID: "req-1",
		Decision:  string(decisionAllowOnce),
	}); err == nil {
		t.Fatal("ApprovePermission(nil proc) error = nil, want non-nil")
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := driver.ApprovePermission(canceledCtx, proc, ApproveRequest{
		RequestID: "req-1",
		Decision:  string(decisionAllowOnce),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApprovePermission(canceled ctx) error = %v, want context.Canceled", err)
	}
}
