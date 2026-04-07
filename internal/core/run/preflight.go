package run

import (
	"context"

	preflightpkg "github.com/compozy/compozy/internal/core/run/preflight"
	"github.com/compozy/compozy/internal/core/tasks"
)

type PreflightDecision = preflightpkg.Decision
type PreflightConfig = preflightpkg.Config

const (
	PreflightOK        = preflightpkg.OK
	PreflightContinued = preflightpkg.Continued
	PreflightAborted   = preflightpkg.Aborted
	PreflightSkipped   = preflightpkg.Skipped
	PreflightForced    = preflightpkg.Forced
)

func PreflightCheck(
	ctx context.Context,
	tasksDir string,
	registry *tasks.TypeRegistry,
	isInteractive func() bool,
	force bool,
) (PreflightDecision, error) {
	return preflightpkg.Check(ctx, tasksDir, registry, isInteractive, force)
}

func PreflightCheckConfig(ctx context.Context, cfg PreflightConfig) (PreflightDecision, error) {
	return preflightpkg.CheckConfig(ctx, cfg)
}
