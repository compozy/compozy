package cli

import (
	"github.com/compozy/agh/internal/agentidentity"
	"github.com/spf13/cobra"
)

const (
	whoamiAgentValue = "Agent"
	whoamiAgentKey   = "agent"
)

const (
	envSessionID = agentidentity.EnvSessionID
	envAgentID   = agentidentity.EnvAgent
	envAgentName = "AGH_AGENT_NAME"
)

func newWhoamiCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print the current AGH agent identity from environment variables",
		RunE: func(cmd *cobra.Command, _ []string) error {
			identity := IdentityRecord{
				SessionID: deps.getenv(envSessionID),
				Agent:     deps.getenv(envAgentID),
				AgentName: deps.getenv(envAgentName),
			}
			return writeCommandOutput(cmd, whoamiBundle(identity))
		},
	}
}

func whoamiBundle(identity IdentityRecord) outputBundle {
	return outputBundle{
		jsonValue: identity,
		human: func() (string, error) {
			return renderHumanSection("Identity", []keyValue{
				{Label: sessionIDLabel, Value: stringOrDash(identity.SessionID)},
				{Label: whoamiAgentValue, Value: stringOrDash(identity.Agent)},
				{Label: "Agent Name", Value: stringOrDash(identity.AgentName)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"identity",
				[]string{automationSessionIDKey, whoamiAgentKey, installAgentNameKey},
				[]string{
					identity.SessionID,
					identity.Agent,
					identity.AgentName,
				},
			), nil
		},
	}
}
