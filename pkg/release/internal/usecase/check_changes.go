package usecase

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
)

// CheckChangesUseCase contains the logic for the check-changes command.

type CheckChangesUseCase struct {
	GitRepo  repository.GitRepository
	CliffSvc service.CliffService
}

// Execute runs the use case.
func (uc *CheckChangesUseCase) Execute(ctx context.Context) (bool, string, error) {
	latestTag, err := uc.GitRepo.LatestTag(ctx)
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest tag: %w", err)
	}
	if latestTag == "" {
		return true, "", nil // Initial release
	}
	commitsSince, err := uc.GitRepo.CommitsSinceTag(ctx, latestTag)
	if err != nil {
		return false, latestTag, fmt.Errorf("failed to get commits since tag: %w", err)
	}
	if commitsSince == 0 {
		return false, latestTag, nil
	}
	nextVer, err := uc.CliffSvc.CalculateNextVersion(ctx, latestTag)
	if err != nil {
		return false, latestTag, fmt.Errorf("failed to calculate next version: %w", err)
	}
	return nextVer.String() != latestTag, latestTag, nil
}
