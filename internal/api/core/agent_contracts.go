package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// CoordinatorConfigPayloadFromConfig converts resolved coordinator config into a safe read model.
func CoordinatorConfigPayloadFromConfig(
	cfg compozyconfig.ResolvedCoordinatorRole,
	source contract.CoordinatorConfigSource,
	workspaceID string,
) contract.CoordinatorConfigPayload {
	return contract.CoordinatorConfigPayload{
		Enabled:                       cfg.Enabled,
		AgentName:                     strings.TrimSpace(cfg.AgentName),
		Provider:                      strings.TrimSpace(cfg.Provider),
		Model:                         strings.TrimSpace(cfg.Model),
		DefaultTTLSeconds:             int64(cfg.TTL.Seconds()),
		MaxChildren:                   cfg.MaxChildren,
		MaxActiveSessionsPerWorkspace: cfg.MaxActiveSessionsPerWorkspace,
		Source:                        source,
		WorkspaceID:                   strings.TrimSpace(workspaceID),
	}
}
