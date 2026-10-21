package automation

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ListSuggestionsAggregate reads existing suggestions without seeding workspace defaults.
func (m *Manager) ListSuggestionsAggregate(ctx context.Context, scope store.ReadScope, status SuggestionStatus) ([]Suggestion, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if status != "" {
		if err := status.Validate("status"); err != nil {
			return nil, err
		}
	}
	reader, ok := m.store.(interface {
		ListSuggestionsAggregate(context.Context, store.ReadScope, SuggestionStatus) ([]Suggestion, error)
	})
	if !ok {
		return nil, fmt.Errorf("automation suggestion aggregate unavailable")
	}
	return reader.ListSuggestionsAggregate(ctx, scope, status)
}
