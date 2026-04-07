package commands

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

// ReviewsFetchCommand fetches provider review comments into a workflow review round.
type ReviewsFetchCommand struct {
	WorkspaceRoot string
	Name          string
	Round         int
	Provider      string
	PR            string
}

// ReviewsFetchResult wraps the existing review fetch result contract.
type ReviewsFetchResult struct {
	Result *model.FetchResult
}

// ReviewsFetchFromConfig translates the legacy core.Config shape into a typed review-fetch command.
func ReviewsFetchFromConfig(cfg core.Config) ReviewsFetchCommand {
	return ReviewsFetchCommand{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		Round:         cfg.Round,
		Provider:      cfg.Provider,
		PR:            cfg.PR,
	}
}

// CoreConfig converts the command into the existing fetch-reviews configuration shape.
func (c ReviewsFetchCommand) CoreConfig() core.Config {
	return core.Config{
		WorkspaceRoot: c.WorkspaceRoot,
		Name:          c.Name,
		Round:         c.Round,
		Provider:      c.Provider,
		PR:            c.PR,
	}
}
