package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) Archive(
	ctx context.Context,
	name, planRevision string,
) (ArchiveResult, error) {
	name, planRevision = strings.TrimSpace(name), strings.TrimSpace(planRevision)
	if planRevision == "" {
		return ArchiveResult{}, fmt.Errorf("%w: archive plan revision is required", ErrInvalidInput)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()

	opID, err := m.newOperationID()
	if err != nil {
		return ArchiveResult{}, err
	}
	var result ArchiveResult
	var idempotent bool
	err = m.write(ctx, "archive profile", func(exec globaldb.ProfileWriteExecutor) error {
		profile, err := getProfileByName(ctx, exec, name)
		if err != nil {
			return err
		}
		if err := rejectPermanent(profile, "archive"); err != nil {
			return err
		}
		if profile.State == StateArchived {
			idempotent = true
			result.PausedAutomations, err = m.pausedAutomations(ctx, exec, profile.ID)
			return err
		}
		if err := ensureAvailable(ctx, exec, profile, false); err != nil {
			return err
		}
		plan, err := m.archivePlan(ctx, exec, profile)
		if err != nil {
			return err
		}
		var permits int
		if err := exec.QueryRowContext(
			ctx, `SELECT COUNT(*) FROM notification_delivery_permits WHERE profile_id = ?`, profile.ID,
		).Scan(&permits); err != nil {
			return fmt.Errorf("profile: count delivery permits: %w", err)
		}
		if permits > 0 {
			return domainError(
				"profile_deliveries_in_flight",
				fmt.Sprintf("profile %q has %d notification deliveries in flight", name, permits),
				"retry after the deliveries settle",
				ErrDeliveriesInFlight,
			)
		}
		if plan.Revision != planRevision {
			return stalePlanError("archive")
		}
		if len(plan.RunningSessions) > 0 {
			return domainError(
				"profile_sessions_running",
				fmt.Sprintf("profile %q has running sessions: %s", name, strings.Join(plan.RunningSessions, ", ")),
				"stop the sessions and retry",
				ErrSessionsRunning,
			)
		}
		if plan.LeasedRuns > 0 {
			return domainError(
				"profile_runs_leased", fmt.Sprintf("profile %q has %d leased runs", name, plan.LeasedRuns), "wait for the runs to settle", ErrOwnsWork,
			)
		}
		if len(plan.ApprovalBlockers) > 0 {
			return approvalsPendingError(plan.ApprovalBlockers)
		}
		now := formatTimestamp(m.now())
		if _, err := exec.ExecContext(
			ctx, `UPDATE profiles SET state = 'archived', archived_at = ? WHERE id = ?`, now, profile.ID,
		); err != nil {
			return fmt.Errorf("profile: archive %q: %w", name, err)
		}
		if _, err := exec.ExecContext(
			ctx,
			`UPDATE tasks SET paused = 1, paused_by = 'profile', paused_at = ?, paused_reason = 'profile archived'
			 WHERE profile_id = ? AND paused = 0`,
			now, profile.ID,
		); err != nil {
			return fmt.Errorf("profile: freeze queued work for %q: %w", name, err)
		}
		for _, automation := range plan.AutomationsToPause {
			kind, id, found := strings.Cut(automation, ":")
			if !found {
				return fmt.Errorf("profile: invalid automation identity %q", automation)
			}
			table := "automation_jobs"
			if kind == "trigger" {
				table = "automation_triggers"
			}
			if _, err := exec.ExecContext(ctx, `UPDATE `+table+` SET enabled = 0 WHERE id = ? AND profile_id = ?`, id, profile.ID); err != nil {
				return fmt.Errorf("profile: pause %s %q: %w", kind, id, err)
			}
		}
		if err := m.insertOperation(ctx, exec, opID, "archive", profile.ID, name, name, plan.Revision, nil); err != nil {
			return err
		}
		if err := m.insertAutomationAudit(ctx, exec, opID, plan.AutomationsToPause); err != nil {
			return err
		}
		result = ArchiveResult{PausedAutomations: plan.AutomationsToPause, FrozenQueuedRuns: plan.QueuedRunsToFreeze}
		return nil
	})
	if err != nil {
		return ArchiveResult{}, err
	}
	if !idempotent {
		if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
			return ArchiveResult{}, err
		}
	}
	return result, nil
}

