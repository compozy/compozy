package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/spf13/cobra"
)

var errManagedSessionSkillCLIUnsupported = errors.New(
	"cli: managed sessions can run only compozy skill expose and compozy skill unexpose; " +
		"use canonical compozy__skill_list, compozy__skill_search, or compozy__skill_view for read-only requests; " +
		"skill install, remove, create, enable, disable, and update require an operator shell",
)

func ensureSkillCLIUsesSupportedSurface(cmd *cobra.Command, deps commandDeps) error {
	if strings.TrimSpace(deps.getenv(agentidentity.EnvSessionID)) == "" &&
		strings.TrimSpace(deps.getenv(agentidentity.EnvAgent)) == "" {
		return nil
	}
	if cmd != nil && (cmd.Name() == "expose" || cmd.Name() == "unexpose") {
		return nil
	}
	return errManagedSessionSkillCLIUnsupported
}
