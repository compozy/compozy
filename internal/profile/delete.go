package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) Delete(
	ctx context.Context,
	name, planRevision string,
) (DeleteResult, error) {
	if err := requireContext(ctx, "delete"); err != nil {
		return DeleteResult{}, err
	}
	name, planRevision = strings.TrimSpace(name), strings.TrimSpace(planRevision)
	if planRevision == "" {
		return DeleteResult{}, fmt.Errorf("%w: delete plan revision is required", ErrInvalidInput)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()

	opID, err := m.newOperationID()
	if err != nil {
		return DeleteResult{}, err
	}
	var result DeleteResult
	var deleted Profile
	err = m.write(ctx, "delete profile", func(exec globaldb.ProfileWriteExecutor) error {
		profile, err := getProfileByName(ctx, exec, name)
		if err != nil {
			return err
		}
		if err := rejectPermanent(profile, "delete"); err != nil {
			return err
		}
		if err := ensureAvailable(ctx, exec, profile, false); err != nil {
			return err
		}
		deleted = profile
		plan, err := m.deletePlan(ctx, exec, profile)
		if err != nil {
			return err
		}
		if err := m.validateDeletePlan(ctx, exec, profile, plan, name, planRevision); err != nil {
			return err
		}
		if err := m.insertOperation(
			ctx, exec, opID, "delete", profile.ID, profile.Name, "", plan.Revision,
			[]lifecycleStep{
				{Seq: 0, Action: stepRemoveProfile, PathOld: m.profileDir(profile.Name)},
				{Seq: 1, Action: stepRemoveDesktopPartitions},
			},
		); err != nil {
			return err
		}
		if err := deleteProfileOwnedRows(ctx, exec, profile); err != nil {
			return err
		}
		result = DeleteResult{Removed: plan.Removed, SweptSelections: plan.SelectionsToSweep}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPlanStale) {
			m.recordEvent(eventspkg.ProfilePlanStale, deleted, opID)
		}
		return DeleteResult{}, err
	}
	if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
		return DeleteResult{}, err
	}
	m.recordEvent(eventspkg.ProfileDeleted, deleted, opID)
	return result, nil
}

func (m *Manager) validateDeletePlan(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	profile Profile,
	plan DeletePlan,
	name string,
	planRevision string,
) error {
	if plan.Revision != planRevision {
		return stalePlanError("delete")
	}
	if err := validateNoDeliveryPermits(ctx, exec, profile.ID, name); err != nil {
		return err
	}
	if len(plan.ApprovalBlockers) > 0 {
		return approvalsPendingError(plan.ApprovalBlockers)
	}
	counts, err := m.profileCounts(ctx, exec, profile.ID)
	if err != nil {
		return err
	}
	if counts.workItems == 0 {
		return nil
	}
	return domainError(
		"profile_owns_work",
		fmt.Sprintf("profile %q owns %d work items", name, counts.workItems),
		"archive the profile or remove its work first",
		ErrOwnsWork,
	)
}

func deleteProfileOwnedRows(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	profile Profile,
) error {
	for _, cleanup := range []struct {
		label     string
		statement string
	}{
		{label: "command palette usage", statement: `DELETE FROM cmd_palette_usage WHERE profile_lens_id = ?`},
		{label: "command palette query hits", statement: `DELETE FROM cmd_palette_query_hits WHERE profile_lens_id = ?`},
		{label: "command palette pins", statement: `DELETE FROM cmd_palette_pins WHERE profile_lens_id = ?`},
		{label: "tool approval history", statement: `DELETE FROM tool_approval_pending WHERE profile_id = ?`},
		{label: "event summaries", statement: `DELETE FROM event_summaries WHERE profile_id = ?`},
		{label: "credential requirements", statement: `DELETE FROM profile_credential_requirements WHERE profile_id = ?`},
		{label: "profile selections", statement: `DELETE FROM profile_selections WHERE profile_id = ?`},
		{label: "operator caller bindings", statement: `DELETE FROM operator_caller_sessions WHERE profile_id = ?`},
	} {
		if _, err := exec.ExecContext(ctx, cleanup.statement, profile.ID); err != nil {
			return fmt.Errorf("profile: remove %s for %q: %w", cleanup.label, profile.Name, err)
		}
	}
	if err := deleteMCPAuthProfileRecords(ctx, exec, profile.Name); err != nil {
		return err
	}
	if _, err := exec.ExecContext(
		ctx, `DELETE FROM vault_secrets WHERE ref LIKE ? OR ref LIKE ?`,
		profileVaultRefPrefix(profile.Name)+"%", profileMCPVaultRefPrefix(profile.Name)+"%",
	); err != nil {
		return fmt.Errorf("profile: remove credential overrides for %q: %w", profile.Name, err)
	}
	if _, err := exec.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, profile.ID); err != nil {
		return fmt.Errorf("profile: delete %q: %w", profile.Name, err)
	}
	return nil
}
