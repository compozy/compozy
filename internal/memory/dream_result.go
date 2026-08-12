package memory

import "time"

// ConsolidationResult describes one successfully committed consolidation run.
type ConsolidationResult struct {
	WorkspaceID string
	CompletedAt time.Time
}

func (s *Service) completeSuccessfulRun(
	workspaceID string,
	priorMtime time.Time,
) (ConsolidationResult, error) {
	if err := s.completeRun(true, priorMtime); err != nil {
		return ConsolidationResult{}, err
	}
	return ConsolidationResult{
		WorkspaceID: workspaceID,
		CompletedAt: s.now().UTC(),
	}, nil
}
