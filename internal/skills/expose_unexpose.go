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
	exposureMutationMu.Lock()
	defer exposureMutationMu.Unlock()

	results := exposureTargetResults(targets)
	if skill != nil && skill.Source == SourceBundled {
		err := newExposureError(exposureErrorParams{
			code:    ExposureCodeSkillNotExposable,
			message: "bundled skills have no on-disk home; copy it with `compozy skill create` first",
		})
		m.failAllExposureTargets(ctx, skill, results, err)
		return results, err
	}
	owner, _, err := m.prepareSkillIdentity(skill)
	if err != nil {
		m.failAllExposureTargets(ctx, skill, results, err)
		return results, err
	}
	if len(targets) == 0 {
		return results, newExposureError(exposureErrorParams{
			code: ExposureCodeTargetInvalid, message: "at least one unexpose target is required",
		})
	}
	failures := make([]error, 0)
	seen := make(map[string]struct{}, len(targets))
	for index, rawTarget := range targets {
		target := strings.TrimSpace(rawTarget)
		if _, duplicate := seen[target]; duplicate {
			err := newExposureError(exposureErrorParams{
				code: ExposureCodeTargetInvalid, target: target,
				message: fmt.Sprintf("duplicate unexpose target %q", target),
			})
			results[index].Err = err
			failures = append(failures, err)
			m.emitExposureFailure(ctx, skill, target, "", err)
			continue
		}
		seen[target] = struct{}{}
		result, err := m.unexposeTarget(ctx, skill, owner, target)
		results[index] = result
		if err != nil {
			failures = append(failures, err)
		}
	}
	return results, errors.Join(failures...)
}

func (m *ExposeManager) unexposeTarget(
	ctx context.Context,
	skill *Skill,
	owner exposureOwner,
	target string,
) (TargetResult, error) {
	result := TargetResult{Target: target}
	record, err := m.store.GetSkillExposureByOwnerTarget(
		ctx, skill.Meta.Name, owner.scope, owner.workspaceID, target,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		wrapped := fmt.Errorf("skills: get exposure before unexpose: %w", err)
		result.Err = wrapped
		m.emitExposureFailure(ctx, skill, target, "", wrapped)
		return result, wrapped
	}
	if errors.Is(err, sql.ErrNoRows) {
		if err := m.unexposeWithoutRecord(owner, skill, target); err != nil {
			result.Err = err
			m.emitExposureFailure(ctx, skill, target, exposureErrorPath(err), err)
			return result, err
		}
		result.OK = true
		return result, nil
	}
	state := m.reconcileRecord(record)
	result.Exposure = &state
	if state.Status == ExposureForeignConflict {
		err := newExposureError(exposureErrorParams{
			code: ExposureCodeForeignLink, target: target, path: record.LinkPath,
			message: fmt.Sprintf("exposure path %q is not the recorded CompozyOS link", record.LinkPath),
		})
		result.Err = err
		m.emitExposureFailure(ctx, skill, target, record.LinkPath, err)
		return result, err
	}
	if state.Status != ExposureMissing {
		if err := m.removeProvenExposureLink(record); err != nil {
			result.Err = err
			m.emitExposureFailure(ctx, skill, target, record.LinkPath, err)
			return result, err
		}
	}
	if err := m.store.DeleteSkillExposure(ctx, record.ID); err != nil {
		wrapped := fmt.Errorf("skills: delete exposure record after link removal: %w", err)
		result.Err = wrapped
		result.Exposure = &ExposureState{Record: record, Status: ExposureMissing}
		m.emitExposureFailure(ctx, skill, target, record.LinkPath, wrapped)
		return result, wrapped
	}
	result.OK = true
	m.emitExposureEvent(ctx, exposureEventRemoved, record, ExposureMissing, nil)
	return result, nil
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
	return newExposureError(exposureErrorParams{
		code: ExposureCodeForeignLink, target: target, path: linkPath,
		message: fmt.Sprintf("exposure path %q has no ownership record and points to %q", linkPath, actual),
	})
}

func exposureErrorPath(err error) string {
	var typed *ExposureError
	if errors.As(err, &typed) {
		return typed.Path
	}
	return ""
}
