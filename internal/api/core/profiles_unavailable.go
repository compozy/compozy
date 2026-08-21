package core

import (
	"context"

	profilepkg "github.com/compozy/compozy/internal/profile"
)

// unavailableProfileService is the explicit fallback used when handlers are
// constructed without the daemon-owned profile boundary.
type unavailableProfileService struct{}

func (unavailableProfileService) unavailable() error {
	return &profilepkg.Error{
		Code:    "profile_unavailable",
		Message: "profile service is unavailable",
		Action:  "start the daemon and retry",
		Cause:   profilepkg.ErrUnavailable,
	}
}

func (s unavailableProfileService) Create(context.Context, profilepkg.CreateInput) (profilepkg.Profile, error) {
	return profilepkg.Profile{}, s.unavailable()
}

func (s unavailableProfileService) GetByName(context.Context, string) (profilepkg.Profile, error) {
	return profilepkg.Profile{}, s.unavailable()
}

func (s unavailableProfileService) GetWithCounts(context.Context, string) (profilepkg.WithCounts, error) {
	return profilepkg.WithCounts{}, s.unavailable()
}

func (s unavailableProfileService) Resolve(context.Context, profilepkg.ResolveInput) (profilepkg.Resolution, error) {
	return profilepkg.Resolution{}, s.unavailable()
}

func (s unavailableProfileService) List(context.Context) ([]profilepkg.WithCounts, error) {
	return nil, s.unavailable()
}

func (s unavailableProfileService) ListNames(context.Context) ([]string, error) {
	return nil, s.unavailable()
}

func (s unavailableProfileService) UpdateIdentity(
	context.Context,
	string,
	profilepkg.IdentityPatch,
) (profilepkg.Profile, error) {
	return profilepkg.Profile{}, s.unavailable()
}

func (s unavailableProfileService) ListSelections(context.Context) ([]profilepkg.Selection, error) {
	return nil, s.unavailable()
}

func (s unavailableProfileService) PutSelection(context.Context, profilepkg.Selection) error {
	return s.unavailable()
}

func (s unavailableProfileService) PrepareRename(context.Context, string, string) (profilepkg.RenamePlan, error) {
	return profilepkg.RenamePlan{}, s.unavailable()
}

func (s unavailableProfileService) PrepareArchive(context.Context, string) (profilepkg.ArchivePlan, error) {
	return profilepkg.ArchivePlan{}, s.unavailable()
}

func (s unavailableProfileService) PrepareDelete(context.Context, string) (profilepkg.DeletePlan, error) {
	return profilepkg.DeletePlan{}, s.unavailable()
}

func (s unavailableProfileService) Rename(
	context.Context,
	string,
	profilepkg.RenameOptions,
) (profilepkg.RenameResult, error) {
	return profilepkg.RenameResult{}, s.unavailable()
}

func (s unavailableProfileService) Archive(context.Context, string, string) (profilepkg.ArchiveResult, error) {
	return profilepkg.ArchiveResult{}, s.unavailable()
}

func (s unavailableProfileService) Unarchive(context.Context, string) (profilepkg.UnarchiveResult, error) {
	return profilepkg.UnarchiveResult{}, s.unavailable()
}

func (s unavailableProfileService) Delete(context.Context, string, string) (profilepkg.DeleteResult, error) {
	return profilepkg.DeleteResult{}, s.unavailable()
}

func (s unavailableProfileService) ListOps(context.Context) ([]profilepkg.LifecycleOp, error) {
	return nil, s.unavailable()
}

func (s unavailableProfileService) RetryOp(context.Context, string) (profilepkg.LifecycleOp, error) {
	return profilepkg.LifecycleOp{}, s.unavailable()
}

var _ ProfileService = unavailableProfileService{}
