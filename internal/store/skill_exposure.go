package store

import (
	"context"
	"time"
)

// SkillExposureOwnerScope identifies the shared owner of one provider-root link.
type SkillExposureOwnerScope string

const (
	SkillExposureOwnerUser      SkillExposureOwnerScope = "user"
	SkillExposureOwnerWorkspace SkillExposureOwnerScope = "workspace"
)

// SkillExposureRecord is the durable ownership proof for one created skill link.
type SkillExposureRecord struct {
	ID           int64
	SkillName    string
	CanonicalDir string
	TargetSlug   string
	LinkPath     string
	LinkTarget   string
	OwnerScope   SkillExposureOwnerScope
	WorkspaceID  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SkillExposureRepository persists exposure ownership independently from link health.
type SkillExposureRepository interface {
	CreateSkillExposure(context.Context, SkillExposureRecord) (SkillExposureRecord, error)
	GetSkillExposureByOwnerTarget(
		context.Context,
		string,
		SkillExposureOwnerScope,
		string,
		string,
	) (SkillExposureRecord, error)
	ListSkillExposuresByOwner(
		context.Context,
		string,
		SkillExposureOwnerScope,
		string,
	) ([]SkillExposureRecord, error)
	ListSkillExposuresByCanonicalDir(context.Context, string) ([]SkillExposureRecord, error)
	DeleteSkillExposure(context.Context, int64) error
}
