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
)

type exposePreflight struct {
	target      string
	root        compozyconfig.SkillRootSpec
	linkPath    string
	linkTarget  string
	record      *ExposureRecord
	state       ExposureStatus
	missingDirs []string
}

type exposeCommit struct {
	preflight   exposePreflight
	record      ExposureRecord
	inserted    bool
	createdLink bool
	createdDirs []string
	state       ExposureState
}

// Expose creates provider-root links after every requested target passes preflight.
func (m *ExposeManager) Expose(
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
	owner, canonicalDir, err := m.prepareSkill(skill)
	if err != nil {
		m.failAllExposureTargets(ctx, skill, results, err)
		return results, &ExposureBatchError{Cause: err}
	}
	preflights, err := m.preflightExpose(ctx, skill, owner, canonicalDir, targets, results)
	if err != nil {
		m.markUnappliedExposureTargets(ctx, skill, results, err)
		return results, &ExposureBatchError{Cause: err}
	}

	completed := make([]exposeCommit, 0, len(preflights))
	for index, preflight := range preflights {
		if preflight.state == ExposureHealthy && preflight.record != nil {
			state := m.reconcileRecord(*preflight.record)
			results[index].OK = true
			results[index].Exposure = &state
			continue
		}
		commit, commitErr := m.commitExposure(ctx, skill, owner, canonicalDir, preflight)
		if commitErr == nil {
			completed = append(completed, commit)
			state := commit.state
			results[index].OK = true
			results[index].Exposure = &state
			m.emitExposureEvent(ctx, exposureEventCreated, commit.record, state.Status, nil)
			continue
		}

		results[index].Err = commitErr
		m.emitExposureFailure(ctx, skill, preflight.target, preflight.linkPath, commitErr)
		cleanupErr := m.rollbackExposureCommit(ctx, commit)
		if cleanupErr != nil {
			results[index].CleanupErr = cleanupErr
			m.emitExposureCleanupFailure(ctx, skill, preflight.target, preflight.linkPath, cleanupErr)
		}
		rollbackErr := m.rollbackCompletedExposures(ctx, skill, completed, results)
		joined := errors.Join(commitErr, cleanupErr, rollbackErr)
		rolledBack := len(completed) > 0 && cleanupErr == nil && rollbackErr == nil
		return results, &ExposureBatchError{RolledBack: rolledBack, Cause: joined}
	}
	return results, nil
}

func (m *ExposeManager) markUnappliedExposureTargets(
	ctx context.Context,
	skill *Skill,
	results []TargetResult,
	cause error,
) {
	var exposureErr *ExposureError
	targetFailure := errors.As(cause, &exposureErr) && strings.TrimSpace(exposureErr.Target) != ""
	for index := range results {
		if results[index].OK || results[index].Err != nil {
			continue
		}
		if !targetFailure {
			results[index].Err = cause
			m.emitExposureFailure(ctx, skill, results[index].Target, exposureErrorPath(cause), cause)
			continue
		}
		results[index].Err = newExposureError(exposureErrorParams{
			code: ExposureCodeNotApplied, target: results[index].Target,
			message: fmt.Sprintf("exposure was not applied because target %q failed preflight", exposureErr.Target),
		})
	}
}

