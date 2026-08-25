package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SkillExposureRepository = (*SkillExposureRepo)(nil)

// CreateSkillExposure inserts one durable link-ownership record.
func (r *SkillExposureRepo) CreateSkillExposure(
	ctx context.Context,
	record store.SkillExposureRecord,
) (store.SkillExposureRecord, error) {
	if err := r.checkReady(ctx, "create skill exposure"); err != nil {
		return store.SkillExposureRecord{}, err
	}
	now := r.now().UTC()
	row, err := r.queries.CreateSkillExposure(ctx, sqlcgen.CreateSkillExposureParams{
		SkillName: record.SkillName, CanonicalDir: record.CanonicalDir,
		TargetSlug: record.TargetSlug, LinkPath: record.LinkPath, LinkTarget: record.LinkTarget,
		OwnerScope: string(record.OwnerScope), WorkspaceID: store.SQLNullString(record.WorkspaceID),
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return store.SkillExposureRecord{}, fmt.Errorf(
			"store: create skill exposure %q for target %q: %w",
			record.SkillName,
			record.TargetSlug,
			err,
		)
	}
	return skillExposureFromGenerated(row), nil
}

// GetSkillExposureByOwnerTarget returns one owner-scoped target record.
func (r *SkillExposureRepo) GetSkillExposureByOwnerTarget(
	ctx context.Context,
	skillName string,
	ownerScope store.SkillExposureOwnerScope,
	workspaceID string,
	targetSlug string,
) (store.SkillExposureRecord, error) {
	if err := r.checkReady(ctx, "get skill exposure"); err != nil {
		return store.SkillExposureRecord{}, err
	}
	row, err := r.queries.GetSkillExposureByOwnerTarget(ctx, sqlcgen.GetSkillExposureByOwnerTargetParams{
		SkillName: skillName, OwnerScope: string(ownerScope),
		WorkspaceID: store.SQLNullString(workspaceID), TargetSlug: targetSlug,
	})
	if err != nil {
		return store.SkillExposureRecord{}, fmt.Errorf(
			"store: get skill exposure %q for target %q: %w",
			skillName,
			targetSlug,
			err,
		)
	}
	return skillExposureFromGenerated(row), nil
}

// ListSkillExposuresByOwner returns deterministic records for one skill owner.
func (r *SkillExposureRepo) ListSkillExposuresByOwner(
	ctx context.Context,
	skillName string,
	ownerScope store.SkillExposureOwnerScope,
	workspaceID string,
) ([]store.SkillExposureRecord, error) {
	if err := r.checkReady(ctx, "list skill exposures by owner"); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSkillExposuresByOwner(ctx, sqlcgen.ListSkillExposuresByOwnerParams{
		SkillName: skillName, OwnerScope: string(ownerScope), WorkspaceID: store.SQLNullString(workspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list skill exposures for %q: %w", skillName, err)
	}
	return skillExposuresFromGenerated(rows), nil
}

// ListSkillExposuresByCanonicalDir returns every record that must be cleaned before skill removal.
func (r *SkillExposureRepo) ListSkillExposuresByCanonicalDir(
	ctx context.Context,
	canonicalDir string,
) ([]store.SkillExposureRecord, error) {
	if err := r.checkReady(ctx, "list skill exposures by canonical directory"); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSkillExposuresByCanonicalDir(ctx, canonicalDir)
	if err != nil {
		return nil, fmt.Errorf("store: list skill exposures for canonical directory %q: %w", canonicalDir, err)
	}
	return skillExposuresFromGenerated(rows), nil
}

// DeleteSkillExposure removes one ownership record after its proven link is gone.
func (r *SkillExposureRepo) DeleteSkillExposure(ctx context.Context, id int64) error {
	if err := r.checkReady(ctx, "delete skill exposure"); err != nil {
		return err
	}
	if err := r.queries.DeleteSkillExposure(ctx, id); err != nil {
		return fmt.Errorf("store: delete skill exposure %d: %w", id, err)
	}
	return nil
}
