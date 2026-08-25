package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
)

func (m *ExposeManager) planMissingExposureDirectories(root string) ([]string, error) {
	absolute, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("skills: make exposure root %q absolute: %w", root, err)
	}
	missing := make([]string, 0, 3)
	current := absolute
	for {
		info, statErr := m.fs.Lstat(current)
		if statErr == nil {
			if !info.IsDir() {
				return nil, newExposureError(exposureErrorParams{
					code: ExposureCodeNameConflict, path: current,
					message: fmt.Sprintf("expose target root component %q is not a directory", current),
				})
			}
			break
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("skills: inspect expose target root %q: %w", current, statErr)
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			return nil, fmt.Errorf("skills: no existing parent for expose target root %q", root)
		}
		current = parent
	}
	slices.Reverse(missing)
	return missing, nil
}

func (m *ExposeManager) createExposureDirectories(planned []string) ([]string, error) {
	created := make([]string, 0, len(planned))
	for _, directory := range planned {
		if err := m.fs.Mkdir(directory, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) {
				info, statErr := m.fs.Stat(directory)
				if statErr == nil && info.IsDir() {
					continue
				}
			}
			return created, fmt.Errorf("skills: create exposure directory %q: %w", directory, err)
		}
		created = append(created, directory)
	}
	return created, nil
}

func (m *ExposeManager) rollbackExposureCommit(ctx context.Context, commit exposeCommit) error {
	failures := make([]error, 0, 3)
	linkRemovalFailed := false
	if commit.createdLink {
		if err := m.removeProvenExposureLink(commit.record); err != nil {
			failures = append(failures, err)
			linkRemovalFailed = true
		}
	}
	// Keep the durable ownership proof whenever the link could not be removed.
	// A later reconcile/cleanup pass can then retry without adopting filesystem
	// state from inference.
	if commit.inserted && !linkRemovalFailed {
		if err := m.store.DeleteSkillExposure(ctx, commit.record.ID); err != nil {
			failures = append(failures, fmt.Errorf("skills: roll back exposure record %d: %w", commit.record.ID, err))
		}
	}
	if err := m.removeEmptyExposureDirectories(commit.createdDirs); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func (m *ExposeManager) rollbackCompletedExposures(
	ctx context.Context,
	skill *Skill,
	completed []exposeCommit,
	results []TargetResult,
) error {
	failures := make([]error, 0)
	for _, commit := range slices.Backward(completed) {
		rollbackErr := m.rollbackExposureCommit(ctx, commit)
		index := resultIndexForTarget(results, commit.preflight.target)
		if index >= 0 {
			results[index].OK = false
			results[index].Exposure = nil
			results[index].RolledBack = rollbackErr == nil
			results[index].Err = newExposureError(exposureErrorParams{
				code: ExposureCodeRolledBack, target: commit.preflight.target, path: commit.preflight.linkPath,
				message: fmt.Sprintf("exposure to %q was rolled back", commit.preflight.target),
			})
			results[index].CleanupErr = rollbackErr
		}
		if rollbackErr != nil {
			failures = append(failures, rollbackErr)
			m.emitExposureCleanupFailure(
				ctx, skill, commit.preflight.target, commit.preflight.linkPath, rollbackErr,
			)
		}
	}
	return errors.Join(failures...)
}

func (m *ExposeManager) removeEmptyExposureDirectories(created []string) error {
	failures := make([]error, 0)
	for _, directory := range slices.Backward(created) {
		entries, err := m.fs.ReadDir(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("skills: inspect rollback directory %q: %w", directory, err))
			continue
		}
		if len(entries) != 0 {
			continue
		}
		if err := m.fs.Remove(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("skills: remove rollback directory %q: %w", directory, err))
		}
	}
	return errors.Join(failures...)
}

