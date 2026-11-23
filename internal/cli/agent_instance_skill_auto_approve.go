package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

// Actions of `compozy agent instance skill-auto-approve on|off`.
const (
	agentInstanceSkillAutoApproveOn  = "skill-auto-approve-on"
	agentInstanceSkillAutoApproveOff = "skill-auto-approve-off"
)

func newAgentInstanceSkillAutoApproveCommand(deps commandDeps) *cobra.Command {
	parent := &cobra.Command{
		Use:   "skill-auto-approve",
		Short: "Turn skill auto-approve on (operator only) or off (experimental)",
	}
	for _, action := range []string{agentInstanceSkillAutoApproveOn, agentInstanceSkillAutoApproveOff} {
		cmd := newAgentInstanceAction(deps, action)
		state := skillAutoApproveState(action == agentInstanceSkillAutoApproveOn)
		cmd.Use = state + " <agent-or-instance-id>"
		cmd.Short = "Turn skill auto-approve " + state + " for an always-on instance"
		parent.AddCommand(cmd)
	}
	return parent
}

// agentInstanceUpdates are the PATCH verbs of one instance, each changing exactly one field.
var agentInstanceUpdates = map[string]func(agentInstanceOptions, *cobra.Command, any, string, string) error{
	agentInstanceBudgetAction: func(options agentInstanceOptions, cmd *cobra.Command, base any, id, _ string) error {
		return options.updateBudget(cmd, base, id)
	},
	agentInstanceSkillAutoApproveOn:  setAgentInstanceSkillAutoApprove,
	agentInstanceSkillAutoApproveOff: setAgentInstanceSkillAutoApprove,
}

func setAgentInstanceSkillAutoApprove(_ agentInstanceOptions, cmd *cobra.Command, base any, id, action string) error {
	client, ok := base.(agentInstanceSkillAutoApproveClient)
	if !ok {
		return errors.New("cli: instance skill auto-approve unavailable")
	}
	result, err := client.SetAgentInstanceSkillAutoApprove(cmd.Context(), id, action == agentInstanceSkillAutoApproveOn)
	if err != nil {
		return err
	}
	return writeInstanceOutput(cmd, result, skillAutoApproveText(result.Instance))
}

func skillAutoApproveText(instance contract.AgentInstancePayload) string {
	label := instance.Label
	if label == "" {
		label = instance.AgentName
	}
	return fmt.Sprintf("Skill auto-approve is %s for %s.\n", skillAutoApproveState(instance.SkillAutoApprove), label)
}

func skillAutoApproveState(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

type agentInstanceSkillAutoApproveClient interface {
	SetAgentInstanceSkillAutoApprove(context.Context, string, bool) (contract.AgentInstanceResponse, error)
}

var _ agentInstanceSkillAutoApproveClient = (*daemonClient)(nil)

func (c *daemonClient) SetAgentInstanceSkillAutoApprove(
	ctx context.Context,
	id string,
	enabled bool,
) (contract.AgentInstanceResponse, error) {
	var result contract.AgentInstanceResponse
	err := c.doJSON(ctx, http.MethodPatch, "/api/agent-instances/"+url.PathEscape(id), nil,
		contract.AgentInstanceUpdateRequest{SkillAutoApprove: &enabled}, &result)
	return result, err
}
