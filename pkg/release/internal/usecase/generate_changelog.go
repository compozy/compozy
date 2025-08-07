package usecase

import (
	"context"

	"github.com/compozy/compozy/pkg/release/internal/service"
)

// GenerateChangelogUseCase contains the logic for the generate-changelog command.

type GenerateChangelogUseCase struct {
	CliffSvc service.CliffService
}

// Execute runs the use case.
func (uc *GenerateChangelogUseCase) Execute(ctx context.Context, version, mode string) (string, error) {
	return uc.CliffSvc.GenerateChangelog(ctx, version, mode)
}
