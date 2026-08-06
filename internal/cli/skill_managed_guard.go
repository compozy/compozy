package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

var errManagedSessionSkillCLIUnsupported = errors.New(
	"cli: managed sessions cannot run compozy skill commands; " +
		"use canonical compozy__skill_list, compozy__skill_search, or compozy__skill_view for read-only requests; " +
		"skill install, remove, create, enable, disable, and update require an operator shell",
)

func ensureSkillCLIUsesSupportedSurface(deps commandDeps) error {
	if strings.TrimSpace(deps.getenv(agentidentity.EnvSessionID)) == "" &&
		strings.TrimSpace(deps.getenv(agentidentity.EnvAgent)) == "" {
		return nil
	}
	return errManagedSessionSkillCLIUnsupported
}
