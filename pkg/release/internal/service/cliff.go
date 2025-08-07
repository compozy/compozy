package service

import (
	"context"

	"github.com/compozy/compozy/pkg/release/internal/domain"
)

// CliffService defines the interface for interacting with git-cliff.

type CliffService interface {
	CalculateNextVersion(ctx context.Context, latestTag string) (*domain.Version, error)
	GenerateChangelog(ctx context.Context, version, mode string) (string, error)
}
