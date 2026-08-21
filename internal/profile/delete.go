package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) Delete(
	ctx context.Context,
	name, planRevision string,
) (DeleteResult, error) {
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
		if plan.Revision != planRevision {
			return stalePlanError("delete")
		}
		if len(plan.ApprovalBlockers) > 0 {
			return approvalsPendingError(plan.ApprovalBlockers)
		}
		counts, err := m.profileCounts(ctx, exec, profile.ID)
		if err != nil {
			return err
		}
		if counts.workItems > 0 {
			return domainError(
				"profile_owns_work",
				fmt.Sprintf("profile %q owns %d work items", name, counts.workItems),
				"archive the profile or remove its work first",
				ErrOwnsWork,
			)
		}
		if err := m.insertOperation(
			ctx, exec, opID, "delete", profile.ID, profile.Name, "", plan.Revision,
			[]lifecycleStep{{Seq: 0, Action: "remove_profile", PathOld: m.profileDir(profile.Name)}},
		); err != nil {
			return err
		}
		for _, statement := range []string{
			`DELETE FROM cmd_palette_usage WHERE profile_lens_id = ?`,
			`DELETE FROM cmd_palette_query_hits WHERE profile_lens_id = ?`,
			`DELETE FROM cmd_palette_pins WHERE profile_lens_id = ?`,
			`DELETE FROM tool_approval_pending WHERE profile_id = ?`,
			`DELETE FROM profile_credential_requirements WHERE profile_id = ?`,
			`DELETE FROM profile_selections WHERE profile_id = ?`,
		} {
			if _, err := exec.ExecContext(ctx, statement, profile.ID); err != nil {
				return fmt.Errorf("profile: remove support state for %q: %w", name, err)
			}
		}
		if _, err := exec.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, profile.ID); err != nil {
			return fmt.Errorf("profile: delete %q: %w", name, err)
		}
		result = DeleteResult{Removed: plan.Removed, SweptSelections: plan.SelectionsToSweep}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPlanStale) {
			m.recordEvent("profile.plan_stale", deleted, opID)
		}
		return DeleteResult{}, err
	}
	if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
		return DeleteResult{}, err
	}
	m.recordEvent("profile.deleted", deleted, opID)
	return result, nil
}
