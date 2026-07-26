package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/transcript"
)

func sessionEventGoalPromptMeta(content string) *contract.GoalPromptMeta {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	event, err := transcript.UnmarshalAgentEvent(content)
	if err != nil {
		return nil
	}
	return goalPromptMetaPayload(event.Goal)
}
