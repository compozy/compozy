package acpmock

import (
	"fmt"

	"github.com/compozy/agh/internal/acp"
)

func validatePromptStopReason(path string, stopReason string) error {
	switch stopReason {
	case string(acp.PromptStopReasonEndTurn),
		string(acp.PromptStopReasonMaxTokens),
		string(acp.PromptStopReasonMaxTurnRequests),
		string(acp.PromptStopReasonRefusal),
		string(acp.PromptStopReasonCancelled):
		return nil
	default:
		return fmt.Errorf("acpmock: %s.stop_reason %q is invalid", path, stopReason)
	}
}
