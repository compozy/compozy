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
	"github.com/compozy/compozy/internal/store"
)

// Exposures returns durable records reconciled against current filesystem state.
func (m *ExposeManager) Exposures(ctx context.Context, skill *Skill) ([]ExposureState, error) {
	if ctx == nil {
		return nil, errors.New("skills: exposure context is required")
	}
	if m == nil {
		return nil, errors.New("skills: expose manager is required")
	}
	exposureMutationMu.Lock()
	defer exposureMutationMu.Unlock()

	if skill != nil && skill.Source == SourceBundled {
		return []ExposureState{}, nil
	}
	owner, _, err := m.prepareSkillIdentity(skill)
	if err != nil {
		return nil, err
	}
	records, err := m.store.ListSkillExposuresByOwner(ctx, skill.Meta.Name, owner.scope, owner.workspaceID)
	if err != nil {
		return nil, fmt.Errorf("skills: list exposures for %q: %w", skill.Meta.Name, err)
	}
	canonicalDir := strings.TrimSpace(skill.Dir)
	if resolved, resolveErr := m.fs.EvalSymlinks(skill.Dir); resolveErr == nil {
		canonicalDir = resolved
	}
	states := make([]ExposureState, 0, len(records)+len(m.roots))
	recordedTargets := make(map[string]struct{}, len(records))
	for _, record := range records {
		state := m.reconcileRecord(record)
		states = append(states, state)
		recordedTargets[record.TargetSlug] = struct{}{}
		m.emitExposureDivergence(ctx, state)
	}
	for _, root := range m.roots {
		if root.Kind != compozyconfig.RootKindPreset ||
			!sameExposureScope(root.ResourceScope.Normalize(), owner.resource) {
			continue
		}
		if _, exists := recordedTargets[root.SourceSlug]; exists {
			continue
		}
		linkPath, resolveErr := resolveExposeDest(root.Dir, skill.Meta.Name)
		if resolveErr != nil {
			continue
		}
		if _, statErr := m.fs.Lstat(linkPath); errors.Is(statErr, os.ErrNotExist) {
			continue
		} else if statErr != nil {
			return nil, fmt.Errorf("skills: inspect unowned exposure path %q: %w", linkPath, statErr)
		}
		state := ExposureState{Record: ExposureRecord{
			SkillName: skill.Meta.Name, CanonicalDir: canonicalDir, TargetSlug: root.SourceSlug,
			LinkPath: linkPath, OwnerScope: owner.scope, WorkspaceID: owner.workspaceID,
		}, Status: ExposureForeignConflict}
		states = append(states, state)
		m.emitExposureDivergence(ctx, state)
	}
	slices.SortFunc(states, func(left ExposureState, right ExposureState) int {
		return strings.Compare(left.Record.TargetSlug, right.Record.TargetSlug)
	})
	return states, nil
}

// Reconcile is the retryable inspection contract used by lifecycle callers.
func (m *ExposeManager) Reconcile(ctx context.Context, skill *Skill) ([]ExposureState, error) {
	return m.Exposures(ctx, skill)
}

func (m *ExposeManager) reconcileRecord(record ExposureRecord) ExposureState {
	state := ExposureState{Record: record, Status: ExposureMissing}
	info, err := m.fs.Lstat(record.LinkPath)
	if errors.Is(err, os.ErrNotExist) {
		return state
	}
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		state.Status = ExposureForeignConflict
		return state
	}
	actualTarget, err := m.fs.Readlink(record.LinkPath)
	if err != nil || actualTarget != record.LinkTarget {
		state.Status = ExposureForeignConflict
		return state
	}
	resolved, err := m.fs.EvalSymlinks(record.LinkPath)
	if err != nil {
		state.Status = ExposureBroken
		return state
	}
	canonicalResolved, err := filepath.Abs(filepath.Clean(resolved))
	if err != nil || canonicalResolved != filepath.Clean(record.CanonicalDir) {
		state.Status = ExposureForeignConflict
		return state
	}
	state.Status = ExposureHealthy
	return state
}

func exposureRecordForTarget(records []store.SkillExposureRecord, target string) (ExposureRecord, bool) {
	for _, record := range records {
		if record.TargetSlug == target {
			return record, true
		}
	}
	return ExposureRecord{}, false
}
