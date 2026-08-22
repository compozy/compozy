package profile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/vault"
)

func (m *Manager) PrepareRename(ctx context.Context, name, newName string) (RenamePlan, error) {
	name = strings.TrimSpace(name)
	newName, err := normalizeName(newName)
	if err != nil {
		return RenamePlan{}, err
	}
	profile, err := getProfileByName(ctx, m.store.DB(), name)
	if err != nil {
		return RenamePlan{}, err
	}
	if err := rejectPermanent(profile, "rename"); err != nil {
		return RenamePlan{}, err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, false); err != nil {
		return RenamePlan{}, err
	}
	if err := ensureNameAvailable(ctx, m.store.DB(), newName, profile.ID); err != nil {
		return RenamePlan{}, err
	}
	return m.renamePlan(ctx, m.store.DB(), profile, newName)
}

func (m *Manager) renamePlan(
	ctx context.Context,
	q queryer,
	profile Profile,
	newName string,
) (RenamePlan, error) {
	plan := RenamePlan{RepoCandidates: make([]RepoFolderRef, 0), DormantPlacements: make([]PlacementRef, 0)}
	oldDir := m.profileDir(profile.Name)
	if _, err := os.Stat(oldDir); err == nil {
		plan.MachineFolders = []string{oldDir}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RenamePlan{}, fmt.Errorf("profile: inspect profile directory %q: %w", oldDir, err)
	}
	rows, err := q.QueryContext(ctx, `SELECT id, name, root_dir FROM workspaces ORDER BY id`)
	if err != nil {
		return RenamePlan{}, fmt.Errorf("profile: list repository candidates: %w", err)
	}
	for rows.Next() {
		var workspaceID, workspaceName, rootDir string
		if err := rows.Scan(&workspaceID, &workspaceName, &rootDir); err != nil {
			closeErr := rows.Close()
			return RenamePlan{}, errors.Join(fmt.Errorf("profile: scan repository candidate: %w", err), closeErr)
		}
		path := filepath.Join(rootDir, ".compozy", "profiles", profile.Name)
		if _, err := os.Stat(path); err == nil {
			plan.RepoCandidates = append(plan.RepoCandidates, RepoFolderRef{
				WorkspaceID: workspaceID, WorkspaceName: workspaceName, Path: path,
			})
		} else if !errors.Is(err, os.ErrNotExist) {
			closeErr := rows.Close()
			return RenamePlan{}, errors.Join(fmt.Errorf("profile: inspect repository profile folder %q: %w", path, err), closeErr)
		}
	}
	if err := rows.Err(); err != nil {
		closeErr := rows.Close()
		return RenamePlan{}, errors.Join(fmt.Errorf("profile: iterate repository candidates: %w", err), closeErr)
	}
	if err := rows.Close(); err != nil {
		return RenamePlan{}, fmt.Errorf("profile: close repository candidate rows: %w", err)
	}
	vaultRefRewrites, err := vault.ListProfileRefRewrites(ctx, q, profile.Name, newName)
	if err != nil {
		return RenamePlan{}, fmt.Errorf("profile: list vault ref rewrites: %w", err)
	}
	plan.VaultRefRewrites = len(vaultRefRewrites)
	if m.placements != nil {
		plan.DormantPlacements, err = m.placements.PlacementsForProfile(ctx, profile.Name)
		if err != nil {
			return RenamePlan{}, fmt.Errorf("profile: list extension placements: %w", err)
		}
		sort.Slice(plan.DormantPlacements, func(i, j int) bool {
			left, right := plan.DormantPlacements[i], plan.DormantPlacements[j]
			if left.Extension != right.Extension {
				return left.Extension < right.Extension
			}
			if left.Resource != right.Resource {
				return left.Resource < right.Resource
			}
			return left.ProfileName < right.ProfileName
		})
	}
	dirDigest, err := directoryDigest(oldDir)
	if err != nil {
		return RenamePlan{}, err
	}
	plan.Revision, err = fingerprint(struct {
		Profile Profile
		NewName string
		Dir     string
		Repos   []RepoFolderRef
		Dormant []PlacementRef
		Vault   int
	}{profile, newName, dirDigest, plan.RepoCandidates, plan.DormantPlacements, plan.VaultRefRewrites})
	if err != nil {
		return RenamePlan{}, err
	}
	return plan, nil
}

func (m *Manager) PrepareArchive(ctx context.Context, name string) (ArchivePlan, error) {
	profile, err := getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
	if err != nil {
		return ArchivePlan{}, err
	}
	if err := rejectPermanent(profile, "archive"); err != nil {
		return ArchivePlan{}, err
	}
	if profile.State == StateArchived {
		plan := ArchivePlan{}
		plan.Revision, err = fingerprint(profile)
		return plan, err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, false); err != nil {
		return ArchivePlan{}, err
	}
	return m.archivePlan(ctx, m.store.DB(), profile)
}

