package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	coordinationReasonEligible            = "eligible"
	coordinationReasonAlreadyEnabled      = "already_enabled"
	coordinationReasonInvitationDismissed = "invitation_dismissed"
	coordinationReasonNetworkUnavailable  = "network_unavailable"
	coordinationReasonRunRequired         = "run_required"
	coordinationReasonRunInactive         = "run_inactive"
	coordinationReasonCoordinatorRequired = "coordinator_required"
	coordinationReasonWorkersRequired     = "workers_required"
)

func (g *WorkspaceRepo) coordinationViewWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	ref workspacepkg.CoordinationRef,
) (workspacepkg.CoordinationView, error) {
	if err := g.validateCoordinationRefWithExecutor(ctx, exec, ref); err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	setting, err := loadCoordinationSetting(ctx, exec, ref)
	if err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	invitation, err := loadCoordinationInvitationWithExecutor(ctx, exec, ref)
	if err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	eligibility, err := coordinationEligibilityWithExecutor(ctx, exec, ref, setting, invitation)
	if err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	return workspacepkg.CoordinationView{
		Setting: setting, Invitation: invitation, Eligibility: eligibility,
	}, nil
}

func (g *WorkspaceRepo) validateCoordinationRefWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	ref workspacepkg.CoordinationRef,
) error {
	if ref.ScopeKind == workspacepkg.InvitationScopeWorkspace {
		var exists int
		if err := exec.QueryRowContext(ctx, `SELECT 1 FROM workspaces WHERE id = ?`, ref.WorkspaceID).
			Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return workspacepkg.ErrWorkspaceNotFound
			}
			return fmt.Errorf("store: validate coordination workspace: %w", err)
		}
		return nil
	}
	var taskWorkspace string
	if err := exec.QueryRowContext(ctx, `SELECT workspace_id FROM tasks WHERE id = ?`, ref.TaskID).
		Scan(&taskWorkspace); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskNotFound
		}
		return fmt.Errorf("store: validate coordination task: %w", err)
	}
	if strings.TrimSpace(taskWorkspace) != ref.WorkspaceID {
		return fmt.Errorf(
			"%w: task %q does not belong to workspace %q",
			workspacepkg.ErrCoordinationScopeInvalid,
			ref.TaskID,
			ref.WorkspaceID,
		)
	}
	return nil
}

func loadCoordinationInvitationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	ref workspacepkg.CoordinationRef,
) (workspacepkg.CoordinationInvitation, error) {
	scopeID := coordinationScopeID(ref)
	row, err := sqlcgen.New(exec).GetNetworkCoordinationInvitation(
		ctx,
		sqlcgen.GetNetworkCoordinationInvitationParams{
			WorkspaceID: ref.WorkspaceID,
			ScopeKind:   ref.ScopeKind,
			ScopeID:     scopeID,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return workspacepkg.CoordinationInvitation{
			WorkspaceID: ref.WorkspaceID, ScopeKind: ref.ScopeKind, ScopeID: scopeID,
		}, nil
	}
	if err != nil {
		return workspacepkg.CoordinationInvitation{}, fmt.Errorf("store: load coordination invitation: %w", err)
	}
	return invitationFromGenerated(row)
}

func coordinationEligibilityWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	ref workspacepkg.CoordinationRef,
	setting workspacepkg.CoordinationSetting,
	invitation workspacepkg.CoordinationInvitation,
) (workspacepkg.CoordinationEligibility, error) {
	result := workspacepkg.CoordinationEligibility{RunID: ref.RunID}
	if setting.Enabled {
		result.Reason = coordinationReasonAlreadyEnabled
		return result, nil
	}
	if invitation.Dismissed {
		result.Reason = coordinationReasonInvitationDismissed
		return result, nil
	}
	available, err := coordinationAvailability(ctx, exec)
	if err != nil {
		return workspacepkg.CoordinationEligibility{}, err
	}
	if !available {
		result.Reason = coordinationReasonNetworkUnavailable
		return result, nil
	}
	if ref.ScopeKind != workspacepkg.InvitationScopeTask || ref.RunID == "" {
		result.Reason = coordinationReasonRunRequired
		return result, nil
	}
	var taskID, workspaceID, status, runKind, loopRunID string
	err = exec.QueryRowContext(
		ctx,
		`SELECT COALESCE(task_id, ''), COALESCE(workspace_id, ''), status, run_kind, COALESCE(loop_run_id, '')
         FROM task_runs WHERE id = ?`,
		ref.RunID,
	).Scan(&taskID, &workspaceID, &status, &runKind, &loopRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return workspacepkg.CoordinationEligibility{}, taskpkg.ErrTaskRunNotFound
	}
	if err != nil {
		return workspacepkg.CoordinationEligibility{}, fmt.Errorf("store: load coordination run: %w", err)
	}
	if strings.TrimSpace(taskID) != ref.TaskID || strings.TrimSpace(workspaceID) != ref.WorkspaceID {
		return workspacepkg.CoordinationEligibility{}, fmt.Errorf(
			"%w: run %q does not belong to task %q in workspace %q",
			workspacepkg.ErrCoordinationScopeInvalid,
			ref.RunID,
			ref.TaskID,
			ref.WorkspaceID,
		)
	}
	result.Coordinator = strings.TrimSpace(runKind) == taskpkg.RunKindCoordinator.String()
	if status != taskpkg.TaskRunStatusClaimed.String() &&
		status != taskpkg.TaskRunStatusStarting.String() &&
		status != taskpkg.TaskRunStatusRunning.String() {
		result.Reason = coordinationReasonRunInactive
		return result, nil
	}
	if !result.Coordinator {
		result.Reason = coordinationReasonCoordinatorRequired
		return result, nil
	}
	if strings.TrimSpace(loopRunID) != "" {
		if err := exec.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ? AND run_kind = ?`,
			loopRunID,
			taskpkg.RunKindWorker.String(),
		).Scan(&result.WorkerCount); err != nil {
			return workspacepkg.CoordinationEligibility{}, fmt.Errorf("store: count coordination workers: %w", err)
		}
	}
	if result.WorkerCount < 2 {
		result.Reason = coordinationReasonWorkersRequired
		return result, nil
	}
	result.Eligible = true
	result.Reason = coordinationReasonEligible
	return result, nil
}

func requireCoordinationAvailability(ctx context.Context, exec taskSQLExecutor) error {
	available, err := coordinationAvailability(ctx, exec)
	if err != nil {
		return err
	}
	if !available {
		return participation.ErrUnavailable
	}
	return nil
}

func coordinationAvailability(ctx context.Context, exec taskSQLExecutor) (bool, error) {
	availability, err := sqlcgen.New(exec).GetNetworkAvailability(ctx)
	if err != nil {
		return false, fmt.Errorf("store: read network availability for coordination: %w", err)
	}
	return availability.Enabled != 0, nil
}
