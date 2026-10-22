package globaldb

import (
	"context"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *AutomationRepo) ListSuggestionsAggregate(
	ctx context.Context,
	scope store.ReadScope,
	status automation.SuggestionStatus,
) ([]automation.Suggestion, error) {
	if err := g.checkReady(ctx, "aggregate automation suggestions"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListAutomationSuggestionsAggregate(
		ctx,
		sqlcgen.ListAutomationSuggestionsAggregateParams{
			AllProfiles: scope.AllProfiles,
			ProfileID:   scope.ProfileID,
			Status:      string(status),
		},
	)
	if err != nil {
		return nil, err
	}
	return automationSuggestionsFromGenerated(rows)
}