func (m *ExposeManager) preflightExpose(
	ctx context.Context,
	skill *Skill,
	owner exposureOwner,
	canonicalDir string,
	targets []string,
	results []TargetResult,
) ([]exposePreflight, error) {
	if len(targets) == 0 {
		return nil, newExposureError(exposureErrorParams{
			code: ExposureCodeTargetInvalid, message: "at least one expose target is required",
		})
	}
	records, err := m.store.ListSkillExposuresByOwner(ctx, skill.Meta.Name, owner.scope, owner.workspaceID)
	if err != nil {
		return nil, fmt.Errorf("skills: list exposure records before expose: %w", err)
	}
	seenTargets := make(map[string]struct{}, len(targets))
	preflights := make([]exposePreflight, 0, len(targets))
	for index, rawTarget := range targets {
		target := strings.TrimSpace(rawTarget)
		if _, duplicate := seenTargets[target]; duplicate {
			err := newExposureError(exposureErrorParams{
				code: ExposureCodeTargetInvalid, target: target,
				message: fmt.Sprintf("duplicate expose target %q", target),
			})
			results[index].Err = err
			m.emitExposureFailure(ctx, skill, target, "", err)
			return nil, err
		}
		seenTargets[target] = struct{}{}
		root, rootErr := m.targetRoot(owner, target)
		if rootErr != nil {
			results[index].Err = rootErr
			m.emitExposureFailure(ctx, skill, target, "", rootErr)
			return nil, rootErr
		}
		linkPath, resolveErr := resolveExposeDest(root.Dir, skill.Meta.Name)
		if resolveErr != nil {
			results[index].Err = resolveErr
			m.emitExposureFailure(ctx, skill, target, root.Dir, resolveErr)
			return nil, resolveErr
		}
		preflight := exposePreflight{
			target: target, root: root, linkPath: linkPath,
			linkTarget: relativeExposureTarget(linkPath, canonicalDir),
		}
		if record, ok := exposureRecordForTarget(records, target); ok {
			preflight.record = &record
			preflight.state = m.reconcileRecord(record).Status
			if filepath.Clean(record.CanonicalDir) != filepath.Clean(canonicalDir) || record.LinkPath != linkPath {
				err := newExposureError(exposureErrorParams{
					code: ExposureCodeNameConflict, target: target, path: linkPath,
					message: fmt.Sprintf("expose target %q is owned by another skill location", target),
				})
				results[index].Err = err
				m.emitExposureFailure(ctx, skill, target, linkPath, err)
				return nil, err
			}
			if preflight.state == ExposureForeignConflict {
				err := newExposureError(exposureErrorParams{
					code: ExposureCodeForeignLink, target: target, path: linkPath,
					message: fmt.Sprintf("exposure path %q is not the recorded CompozyOS link", linkPath),
				})
				results[index].Err = err
				m.emitExposureFailure(ctx, skill, target, linkPath, err)
				return nil, err
			}
			preflight.linkTarget = record.LinkTarget
		} else if _, statErr := m.fs.Lstat(linkPath); statErr == nil {
			err := newExposureError(exposureErrorParams{
				code: ExposureCodeNameConflict, target: target, path: linkPath,
				message: fmt.Sprintf("exposure path %q is occupied by a foreign entry", linkPath),
			})
			results[index].Err = err
			m.emitExposureFailure(ctx, skill, target, linkPath, err)
			return nil, err
		} else if !errors.Is(statErr, os.ErrNotExist) {
			err := newExposureError(exposureErrorParams{
				code: ExposureCodeNameConflict, target: target, path: linkPath,
				message: fmt.Sprintf("cannot inspect exposure path %q", linkPath), cause: statErr,
			})
			results[index].Err = err
			m.emitExposureFailure(ctx, skill, target, linkPath, err)
			return nil, err
		}
		missingDirs, planErr := m.planMissingExposureDirectories(root.Dir)
		if planErr != nil {
			results[index].Err = planErr
			m.emitExposureFailure(ctx, skill, target, root.Dir, planErr)
			return nil, planErr
		}
		preflight.missingDirs = missingDirs
		preflights = append(preflights, preflight)
	}
	return preflights, nil
}

func (m *ExposeManager) commitExposure(
	ctx context.Context,
	skill *Skill,
	owner exposureOwner,
	canonicalDir string,
	preflight exposePreflight,
) (exposeCommit, error) {
	commit := exposeCommit{preflight: preflight}
	createdDirs, err := m.createExposureDirectories(preflight.missingDirs)
	commit.createdDirs = createdDirs
	if err != nil {
		return commit, newExposureError(exposureErrorParams{
			code: ExposureCodeLinkUnsupported, target: preflight.target, path: preflight.root.Dir,
			message: fmt.Sprintf("cannot create expose target root %q", preflight.root.Dir), cause: err,
		})
	}
	if preflight.record != nil {
		commit.record = *preflight.record
		if preflight.state == ExposureBroken {
			if err := m.removeProvenExposureLink(*preflight.record); err != nil {
				return commit, err
			}
		}
	} else {
		record, createErr := m.store.CreateSkillExposure(ctx, ExposureRecord{
			SkillName: skill.Meta.Name, CanonicalDir: canonicalDir,
			TargetSlug: preflight.target, LinkPath: preflight.linkPath, LinkTarget: preflight.linkTarget,
			OwnerScope: owner.scope, WorkspaceID: owner.workspaceID,
		})
		if createErr != nil {
			return commit, fmt.Errorf("skills: insert exposure ownership record: %w", createErr)
		}
		commit.record = record
		commit.inserted = true
	}
	if err := m.fs.Symlink(commit.record.LinkTarget, commit.record.LinkPath); err != nil {
		return commit, newExposureError(exposureErrorParams{
			code: ExposureCodeLinkUnsupported, target: preflight.target, path: preflight.linkPath,
			message: fmt.Sprintf("cannot create skill link at %q; copying is not supported", preflight.linkPath), cause: err,
		})
	}
	commit.createdLink = true
	state := m.reconcileRecord(commit.record)
	if state.Status != ExposureHealthy {
		return commit, newExposureError(exposureErrorParams{
			code: ExposureCodeLinkUnsupported, target: preflight.target, path: preflight.linkPath,
			message: fmt.Sprintf("created skill link at %q did not reconcile as healthy", preflight.linkPath),
		})
	}
	commit.state = state
	return commit, nil
}

func exposureTargetResults(targets []string) []TargetResult {
	results := make([]TargetResult, len(targets))
	for index, target := range targets {
		results[index].Target = strings.TrimSpace(target)
	}
	return results
}

func relativeExposureTarget(linkPath string, canonicalDir string) string {
	relative, err := filepath.Rel(filepath.Dir(linkPath), canonicalDir)
	if err == nil && !filepath.IsAbs(relative) {
		return relative
	}
	return canonicalDir
}

func (m *ExposeManager) failAllExposureTargets(
	ctx context.Context,
	skill *Skill,
	results []TargetResult,
	err error,
) {
	for index := range results {
		results[index].Err = err
		m.emitExposureFailure(ctx, skill, results[index].Target, "", err)
	}
}

func resultIndexForTarget(results []TargetResult, target string) int {
	return slices.IndexFunc(results, func(result TargetResult) bool { return result.Target == target })
}
