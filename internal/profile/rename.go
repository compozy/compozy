package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) Rename(
	ctx context.Context,
	name string,
	opts RenameOptions,
) (RenameResult, error) {
	name = strings.TrimSpace(name)
	newName, err := normalizeName(opts.NewName)
	if err != nil {
		return RenameResult{}, err
	}
	if err := opts.Repos.Validate(); err != nil {
		return RenameResult{}, err
	}
	if strings.TrimSpace(opts.PlanRevision) == "" {
		return RenameResult{}, fmt.Errorf("%w: rename plan revision is required", ErrInvalidInput)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()

	opID, err := m.newOperationID()
	if err != nil {
		return RenameResult{}, err
	}
	var plan RenamePlan
	var renamed Profile
	err = m.write(ctx, "rename profile", func(exec globaldb.ProfileWriteExecutor) error {
		profile, err := getProfileByName(ctx, exec, name)
		if err != nil {
			return err
		}
		if err := rejectPermanent(profile, "rename"); err != nil {
			return err
		}
		if err := ensureAvailable(ctx, exec, profile, false); err != nil {
			return err
		}
		if err := ensureNameAvailable(ctx, exec, newName, profile.ID); err != nil {
			return err
		}
		renamed = profile
		renamed.Name = newName
		plan, err = m.renamePlan(ctx, exec, profile, newName)
		if err != nil {
			return err
		}
		if plan.Revision != strings.TrimSpace(opts.PlanRevision) {
			return stalePlanError("rename")
		}
		if _, err := exec.ExecContext(ctx, `UPDATE profiles SET name = ? WHERE id = ?`, newName, profile.ID); err != nil {
			return mapNameConstraint(err, newName)
		}
		oldPrefix := "vault:profiles/" + profile.Name + "/"
		newPrefix := "vault:profiles/" + newName + "/"
		if _, err := exec.ExecContext(
			ctx,
			`UPDATE extension_env_bindings
			 SET secret_ref = REPLACE(secret_ref, ?, ?), updated_at = ?
			 WHERE secret_ref LIKE ?`,
			oldPrefix, newPrefix, formatTimestamp(m.now()), oldPrefix+"%",
		); err != nil {
			return fmt.Errorf("profile: rewrite extension vault refs: %w", err)
		}
		return m.insertOperation(
			ctx, exec, opID, "rename", profile.ID, profile.Name, newName, plan.Revision,
			[]lifecycleStep{{
				Seq: 0, Action: "rename_profile", PathOld: m.profileDir(profile.Name), PathNew: m.profileDir(newName),
			}},
		)
	})
	if err != nil {
		if errors.Is(err, ErrPlanStale) {
			m.recordEvent("profile.plan_stale", renamed, opID)
		}
		return RenameResult{}, err
	}
	if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
		return RenameResult{}, err
	}
	result := RenameResult{DormantPlacements: plan.DormantPlacements}
	result.RepoResults = m.applyRepoRenames(plan.RepoCandidates, name, newName, opts.Repos)
	m.recordEvent("profile.renamed", renamed, opID)
	return result, nil
}

func (m *Manager) applyRepoRenames(
	candidates []RepoFolderRef,
	oldName, newName string,
	choice RepoChoice,
) []RepoRenameOutcome {
	selected := make(map[string]struct{}, len(choice.WorkspaceIDs))
	for _, workspaceID := range choice.WorkspaceIDs {
		selected[strings.TrimSpace(workspaceID)] = struct{}{}
	}
	results := make([]RepoRenameOutcome, 0, len(candidates))
	for _, candidate := range candidates {
		_, explicitlySelected := selected[candidate.WorkspaceID]
		if choice.None || (!choice.All && !explicitlySelected) {
			results = append(results, RepoRenameOutcome{
				WorkspaceID: candidate.WorkspaceID, Reason: "not_selected",
			})
			continue
		}
		newPath := filepath.Join(filepath.Dir(candidate.Path), newName)
		if err := os.Rename(candidate.Path, newPath); err != nil {
			m.logger.Warn("profile repository folder rename deferred",
				"workspace_id", candidate.WorkspaceID,
				"old_name", oldName,
				"new_name", newName,
				"error", err,
			)
			results = append(results, RepoRenameOutcome{
				WorkspaceID: candidate.WorkspaceID, Reason: err.Error(),
			})
			continue
		}
		results = append(results, RepoRenameOutcome{WorkspaceID: candidate.WorkspaceID, Renamed: true})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].WorkspaceID < results[j].WorkspaceID })
	return results
}

func stalePlanError(operation string) error {
	return domainError(
		"profile_plan_stale",
		fmt.Sprintf("the %s plan is stale", operation),
		"fetch the plan again and confirm it",
		ErrPlanStale,
	)
}
