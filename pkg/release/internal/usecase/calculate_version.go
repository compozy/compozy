package usecase

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
)

// CalculateVersionUseCase contains the logic for the calculate-version command.

type CalculateVersionUseCase struct {
	GitRepo  repository.GitRepository
	CliffSvc service.CliffService
}

// Execute runs the use case.
func (uc *CalculateVersionUseCase) Execute(ctx context.Context) (*domain.Version, error) {
	latestTag, err := uc.GitRepo.LatestTag(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest tag: %w", err)
	}

	return uc.CliffSvc.CalculateNextVersion(ctx, latestTag)
}
