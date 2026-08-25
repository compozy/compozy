package skills

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Unexpose removes only links proven by durable ownership records.
func (m *ExposeManager) Unexpose(
	ctx context.Context,
	skill *Skill,
	targets []string,
) ([]TargetResult, error) {
	if ctx == nil {
		return nil, errors.New("skills: exposure context is required")
	}
	if m == nil {
		return nil, errors.New("skills: expose manager is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	results := exposureTargetResults(targets)
	if skill != nil && skill.Source == SourceBundled {
		err := newExposureError(
			ExposureCodeSkillNotExposable,
			"",
			"",
			"bundled skills have no on-disk home; copy it with `compozy skill create` first",
			nil,
		)
		m.failAllExposureTargets(ctx, skill, results, err)
		return results, err
	}
	owner, _, err := m.prepareSkillIdentity(skill)
	if err != nil {
		m.failAllExposureTargets(ctx, skill, results, err)
		return results, err
	}
	if len(targets) == 0 {
		return results, newExposureError(
			ExposureCodeTargetInvalid, "", "", "at least one unexpose target is required", nil,
		)
	}
	failures := make([]error, 0)
	seen := make(map[string]struct{}, len(targets))
	for index, rawTarget := range targets {
		target := strings.TrimSpace(rawTarget)
		if _, duplicate := seen[target]; duplicate {
			err := newExposureError(
				ExposureCodeTargetInvalid, target, "", fmt.Sprintf("duplicate unexpose target %q", target), nil,
			)
			results[index].Err = err
			failures = append(failures, err)
			m.emitExposureFailure(ctx, skill, target, "", err)
			continue
		}
		seen[target] = struct{}{}
		record, getErr := m.store.GetSkillExposureByOwnerTarget(
			ctx, skill.Meta.Name, owner.scope, owner.workspaceID, target,
		)
		if getErr != nil && !errors.Is(getErr, sql.ErrNoRows) {
			err := fmt.Errorf("skills: get exposure before unexpose: %w", getErr)
			results[index].Err = err
			failures = append(failures, err)
			m.emitExposureFailure(ctx, skill, target, "", err)
			continue
		}
		if errors.Is(getErr, sql.ErrNoRows) {
			err := m.unexposeWithoutRecord(owner, skill, target)
			if err != nil {
				results[index].Err = err
				failures = append(failures, err)
				m.emitExposureFailure(ctx, skill, target, exposureErrorPath(err), err)
				continue
			}
			results[index].OK = true
			continue
		}
		state := m.reconcileRecord(record)
		results[index].Exposure = &state
		if state.Status == ExposureForeignConflict {
			err := newExposureError(
				ExposureCodeForeignLink,
				target,
				record.LinkPath,
				fmt.Sprintf("exposure path %q is not the recorded CompozyOS link", record.LinkPath),
				nil,
			)
			results[index].Err = err
			failures = append(failures, err)
			m.emitExposureFailure(ctx, skill, target, record.LinkPath, err)
			continue
		}
		if state.Status != ExposureMissing {
			if err := m.removeProvenExposureLink(record); err != nil {
				results[index].Err = err
				results[index].Exposure = &state
				failures = append(failures, err)
				m.emitExposureFailure(ctx, skill, target, record.LinkPath, err)
				continue
			}
		}
		if err := m.store.DeleteSkillExposure(ctx, record.ID); err != nil {
			missing := ExposureState{Record: record, Status: ExposureMissing}
			wrapped := fmt.Errorf("skills: delete exposure record after link removal: %w", err)
			results[index].Err = wrapped
			results[index].Exposure = &missing
			failures = append(failures, wrapped)
			m.emitExposureFailure(ctx, skill, target, record.LinkPath, wrapped)
			continue
		}
		results[index].OK = true
		m.emitExposureEvent(ctx, exposureEventRemoved, record, ExposureMissing, nil)
	}
	return results, errors.Join(failures...)
}

func (m *ExposeManager) unexposeWithoutRecord(owner exposureOwner, skill *Skill, target string) error {
	root, err := m.targetRoot(owner, target)
	if err != nil {
		if exposureErrorCode(err) == ExposureCodeTargetDisabled && knownExposePreset(target) {
			return nil
		}
		return err
	}
	linkPath, err := resolveExposeDest(root.Dir, skill.Meta.Name)
	if err != nil {
		return err
	}
	info, err := m.fs.Lstat(linkPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("skills: inspect unowned exposure path %q: %w", linkPath, err)
	}
	actual := info.Name()
	if info.Mode()&os.ModeSymlink != 0 {
		if targetValue, readErr := m.fs.Readlink(linkPath); readErr == nil {
			actual = targetValue
		}
	}
	return newExposureError(
		ExposureCodeForeignLink,
		target,
		linkPath,
		fmt.Sprintf("exposure path %q has no ownership record and points to %q", linkPath, actual),
		nil,
	)
}

func exposureErrorPath(err error) string {
	var typed *ExposureError
	if errors.As(err, &typed) {
		return typed.Path
	}
	return ""
}
