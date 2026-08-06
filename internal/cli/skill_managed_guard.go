package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

var errManagedSessionSkillCLIUnsupported = errors.New(
	"cli: skill commands are available only from an operator shell; " +
		"managed sessions must use compozy__skill_list, compozy__skill_search, or compozy__skill_view",
)

func ensureSkillCLIUsesSupportedSurface(deps commandDeps) error {
	if strings.TrimSpace(deps.getenv(agentidentity.EnvSessionID)) == "" &&
		strings.TrimSpace(deps.getenv(agentidentity.EnvAgent)) == "" {
		return nil
	}
	return errManagedSessionSkillCLIUnsupported
}
