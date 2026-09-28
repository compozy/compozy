package situation

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

func startupSessionPayload(startup session.StartupPromptContext) contract.AgentSessionPayload {
	payload := contract.AgentSessionPayload{
		ID:        strings.TrimSpace(startup.SessionID),
		Name:      strings.TrimSpace(startup.SessionName),
		Type:      startup.SessionType,
		State:     session.StateStarting,
		CreatedAt: startup.CreatedAt.UTC(),
		UpdatedAt: startup.UpdatedAt.UTC(),
	}
	return payload
}
