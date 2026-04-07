package run

import (
	"context"

	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	executorpkg "github.com/compozy/compozy/internal/core/run/executor"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

var execute = executorpkg.Execute
var executeExec = execpkg.ExecuteExec

func Execute(
	ctx context.Context,
	jobs []model.Job,
	runArtifacts model.RunArtifacts,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
	cfg *model.RuntimeConfig,
) error {
	return execute(ctx, jobs, runArtifacts, runJournal, bus, cfg)
}

func ExecuteExec(ctx context.Context, cfg *model.RuntimeConfig) error {
	return executeExec(ctx, cfg)
}
