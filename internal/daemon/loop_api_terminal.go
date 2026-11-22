package daemon

import (
	"context"
	"maps"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
)

// loopRunTerminalCauses batches the durable terminal cause read used by run
// list and run status projections.
func (s *daemonLoopAPIService) loopRunTerminalCauses(
	ctx context.Context,
	runs []looppkg.Run,
) (map[looppkg.RunID]looppkg.RunTerminalCause, error) {
	reader, ok := s.persistence.(looppkg.RunTerminalCauseReader)
	if !ok || len(runs) == 0 {
		return nil, nil
	}
	groups := make(map[looppkg.WorkspaceID][]looppkg.RunID)
	for _, run := range runs {
		groups[run.WorkspaceID] = append(groups[run.WorkspaceID], run.ID)
	}
	causes := make(map[looppkg.RunID]looppkg.RunTerminalCause, len(runs))
	for workspaceID, runIDs := range groups {
		found, err := reader.ListLoopRunTerminalCauses(ctx, workspaceID, runIDs)
		if err != nil {
			return nil, err
		}
		maps.Copy(causes, found)
	}
	return causes, nil
}

func applyLoopRunTerminalCause(payload *contract.LoopRunPayload, cause looppkg.RunTerminalCause) {
	payload.Terminal = &contract.LoopRunTerminal{
		Status:     contract.LoopRunStatus(cause.Status),
		Cause:      string(cause.Cause),
		ReasonCode: cause.ReasonCode,
		GateID:     string(cause.GateID),
		NodeID:     string(cause.NodeID),
		Message:    cause.Message,
		At:         cause.At,
	}
}