func (m *Manager) archivePlan(ctx context.Context, q queryer, profile Profile) (ArchivePlan, error) {
	var plan ArchivePlan
	var err error
	plan.RunningSessions, err = stringColumn(ctx, q,
		`SELECT COALESCE(NULLIF(name, ''), id) FROM sessions WHERE profile_id = ? AND state <> 'stopped' ORDER BY id`, profile.ID)
	if err != nil {
		return ArchivePlan{}, err
	}
	plan.ApprovalBlockers, err = executableApprovals(ctx, q, profile.ID)
	if err != nil {
		return ArchivePlan{}, err
	}
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM task_runs r JOIN tasks t ON t.id = r.task_id
		WHERE t.profile_id = ? AND r.status IN ('claimed','starting','running')`, profile.ID).Scan(&plan.LeasedRuns); err != nil {
		return ArchivePlan{}, fmt.Errorf("profile: count leased runs: %w", err)
	}
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM task_runs r JOIN tasks t ON t.id = r.task_id
		WHERE t.profile_id = ? AND r.status = 'queued' AND t.paused = 0`, profile.ID).Scan(&plan.QueuedRunsToFreeze); err != nil {
		return ArchivePlan{}, fmt.Errorf("profile: count queued runs to freeze: %w", err)
	}
	jobs, err := stringColumn(ctx, q, `SELECT 'job:' || id FROM automation_jobs WHERE profile_id = ? AND enabled = 1`, profile.ID)
	if err != nil {
		return ArchivePlan{}, err
	}
	triggers, err := stringColumn(ctx, q, `SELECT 'trigger:' || id FROM automation_triggers WHERE profile_id = ? AND enabled = 1`, profile.ID)
	if err != nil {
		return ArchivePlan{}, err
	}
	plan.AutomationsToPause = append(jobs, triggers...)
	sort.Strings(plan.AutomationsToPause)
	var permits int
	if err := q.QueryRowContext(
		ctx, `SELECT COUNT(*) FROM notification_delivery_permits WHERE profile_id = ?`, profile.ID,
	).Scan(&permits); err != nil {
		return ArchivePlan{}, fmt.Errorf("profile: count notification delivery permits: %w", err)
	}
	plan.Revision, err = fingerprint(struct {
		Profile Profile
		Plan    ArchivePlan
		Permits int
	}{profile, plan, permits})
	if err != nil {
		return ArchivePlan{}, err
	}
	return plan, nil
}

func (m *Manager) PrepareDelete(ctx context.Context, name string) (DeletePlan, error) {
	profile, err := getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
	if err != nil {
		return DeletePlan{}, err
	}
	if err := rejectPermanent(profile, "delete"); err != nil {
		return DeletePlan{}, err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, false); err != nil {
		return DeletePlan{}, err
	}
	return m.deletePlan(ctx, m.store.DB(), profile)
}

func (m *Manager) deletePlan(ctx context.Context, q queryer, profile Profile) (DeletePlan, error) {
	var plan DeletePlan
	var err error
	plan.Removed, err = profileFileRemovalSummary(m.profileDir(profile.Name))
	if err != nil {
		return DeletePlan{}, err
	}
	plan.ApprovalBlockers, err = executableApprovals(ctx, q, profile.ID)
	if err != nil {
		return DeletePlan{}, err
	}
	if err := q.QueryRowContext(
		ctx, `SELECT COUNT(*) FROM profile_selections WHERE profile_id = ?`, profile.ID,
	).Scan(&plan.SelectionsToSweep); err != nil {
		return DeletePlan{}, fmt.Errorf("profile: count selections to sweep: %w", err)
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM cmd_palette_usage WHERE profile_lens_id = ?`, profile.ID).Scan(&plan.Removed.PaletteUsage); err != nil {
		return DeletePlan{}, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM cmd_palette_query_hits WHERE profile_lens_id = ?`, profile.ID).Scan(&plan.Removed.PaletteQueryHits); err != nil {
		return DeletePlan{}, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM cmd_palette_pins WHERE profile_lens_id = ?`, profile.ID).Scan(&plan.Removed.PalettePins); err != nil {
		return DeletePlan{}, err
	}
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tool_approval_pending
		WHERE profile_id = ? AND approval_status <> 'pending'
		  AND COALESCE(execution_status, '') NOT IN ('dispatching','uncertain')`, profile.ID).Scan(&plan.Removed.TerminalApprovals); err != nil {
		return DeletePlan{}, err
	}
	plan.Removed.CredentialOverrides, err = countProfileCredentialRows(ctx, q, profile)
	if err != nil {
		return DeletePlan{}, err
	}
	plan.Revision, err = fingerprint(struct {
		Profile Profile
		Plan    DeletePlan
	}{profile, plan})
	if err != nil {
		return DeletePlan{}, err
	}
	return plan, nil
}

func executableApprovals(ctx context.Context, q queryer, profileID string) ([]string, error) {
	return stringColumn(ctx, q, `
		SELECT approval_id FROM tool_approval_pending
		WHERE profile_id = ? AND (
			approval_status = 'pending' OR execution_status IN ('dispatching','uncertain')
		) ORDER BY approval_id`, profileID)
}

func stringColumn(ctx context.Context, q queryer, statement string, args ...any) (values []string, err error) {
	rows, err := q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func directoryDigest(path string) (string, error) {
	entries := make([]string, 0)
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) && current == path {
				return fs.SkipAll
			}
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s:%d:%d", relative, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("profile: fingerprint directory %q: %w", path, err)
	}
	sort.Strings(entries)
	return fingerprint(entries)
}

func countFiles(path string) (int, error) {
	count := 0
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) && current == path {
				return fs.SkipAll
			}
			return walkErr
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("profile: count files in %q: %w", path, err)
	}
	return count, nil
}

func rejectPermanent(profile Profile, action string) error {
	if profile.ID != defaultProfileID {
		return nil
	}
	return domainError(
		"profile_permanent", fmt.Sprintf("the default profile cannot be %sd", action), "", ErrPermanent,
	)
}