func (m *ExposeManager) removeProvenExposureLink(record ExposureRecord) error {
	if err := m.validateRecordedExposurePath(record); err != nil {
		return err
	}
	info, err := m.fs.Lstat(record.LinkPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("skills: inspect exposure link %q: %w", record.LinkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return newExposureError(exposureErrorParams{
			code: ExposureCodeForeignLink, target: record.TargetSlug, path: record.LinkPath,
			message: fmt.Sprintf("exposure path %q is no longer the recorded symlink", record.LinkPath),
		})
	}
	target, err := m.fs.Readlink(record.LinkPath)
	if err != nil {
		return fmt.Errorf("skills: read exposure link %q: %w", record.LinkPath, err)
	}
	if target != record.LinkTarget {
		return newExposureError(exposureErrorParams{
			code: ExposureCodeForeignLink, target: record.TargetSlug, path: record.LinkPath,
			message: fmt.Sprintf("exposure path %q points to foreign target %q", record.LinkPath, target),
		})
	}
	if err := m.fs.Remove(record.LinkPath); err != nil {
		return fmt.Errorf("skills: remove exposure link %q: %w", record.LinkPath, err)
	}
	return nil
}

func (m *ExposeManager) validateRecordedExposurePath(record ExposureRecord) error {
	owner, err := exposureOwnerFromRecord(record)
	if err != nil {
		return err
	}
	root, err := m.recordedTargetRoot(owner, record.TargetSlug)
	if err != nil {
		return err
	}
	expected, err := resolveExposeDest(root.Dir, record.SkillName)
	if err != nil {
		return err
	}
	if filepath.Clean(expected) != filepath.Clean(record.LinkPath) {
		return newExposureError(exposureErrorParams{
			code: ExposureCodeForeignLink, target: record.TargetSlug, path: record.LinkPath,
			message: fmt.Sprintf("recorded exposure path %q is outside its configured target root", record.LinkPath),
		})
	}
	canonicalRoot, err := m.fs.EvalSymlinks(root.Dir)
	if err != nil {
		return fmt.Errorf("skills: resolve exposure target root %q: %w", root.Dir, err)
	}
	canonicalParent, err := m.fs.EvalSymlinks(filepath.Dir(record.LinkPath))
	if err != nil {
		return fmt.Errorf("skills: resolve exposure link parent %q: %w", filepath.Dir(record.LinkPath), err)
	}
	if filepath.Clean(canonicalParent) != filepath.Clean(canonicalRoot) {
		return newExposureError(exposureErrorParams{
			code: ExposureCodeForeignLink, target: record.TargetSlug, path: record.LinkPath,
			message: fmt.Sprintf("exposure path %q resolves outside its configured target root", record.LinkPath),
		})
	}
	return nil
}

func (m *ExposeManager) recordedTargetRoot(
	owner exposureOwner,
	target string,
) (compozyconfig.SkillRootSpec, error) {
	for _, root := range m.knownRoots {
		if root.SourceSlug == strings.TrimSpace(target) && root.Kind == compozyconfig.RootKindPreset &&
			sameExposureScope(root.ResourceScope.Normalize(), owner.resource) {
			return root, nil
		}
	}
	return compozyconfig.SkillRootSpec{}, newExposureError(exposureErrorParams{
		code: ExposureCodeForeignLink, target: target,
		message: fmt.Sprintf("recorded exposure target %q is no longer an approved provider root", target),
	})
}

func exposureOwnerFromRecord(record ExposureRecord) (exposureOwner, error) {
	switch record.OwnerScope {
	case store.SkillExposureOwnerUser:
		return exposureOwner{
			scope:    record.OwnerScope,
			resource: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}, nil
	case store.SkillExposureOwnerWorkspace:
		workspaceID := strings.TrimSpace(record.WorkspaceID)
		if workspaceID == "" {
			return exposureOwner{}, newExposureError(exposureErrorParams{
				code: ExposureCodeForeignLink, target: record.TargetSlug, path: record.LinkPath,
				message: "recorded workspace exposure has no workspace id",
			})
		}
		return exposureOwner{
			scope:       record.OwnerScope,
			workspaceID: workspaceID,
			resource:    resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspaceID},
		}, nil
	default:
		return exposureOwner{}, newExposureError(exposureErrorParams{
			code: ExposureCodeForeignLink, target: record.TargetSlug, path: record.LinkPath,
			message: "recorded exposure has an unsupported owner scope",
		})
	}
}
