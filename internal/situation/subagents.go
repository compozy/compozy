package situation

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// SubagentProvider supplies persisted subagent counts without owning their lifecycle.
type SubagentProvider interface {
	GetSubagentByChild(context.Context, string) (store.SessionSubagent, error)
	Summaries(context.Context, []string) (map[string]store.SubagentSummary, error)
}

func (s *Service) appendSubagentSection(
	ctx context.Context,
	sections []renderedSection,
	workspaceID, sessionID string,
) ([]renderedSection, error) {
	if s == nil || s.subagents == nil || sessionID == "" {
		return sections, nil
	}
	provider := s.subagents()
	if provider == nil {
		return sections, nil
	}
	row, err := provider.GetSubagentByChild(ctx, sessionID)
	if err != nil && !errors.Is(err, store.ErrSubagentNotFound) {
		return nil, fmt.Errorf("situation: read subagent depth: %w", err)
	}
	depth := 0
	if err == nil && row.WorkspaceID == workspaceID {
		depth = row.Depth
	}
	summaries, err := provider.Summaries(ctx, []string{sessionID})
	if err != nil {
		return nil, fmt.Errorf("situation: read live subagents: %w", err)
	}
	return appendSubagentSummary(sections, depth, summaries[sessionID].Live)
}
