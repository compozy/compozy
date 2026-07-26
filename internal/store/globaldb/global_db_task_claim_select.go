package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRunRepo) ensureClaimerHasNoActiveLease(
	ctx context.Context,
	exec taskSQLExecutor,
	criteria taskpkg.ClaimCriteria,
) error {
	count, err := sqlcgen.New(exec).CountActiveTaskRunLeasesForSession(
		ctx,
		sqlcgen.CountActiveTaskRunLeasesForSessionParams{
			SessionID:      sql.NullString{String: criteria.ClaimerSessionID, Valid: true},
			ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
			StartingStatus: taskpkg.TaskRunStatusStarting.String(),
			RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
			Now:            nullableTaskTime(criteria.Now),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"store: count active task-run leases for %q: %w",
			criteria.ClaimerSessionID,
			err,
		)
	}
	if count > 0 {
		return fmt.Errorf(
			"%w: session %q already owns an active task-run lease",
			taskpkg.ErrActiveRunLease,
			criteria.ClaimerSessionID,
		)
	}
	return nil
}

func (g *TaskRunRepo) ensureWorkspaceActiveRunCapacity(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	criteria taskpkg.ClaimCriteria,
) error {
	if criteria.WorkspaceActiveRunCap == 0 {
		return nil
	}
	candidate, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
	if err != nil {
		return err
	}
	workspaceID := strings.TrimSpace(candidate.WorkspaceID)
	if workspaceID == "" || candidate.IsNetworkWake() {
		return nil
	}
	count, err := sqlcgen.New(exec).CountActiveTaskRunLeasesForWorkspace(
		ctx,
		sqlcgen.CountActiveTaskRunLeasesForWorkspaceParams{
			WorkspaceID:    sql.NullString{String: workspaceID, Valid: true},
			ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
			StartingStatus: taskpkg.TaskRunStatusStarting.String(),
			RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
			Now:            nullableTaskTime(criteria.Now),
		},
	)
	if err != nil {
		return fmt.Errorf("store: count active task-run leases for workspace %q: %w", workspaceID, err)
	}
	if count >= int64(criteria.WorkspaceActiveRunCap) {
		return fmt.Errorf(
			"%w: workspace %q has %d active runs (limit %d)",
			taskpkg.ErrWorkspaceActiveRunCapReached,
			workspaceID,
			count,
			criteria.WorkspaceActiveRunCap,
		)
	}
	return nil
}

