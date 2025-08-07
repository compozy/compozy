package usecase

import (
	"context"
	"fmt"
	"os"

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
	// If no tags exist, use INITIAL_VERSION from environment
	if latestTag == "" {
		if initialVersion := os.Getenv("INITIAL_VERSION"); initialVersion != "" {
			latestTag = initialVersion
		} else {
			latestTag = "v0.0.0" // Default fallback
		}
	}
	return uc.CliffSvc.CalculateNextVersion(ctx, latestTag)
}
