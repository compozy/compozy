package usecase

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/repository"
)

// CreateGitTagUseCase contains the logic for the create-git-tag command.

type CreateGitTagUseCase struct {
	GitRepo repository.GitRepository
}

// Execute runs the use case.
func (uc *CreateGitTagUseCase) Execute(ctx context.Context, tagName, message string) error {
	if err := uc.GitRepo.CreateTag(ctx, tagName, message); err != nil {
		return fmt.Errorf("failed to create git tag: %w", err)
	}
	return uc.GitRepo.PushTag(ctx, tagName)
}