func (m *Manager) Unarchive(ctx context.Context, name string) (UnarchiveResult, error) {
	name = strings.TrimSpace(name)
	m.opMu.Lock()
	defer m.opMu.Unlock()

	opID, err := m.newOperationID()
	if err != nil {
		return UnarchiveResult{}, err
	}
	var result UnarchiveResult
	var idempotent bool
	err = m.write(ctx, "unarchive profile", func(exec globaldb.ProfileWriteExecutor) error {
		profile, err := getProfileByName(ctx, exec, name)
		if err != nil {
			return err
		}
		if profile.State == StateActive {
			idempotent = true
			result.Profile = profile
			result.PausedAutomations, err = m.pausedAutomations(ctx, exec, profile.ID)
			return err
		}
		if err := ensureAvailable(ctx, exec, profile, false); err != nil {
			return err
		}
		paused, err := m.pausedAutomations(ctx, exec, profile.ID)
		if err != nil {
			return err
		}
		now := formatTimestamp(m.now())
		if _, err := exec.ExecContext(
			ctx, `UPDATE profiles SET state = 'active', archived_at = NULL WHERE id = ?`, profile.ID,
		); err != nil {
			return fmt.Errorf("profile: unarchive %q: %w", name, err)
		}
		if _, err := exec.ExecContext(
			ctx,
			`UPDATE tasks SET paused = 0, paused_by = '', paused_at = NULL, paused_reason = ''
			 WHERE profile_id = ? AND paused = 1 AND paused_by = 'profile' AND paused_reason = 'profile archived'`,
			profile.ID,
		); err != nil {
			return fmt.Errorf("profile: unfreeze queued work for %q: %w", name, err)
		}
		revision, err := fingerprint(struct {
			ProfileID, Name, At string
		}{profile.ID, profile.Name, now})
		if err != nil {
			return err
		}
		if err := m.insertOperation(ctx, exec, opID, "unarchive", profile.ID, name, name, revision, nil); err != nil {
			return err
		}
		profile.State, profile.ArchivedAt = StateActive, nil
		result = UnarchiveResult{Profile: profile, PausedAutomations: paused}
		return nil
	})
	if err != nil {
		return UnarchiveResult{}, err
	}
	if !idempotent {
		if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
			return UnarchiveResult{}, err
		}
	}
	return result, nil
}

func (m *Manager) insertAutomationAudit(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	opID string,
	automations []string,
) error {
	for seq, automation := range automations {
		if _, err := exec.ExecContext(
			ctx,
			`INSERT INTO profile_lifecycle_op_steps
			 (op_id, seq, action, status, updated_at) VALUES (?, ?, ?, 'done', ?)`,
			opID, seq, "paused_automation:"+automation, formatTimestamp(m.now()),
		); err != nil {
			return fmt.Errorf("profile: record paused automation %q: %w", automation, err)
		}
	}
	return nil
}

func (m *Manager) pausedAutomations(ctx context.Context, q queryer, profileID string) ([]string, error) {
	return stringColumn(ctx, q, `
		SELECT SUBSTR(s.action, LENGTH('paused_automation:') + 1)
		FROM profile_lifecycle_op_steps s
		JOIN profile_lifecycle_ops o ON o.id = s.op_id
		WHERE o.profile_id = ? AND o.kind = 'archive' AND s.action LIKE 'paused_automation:%'
		ORDER BY s.seq`, profileID)
}

func approvalsPendingError(ids []string) error {
	return domainError(
		"profile_approvals_pending",
		"profile has executable approvals: "+strings.Join(ids, ", "),
		"resolve or cancel the approvals and retry",
		ErrApprovalsPending,
	)
}
