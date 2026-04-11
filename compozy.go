// Package compozy provides a reusable API for preparing, executing, and embedding
// markdown-driven AI work loops.
package compozy

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/cli"
	core "github.com/compozy/compozy/internal/core"

	// Register the extension-aware run-scope factory used by kernel/core runtime paths.
	_ "github.com/compozy/compozy/internal/core/extension"
	// Register dispatcher-backed adapters for the legacy public core API surface.
	_ "github.com/compozy/compozy/internal/core/kernel"
	"github.com/spf13/cobra"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = core.ErrNoWork

// Mode identifies the execution flow used by compozy.
type Mode = core.Mode

const (
	// ModePRReview processes PR review issue markdown files.
	ModePRReview = core.ModePRReview
	// ModePRDTasks processes PRD task markdown files.
	ModePRDTasks = core.ModePRDTasks
)

// IDE identifies the downstream coding tool that compozy should invoke.
type IDE = core.IDE

const (
	// IDECodex runs Codex jobs.
	IDECodex = core.IDECodex
	// IDEClaude runs Claude Code jobs.
	IDEClaude = core.IDEClaude
	// IDEDroid runs Droid jobs.
	IDEDroid = core.IDEDroid
	// IDECursor runs Cursor Agent jobs.
	IDECursor = core.IDECursor
	// IDEOpenCode runs OpenCode jobs.
	IDEOpenCode = core.IDEOpenCode
	// IDEPi runs Pi jobs.
	IDEPi = core.IDEPi
	// IDEGemini runs Gemini jobs.
	IDEGemini = core.IDEGemini
	// IDECopilot runs GitHub Copilot CLI jobs.
	IDECopilot = core.IDECopilot
)

// Config configures compozy preparation and execution.
type Config = core.Config

// Preparation contains the resolved execution plan for a compozy run.
type Preparation = core.Preparation

// FetchResult contains the output of a fetch-reviews operation.
type FetchResult = core.FetchResult

// MigrationConfig configures a repository artifact migration run.
type MigrationConfig = core.MigrationConfig

// MigrationResult contains the output of a migration run.
type MigrationResult = core.MigrationResult

// SyncConfig configures a task metadata sync run.
type SyncConfig = core.SyncConfig

// ArchiveConfig configures a completed workflow archive run.
type ArchiveConfig = core.ArchiveConfig

// SyncResult contains the output of a task metadata sync run.
type SyncResult = core.SyncResult

// ArchiveResult contains the output of a workflow archive run.
type ArchiveResult = core.ArchiveResult

// Job is a prepared execution unit with its generated artifacts.
type Job = core.Job

// NewCommand returns the reusable compozy Cobra command for embedding in other Go CLIs.
func NewCommand() *cobra.Command {
	return cli.NewRootCommand()
}

// ExitCode extracts a command-specific exit code from an execution error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return exitErr.ExitCode()
	}
	return 1
}

// Prepare resolves inputs, validates the environment, and generates batch artifacts.
func Prepare(ctx context.Context, cfg Config) (*Preparation, error) {
	return core.Prepare(ctx, cfg)
}

// Run executes compozy end to end for the provided configuration.
func Run(ctx context.Context, cfg Config) error {
	return core.Run(ctx, cfg)
}

// FetchReviews fetches provider review comments into a PRD review round.
func FetchReviews(ctx context.Context, cfg Config) (*FetchResult, error) {
	return core.FetchReviews(ctx, cfg)
}

// Migrate converts legacy workflow artifacts to frontmatter.
func Migrate(ctx context.Context, cfg MigrationConfig) (*MigrationResult, error) {
	return core.Migrate(ctx, cfg)
}

// Sync refreshes task workflow metadata files.
func Sync(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	return core.Sync(ctx, cfg)
}

// Archive moves fully completed workflows into the archive root.
func Archive(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	return core.Archive(ctx, cfg)
}
