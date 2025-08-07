package usecase

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/repository"
)

// CreateReleaseBranchUseCase contains the logic for the create-release-branch command.

type CreateReleaseBranchUseCase struct {
	GitRepo repository.GitRepository
}

// Execute runs the use case.
func (uc *CreateReleaseBranchUseCase) Execute(ctx context.Context, branchName string) error {
	if err := uc.GitRepo.CreateBranch(ctx, branchName); err != nil {
		return fmt.Errorf("failed to create release branch: %w", err)
	}
	return uc.GitRepo.PushBranch(ctx, branchName)
}