func (g *TaskRunRepo) selectClaimableRunID(
	ctx context.Context,
	exec taskSQLExecutor,
	criteria taskpkg.ClaimCriteria,
) (string, error) {
	if criteria.RunKind.Normalize() == taskpkg.RunKindNetworkWake {
		return g.selectClaimableNetworkWakeRunID(ctx, exec, criteria)
	}
	where, args := baseClaimPredicates(criteria)
	exactRunID := strings.TrimSpace(criteria.RunID)
	workspaceID := strings.TrimSpace(criteria.WorkspaceID)
	if exactRunID != "" {
		where = append(where, "tr.id = ?")
		args = append(args, exactRunID)
	}
	where, args = appendClaimRunKindPredicate(where, args, criteria.RunKind)
	switch {
	case exactRunID != "" && workspaceID != "":
		where = append(where, "(t.scope = ? OR (t.scope = ? AND t.workspace_id = ?))")
		args = append(
			args,
			string(taskpkg.ScopeGlobal),
			string(taskpkg.ScopeWorkspace),
			workspaceID,
		)
	case criteria.Scope == taskpkg.ScopeWorkspace:
		where = append(where, "t.scope = ?", "t.workspace_id = ?")
		args = append(args, string(taskpkg.ScopeWorkspace), workspaceID)
	default:
		where = append(where, "t.scope = ?")
		args = append(args, string(taskpkg.ScopeGlobal))
	}
	if strings.TrimSpace(criteria.ParticipationChannel) != "" {
		where = append(where, "tr.network_channel = ?")
		args = append(args, criteria.ParticipationChannel)
	}
	where, args = appendProfileClaimFilters(where, args, criteria)
	if exactRunID != "" {
		where, args = appendExactClaimOwnerPredicate(where, args, criteria)
	} else {
		where, args = appendClaimOwnerPredicate(where, args, criteria)
	}
	args = append(args, preferredCapabilityArgs(criteria.RequiredCapabilities)...)

	// dynamic-sql: claim filters, capability cardinality, ownership, and priority ordering alter query structure.
	query := `SELECT tr.id
		FROM task_runs tr
		JOIN tasks t ON t.id = tr.task_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + taskPriorityValueSQL + ` DESC,
		         ` + preferredCapabilityOrder(criteria.RequiredCapabilities) + `
		         tr.queued_at ASC,
		         tr.id ASC
		LIMIT 1`

	var runID string
	if err := exec.QueryRowContext(ctx, query, args...).Scan(&runID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("store: select claimable task run: %w", err)
	}
	return runID, nil
}

func baseClaimPredicates(criteria taskpkg.ClaimCriteria) ([]string, []any) {
	where := []string{
		"tr.status = ?",
		"COALESCE(t.paused, 0) = 0",
		`NOT EXISTS (SELECT 1 FROM scheduler_pause sp WHERE sp.id = 1 AND sp.paused = 1)`,
		`NOT EXISTS (
			WITH RECURSIVE ancestors(id, parent_task_id, paused) AS (
				SELECT parent.id, parent.parent_task_id, parent.paused
				  FROM tasks parent
				 WHERE parent.id = t.parent_task_id
				UNION ALL
				SELECT parent.id, parent.parent_task_id, parent.paused
				  FROM tasks parent
				  JOIN ancestors a ON parent.id = a.parent_task_id
			)
			SELECT 1 FROM ancestors WHERE COALESCE(paused, 0) = 1
		)`,
		"t.status NOT IN (?, ?, ?, ?)",
		unresolvedTaskDependencyClaimPredicate,
		`NOT EXISTS (
			SELECT 1
			  FROM task_blocks b
			 WHERE b.task_id = t.id
			   AND b.cleared_at IS NULL
			   AND (b.expires_at IS NULL OR b.expires_at > ?)
		)`,
		taskPriorityValueSQL + " >= ?",
		`NOT EXISTS (
			SELECT 1
			  FROM task_run_required_capabilities req
			 WHERE req.run_id = tr.id` + missingCapabilityPredicate(criteria.RequiredCapabilities) + `
		)`,
	}
	args := []any{
		taskpkg.TaskRunStatusQueued.String(),
		string(taskpkg.TaskStatusDraft),
		string(taskpkg.TaskStatusBlocked),
		string(taskpkg.TaskStatusNeedsAttention),
		string(taskpkg.TaskStatusCanceled),
	}
	args = append(args, unresolvedTaskDependencyClaimArgs()...)
	args = append(args, store.FormatTimestamp(criteria.Now), criteria.PriorityMin)
	args = append(args, missingCapabilityArgs(criteria.RequiredCapabilities)...)
	return where, args
}

func appendClaimRunKindPredicate(
	where []string,
	args []any,
	kind taskpkg.RunKind,
) ([]string, []any) {
	normalized := kind.Normalize()
	if normalized == taskpkg.RunKindUnknown {
		return append(where, "tr.run_kind IN (?, ?)"), append(
			args,
			taskpkg.RunKindWorker.String(),
			taskpkg.RunKindCoordinator.String(),
		)
	}
	return append(where, "tr.run_kind = ?"), append(args, normalized.String())
}

func (g *TaskRunRepo) selectClaimableNetworkWakeRunID(
	ctx context.Context,
	exec taskSQLExecutor,
	criteria taskpkg.ClaimCriteria,
) (string, error) {
	where := []string{
		"run_kind = ?",
		"status = ?",
		"workspace_id = ?",
		"network_target_session_id = ?",
		`NOT EXISTS (SELECT 1 FROM scheduler_pause sp WHERE sp.id = 1 AND sp.paused = 1)`,
	}
	args := []any{
		taskpkg.RunKindNetworkWake.String(),
		taskpkg.TaskRunStatusQueued.String(),
		strings.TrimSpace(criteria.WorkspaceID),
		strings.TrimSpace(criteria.TargetSessionID),
	}
	if runID := strings.TrimSpace(criteria.RunID); runID != "" {
		where = append(where, "id = ?")
		args = append(args, runID)
	}
	query := `SELECT id FROM task_runs WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY queued_at ASC, id ASC LIMIT 1`
	var runID string
	if err := exec.QueryRowContext(ctx, query, args...).Scan(&runID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("store: select claimable network wake run: %w", err)
	}
	return runID, nil
}

func appendProfileClaimFilters(
	where []string,
	args []any,
	criteria taskpkg.ClaimCriteria,
) ([]string, []any) {
	where, args = appendProfileRequiredCapabilityFilter(
		where,
		args,
		profileRoleWorker,
		criteria.RequiredCapabilities,
	)
	where, args = appendProfileRequiredCapabilityFilter(
		where,
		args,
		profileRoleParticipant,
		criteria.RequiredCapabilities,
	)
	agentName := strings.TrimSpace(criteria.AgentName)
	where = append(where, `NOT EXISTS (
			SELECT 1
			  FROM task_execution_profiles tep
			 WHERE tep.task_id = t.id
			   AND COALESCE(tep.worker_agent_name, '') <> ''
			   AND tep.worker_agent_name <> ?
		)`)
	args = append(args, agentName)
	where, args = appendProfileAllowedAgentFilter(where, args, profileRoleWorker, agentName)
	return appendProfileAllowedAgentFilter(where, args, profileRoleParticipant, agentName)
}

func appendProfileRequiredCapabilityFilter(
	where []string,
	args []any,
	role string,
	capabilities []string,
) ([]string, []any) {
	where = append(where, `NOT EXISTS (
			SELECT 1
			  FROM task_profile_capabilities pc
			 WHERE pc.task_id = t.id
			   AND pc.role = ?
			   AND pc.preference = ?`+missingCapabilityPredicateFor("pc.capability_id", capabilities)+`
		)`)
	args = append(args, role, profilePreferenceRequired)
	args = append(args, missingCapabilityArgs(capabilities)...)
	return where, args
}

func appendProfileAllowedAgentFilter(
	where []string,
	args []any,
	role string,
	agentName string,
) ([]string, []any) {
	where = append(where, `(NOT EXISTS (
			SELECT 1
			  FROM task_profile_agents pa_all
			 WHERE pa_all.task_id = t.id
			   AND pa_all.role = ?
			   AND pa_all.preference = ?
		) OR EXISTS (
			SELECT 1
			  FROM task_profile_agents pa_match
			 WHERE pa_match.task_id = t.id
			   AND pa_match.role = ?
			   AND pa_match.preference = ?
			   AND pa_match.agent_name = ?
		))`)
	args = append(args, role, profilePreferenceAllowed, role, profilePreferenceAllowed, agentName)
	return where, args
}

func appendClaimOwnerPredicate(
	where []string,
	args []any,
	criteria taskpkg.ClaimCriteria,
) ([]string, []any) {
	clauses := []string{"COALESCE(t.owner_kind, '') = ''"}
	if agentName := strings.TrimSpace(criteria.AgentName); agentName != "" {
		clauses = append(clauses, "(t.owner_kind = ? AND t.owner_ref = ?)")
		args = append(args, string(taskpkg.OwnerKindPool), agentName)
	}
	if sessionID := strings.TrimSpace(criteria.ClaimerSessionID); sessionID != "" {
		clauses = append(clauses, "(t.owner_kind = ? AND t.owner_ref = ?)")
		args = append(args, string(taskpkg.OwnerKindAgentSession), sessionID)
	}
	where = append(where, "("+strings.Join(clauses, " OR ")+")")
	return where, args
}

func appendExactClaimOwnerPredicate(
	where []string,
	args []any,
	criteria taskpkg.ClaimCriteria,
) ([]string, []any) {
	clauses := []string{
		"COALESCE(t.owner_kind, '') = ''",
		"t.owner_kind IN (?, ?, ?, ?)",
	}
	args = append(
		args,
		string(taskpkg.OwnerKindHuman),
		string(taskpkg.OwnerKindAutomation),
		string(taskpkg.OwnerKindExtension),
		string(taskpkg.OwnerKindNetworkPeer),
	)
	if agentName := strings.TrimSpace(criteria.AgentName); agentName != "" {
		clauses = append(clauses, "(t.owner_kind = ? AND t.owner_ref = ?)")
		args = append(args, string(taskpkg.OwnerKindPool), agentName)
	}
	if sessionID := strings.TrimSpace(criteria.ClaimerSessionID); sessionID != "" {
		clauses = append(clauses, "(t.owner_kind = ? AND t.owner_ref = ?)")
		args = append(args, string(taskpkg.OwnerKindAgentSession), sessionID)
	}
	if criteria.ClaimedBy != nil && criteria.ClaimedBy.Kind.Normalize() == taskpkg.ActorKindDaemon {
		clauses = append(clauses, "t.owner_kind = ?")
		args = append(args, string(taskpkg.OwnerKindAgentSession))
	}
	where = append(where, "("+strings.Join(clauses, " OR ")+")")
	return where, args
}
