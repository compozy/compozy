package commands

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

// WorkspaceMigrateCommand migrates workflow artifacts under one workspace target.
type WorkspaceMigrateCommand struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
	ReviewsDir    string
	DryRun        bool
}

// WorkspaceMigrateResult wraps the existing migration result contract.
type WorkspaceMigrateResult struct {
	Result *model.MigrationResult
}

// WorkspaceMigrateFromConfig translates the legacy core.Config shape into a typed migration command.
func WorkspaceMigrateFromConfig(cfg core.Config) WorkspaceMigrateCommand {
	return WorkspaceMigrateCommand{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
		ReviewsDir:    cfg.ReviewsDir,
		DryRun:        cfg.DryRun,
	}
}

// WorkspaceMigrateFromMigrationConfig translates the direct migration config into
// a typed migration command without lossy field remapping at the caller.
func WorkspaceMigrateFromMigrationConfig(cfg model.MigrationConfig) WorkspaceMigrateCommand {
	return WorkspaceMigrateCommand{
		WorkspaceRoot: cfg.WorkspaceRoot,
		RootDir:       cfg.RootDir,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
		ReviewsDir:    cfg.ReviewsDir,
		DryRun:        cfg.DryRun,
	}
}

// CoreConfig converts the command into the existing migration configuration shape.
func (c WorkspaceMigrateCommand) CoreConfig() model.MigrationConfig {
	return model.MigrationConfig{
		WorkspaceRoot: c.WorkspaceRoot,
		RootDir:       c.RootDir,
		Name:          c.Name,
		TasksDir:      c.TasksDir,
		ReviewsDir:    c.ReviewsDir,
		DryRun:        c.DryRun,
	}
}
