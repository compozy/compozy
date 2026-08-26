package globaldb

import (
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func skillExposureFromGenerated(row sqlcgen.SkillExposure) store.SkillExposureRecord {
	return store.SkillExposureRecord{
		ID: row.ID, SkillName: row.SkillName, CanonicalDir: row.CanonicalDir,
		TargetSlug: row.TargetSlug, LinkPath: row.LinkPath, LinkTarget: row.LinkTarget,
		OwnerScope: store.SkillExposureOwnerScope(row.OwnerScope), WorkspaceID: row.WorkspaceID.String,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func skillExposuresFromGenerated(rows []sqlcgen.SkillExposure) []store.SkillExposureRecord {
	records := make([]store.SkillExposureRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, skillExposureFromGenerated(row))
	}
	return records
}
