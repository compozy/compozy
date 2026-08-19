package globaldb

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Suite: durable asynchronous tool approvals.
// Invariant: pending approvals are workspace-owned, resolve once, atomically fence dispatch,
// recover ambiguous execution without replay, and survive the 00069 upgrade/reopen boundary.
// Owning layer: GlobalDB pending-approval repository. Canonical suite: this file.
func TestGlobalDBToolApprovalPending(t *testing.T) {
	t.Parallel()

	t.Run("Should fence transitions recover ambiguous dispatch and cascade workspace deletion", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "approval-pending", t.TempDir())
		now := approvalGrantTestTime()
		created, err := globalDB.CreateApproval(
			ctx,
			"apr_pending",
			pendingApprovalTestRequest(workspaceID, "invocation-pending", now.Add(time.Minute)),
			now,
		)
		if err != nil {
			t.Fatalf("CreateApproval() error = %v", err)
		}
		if created.ApprovalStatus != toolspkg.ApprovalPending || created.ResumeFence {
			t.Fatalf("CreateApproval() = %#v, want unfenced pending", created)
		}
		claimed, err := globalDB.ResolveApproval(
			ctx,
			created.ApprovalID,
			toolspkg.ApprovalApproved,
			now.Add(time.Second),
		)
		if err != nil {
			t.Fatalf("ResolveApproval() error = %v", err)
		}
		if claimed.ExecutionStatus != toolspkg.ApprovalDispatching || !claimed.ResumeFence {
			t.Fatalf("ResolveApproval() = %#v, want atomically fenced dispatch", claimed)
		}
		if _, err := globalDB.ResolveApproval(
			ctx,
			created.ApprovalID,
			toolspkg.ApprovalDenied,
			now.Add(2*time.Second),
		); !errors.Is(err, toolspkg.ErrApprovalTerminal) {
			t.Fatalf("ResolveApproval(duplicate) error = %v, want ErrApprovalTerminal", err)
		}
		recovered, err := globalDB.RecoverDispatchingApprovals(ctx, now.Add(3*time.Second))
		if err != nil {
			t.Fatalf("RecoverDispatchingApprovals() error = %v", err)
		}
		if len(recovered) != 1 || recovered[0].ExecutionStatus != toolspkg.ApprovalUncertain {
			t.Fatalf("RecoverDispatchingApprovals() = %#v, want one uncertain", recovered)
		}
		if err := globalDB.DeleteWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM tool_approval_pending WHERE workspace_id = ?`,
			workspaceID,
		).Scan(&count); err != nil {
			t.Fatalf("query pending approval cascade count error = %v", err)
		}
		if count != 0 {
			t.Fatalf("pending approval cascade count = %d, want 0", count)
		}
	})
}

func TestGlobalDBToolApprovalPendingMigration(t *testing.T) {
	t.Run("Should upgrade 00068 to 00069 and preserve pending state across reopen [IT-020]", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00069_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open 00068 prefix error = %v", err)
		}
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: approvalGrantTestTime}
		prefixGlobalDB.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(t, prefixGlobalDB, "approval-00069", t.TempDir())
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(00069 upgrade) error = %v", err)
		}
		ctx := testutil.Context(t)
		assertTableHasColumns(t, upgraded.db, "tool_approval_pending", []string{
			"approval_id", "workspace_id", "invocation_id", "approval_status",
			"execution_status", "resume_fence", "expires_at",
		})
		now := approvalGrantTestTime()
		created, err := upgraded.CreateApproval(
			ctx,
			"apr_upgrade",
			pendingApprovalTestRequest(workspaceID, "invocation-upgrade", now.Add(time.Minute)),
			now,
		)
		if err != nil {
			t.Fatalf("CreateApproval(upgraded) error = %v", err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("Close(upgraded) error = %v", err)
		}

		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		status, err := reopened.GetApproval(testutil.Context(t), created.ApprovalID)
		if err != nil || status.InvocationID != "invocation-upgrade" ||
			status.ApprovalStatus != toolspkg.ApprovalPending {
			t.Fatalf("GetApproval(reopened) = %#v, error = %v", status, err)
		}
		migrationStatus, err := store.Status(testutil.Context(t), reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(reopened) error = %v", err)
		}
		assertCompleteMigrationStream(t, migrationStatus, MigrationStream())
	})
}

func pendingApprovalTestRequest(
	workspaceID string,
	invocationID string,
	expiresAt time.Time,
) toolspkg.ApprovalRequest {
	return toolspkg.ApprovalRequest{
		WorkspaceID: workspaceID, InvocationID: invocationID,
		Target: toolspkg.ApprovalTarget{
			Kind: toolspkg.ApprovalTargetTool, ToolID: toolspkg.ToolID("compozy__test"),
		},
		Args: json.RawMessage(`{"value":1}`), ExpiresAt: expiresAt,
	}
}
