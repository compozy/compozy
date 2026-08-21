package core

import (
	"context"

	profilepkg "github.com/compozy/compozy/internal/profile"
)

type profileCatalog interface {
	Create(context.Context, profilepkg.CreateInput) (profilepkg.Profile, error)
	GetByName(context.Context, string) (profilepkg.Profile, error)
	List(context.Context) ([]profilepkg.ProfileWithCounts, error)
	UpdateIdentity(context.Context, string, profilepkg.IdentityPatch) (profilepkg.Profile, error)
	ListSelections(context.Context) ([]profilepkg.Selection, error)
	PutSelection(context.Context, profilepkg.Selection) error
}

type profilePlans interface {
	PrepareRename(context.Context, string, string) (profilepkg.RenamePlan, error)
	PrepareArchive(context.Context, string) (profilepkg.ArchivePlan, error)
	PrepareDelete(context.Context, string) (profilepkg.DeletePlan, error)
}

type profileLifecycle interface {
	Rename(context.Context, string, profilepkg.RenameOptions) (profilepkg.RenameResult, error)
	Archive(context.Context, string, string) (profilepkg.ArchiveResult, error)
	Unarchive(context.Context, string) (profilepkg.UnarchiveResult, error)
	Delete(context.Context, string, string) (profilepkg.DeleteResult, error)
	ListOps(context.Context) ([]profilepkg.LifecycleOp, error)
	RetryOp(context.Context, string) (profilepkg.LifecycleOp, error)
}

// ProfileService is the daemon-owned profile boundary exposed by both transports.
type ProfileService interface {
	profileCatalog
	profilePlans
	profileLifecycle
}
