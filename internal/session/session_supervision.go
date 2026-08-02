package session

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
)

func supervisionForSession(
	session *Session,
	configured compozyconfig.SessionSupervisionConfig,
) compozyconfig.SessionSupervisionConfig {
	if session == nil {
		return configured
	}
	loopOwnerPrefix := string(participation.OwnerKindLoopRun) + ":"
	if !strings.HasPrefix(strings.TrimSpace(session.NetworkOwnerKey), loopOwnerPrefix) {
		return configured
	}
	configured.InactivityWarningAfter = 0
	configured.InactivityTimeout = 0
	return configured
}
