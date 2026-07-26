package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
)

// CoordinatorConfigPayloadFromConfig converts resolved coordinator config into a safe read model.
func CoordinatorConfigPayloadFromConfig(
	cfg aghconfig.ResolvedCoordinatorRole,
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
