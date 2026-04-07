package core

import (
	"context"

	migrationpkg "github.com/compozy/compozy/internal/core/migration"
)

func migrateArtifacts(ctx context.Context, cfg MigrationConfig) (*MigrationResult, error) {
	return migrationpkg.Migrate(ctx, cfg)
}
