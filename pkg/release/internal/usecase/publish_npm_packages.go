package usecase

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
	"github.com/spf13/afero"
)

// PublishNpmPackagesUseCase contains the logic for the publish-npm-packages command.
type PublishNpmPackagesUseCase struct {
	FsRepo   repository.FileSystemRepository
	NpmSvc   service.NpmService
	ToolsDir string
}

// Execute runs the use case.
func (uc *PublishNpmPackagesUseCase) Execute(ctx context.Context) error {
	return afero.Walk(uc.FsRepo, uc.ToolsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != uc.ToolsDir {
			if err := uc.NpmSvc.Publish(ctx, path); err != nil {
				return fmt.Errorf("failed to publish package at %s: %w", path, err)
			}
		}
		return nil
	})
}
