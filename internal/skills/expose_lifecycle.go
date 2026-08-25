package skills

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

// CleanupCanonicalDir removes every proven exposure before a canonical skill directory is deleted.
func (m *ExposeManager) CleanupCanonicalDir(ctx context.Context, canonicalDir string) error {
	if ctx == nil {
		return errors.New("skills: exposure cleanup context is required")
	}
	if m == nil || m.store == nil {
		return errors.New("skills: exposure repository is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	canonical, err := m.fs.EvalSymlinks(canonicalDir)
	if err != nil {
		return fmt.Errorf("skills: resolve canonical directory before exposure cleanup: %w", err)
	}
	records, err := m.store.ListSkillExposuresByCanonicalDir(ctx, canonical)
	if err != nil {
		return fmt.Errorf("skills: list exposure cleanup records for %q: %w", canonical, err)
	}
	for _, record := range records {
		state := m.reconcileRecord(record)
		if state.Status == ExposureForeignConflict {
			return m.skillRemoveBlocked(ctx, record, errors.New("exposure path is a foreign conflict"))
		}
		if state.Status != ExposureMissing {
			if err := m.removeProvenExposureLink(record); err != nil {
				return m.skillRemoveBlocked(ctx, record, err)
			}
		}
		if err := m.store.DeleteSkillExposure(ctx, record.ID); err != nil {
			return m.skillRemoveBlocked(ctx, record, err)
		}
		m.emitExposureEvent(ctx, exposureEventRemoved, record, ExposureMissing, nil)
	}
	return nil
}

// VerifyCanonicalDir ensures every persisted exposure remains healthy after an in-place update.
func (m *ExposeManager) VerifyCanonicalDir(ctx context.Context, canonicalDir string) error {
	if ctx == nil {
		return errors.New("skills: exposure verification context is required")
	}
	if m == nil || m.store == nil {
		return errors.New("skills: exposure repository is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	canonical, err := m.fs.EvalSymlinks(canonicalDir)
	if err != nil {
		return fmt.Errorf("skills: resolve canonical directory after update: %w", err)
	}
	records, err := m.store.ListSkillExposuresByCanonicalDir(ctx, canonical)
	if err != nil {
		return fmt.Errorf("skills: list exposure records after update: %w", err)
	}
	for _, record := range records {
		state := m.reconcileRecord(record)
		if state.Status == ExposureHealthy {
			continue
		}
		m.emitExposureDivergence(ctx, state)
		return fmt.Errorf(
			"skills: exposure %q for %q is %s after update",
			record.LinkPath,
			filepath.Clean(canonical),
			state.Status,
		)
	}
	return nil
}

func (m *ExposeManager) skillRemoveBlocked(
	ctx context.Context,
	record ExposureRecord,
	cause error,
) error {
	err := &ExposureError{
		Code: ExposureCodeSkillRemoveBlocked, Target: record.TargetSlug, Path: record.LinkPath,
		Message:   fmt.Sprintf("skill removal blocked by exposure link %q", record.LinkPath),
		Retryable: true, Cause: cause,
	}
	m.emitExposureEvent(ctx, exposureEventCleanupFailure, record, m.reconcileRecord(record).Status, err)
	return err
}
