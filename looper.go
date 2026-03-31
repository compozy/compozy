// Package looper provides a reusable API for preparing and executing
// markdown-driven AI work loops plus a public Cobra wrapper in package command.
package looper

import (
	"context"

	core "github.com/compozy/looper/internal/looper"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = core.ErrNoWork

// Mode identifies the execution flow used by looper.
type Mode = core.Mode

const (
	// ModePRReview processes PR review issue markdown files.
	ModePRReview = core.ModePRReview
	// ModePRDTasks processes PRD task markdown files.
	ModePRDTasks = core.ModePRDTasks
)

// IDE identifies the downstream coding tool that looper should invoke.
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
)

// Config configures looper preparation and execution.
type Config = core.Config

// Preparation contains the resolved execution plan for a looper run.
type Preparation = core.Preparation

// FetchResult contains the output of a fetch-reviews operation.
type FetchResult = core.FetchResult

// Job is a prepared execution unit with its generated artifacts.
type Job = core.Job

// Prepare resolves inputs, validates the environment, and generates batch artifacts.
func Prepare(ctx context.Context, cfg Config) (*Preparation, error) {
	return core.Prepare(ctx, cfg)
}

// Run executes looper end to end for the provided configuration.
func Run(ctx context.Context, cfg Config) error {
	return core.Run(ctx, cfg)
}

// FetchReviews fetches provider review comments into a PRD review round.
func FetchReviews(ctx context.Context, cfg Config) (*FetchResult, error) {
	return core.FetchReviews(ctx, cfg)
}
