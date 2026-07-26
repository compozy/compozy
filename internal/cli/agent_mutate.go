package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/spf13/cobra"
)

func newAgentCreateCommand(deps commandDeps) *cobra.Command {
	var flags agentDefinitionFlags
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a global or workspace-local agent definition",
		Long:  "Create a global or workspace-local agent definition through a running AGH daemon.",
		Example: `  agh agent create pricing_strategist \
    --workspace ~/dev/ad8 \
    --provider claude \
    --model claude-sonnet-5 \
    --reasoning-effort max \
    --prompt "You own pricing strategy." \
    -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return fmt.Errorf("cli: initialize agent client: %w", err)
			}
			request, err := createAgentRequestFromFlags(cmd, args[0], flags)
			if err != nil {
				return fmt.Errorf("cli: build create-agent request: %w", err)
			}
			agent, err := client.CreateAgent(cmd.Context(), request)
			if err != nil {
				return fmt.Errorf("cli: create agent: %w", err)
			}
			if err := writeCommandOutput(cmd, agentBundle(agent)); err != nil {
				return fmt.Errorf("cli: write created agent output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().String("workspace", "", "Workspace id, name, or path to create the agent under")
	addAgentDefinitionFlags(cmd, &flags)
	return cmd
}

func newAgentUpdateCommand(deps commandDeps) *cobra.Command {
	var flags agentDefinitionFlags
	var expectedDigest string
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an agent definition for future sessions",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return fmt.Errorf("cli: initialize agent client: %w", err)
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return fmt.Errorf("cli: resolve update workspace: %w", err)
			}
			request, err := updateAgentRequestFromFlags(
				cmd,
				client,
				args[0],
				workspace,
				expectedDigest,
				flags,
			)
			if err != nil {
				return fmt.Errorf("cli: build update-agent request: %w", err)
			}
			agent, err := client.UpdateAgent(cmd.Context(), args[0], request)
			if err != nil {
				return fmt.Errorf("cli: update agent: %w", err)
			}
			if err := writeCommandOutput(cmd, agentBundle(agent)); err != nil {
				return fmt.Errorf("cli: write updated agent output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().String("workspace", "", "Resolve the effective agent from a workspace")
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Definition digest from the last read")
	addAgentDefinitionFlags(cmd, &flags)
	return cmd
}

func newAgentDeleteCommand(deps commandDeps) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Durably delete an agent definition",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmAgentDelete(cmd, args[0], yes, deps.inputIsTerminal); err != nil {
				return fmt.Errorf("cli: confirm agent deletion: %w", err)
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return fmt.Errorf("cli: resolve delete workspace: %w", err)
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return fmt.Errorf("cli: initialize agent client: %w", err)
			}
			result, err := client.DeleteAgent(cmd.Context(), args[0], workspace)
			if err != nil {
				return fmt.Errorf("cli: delete agent: %w", err)
			}
			if err := writeCommandOutput(cmd, agentDeleteBundle(result)); err != nil {
				return fmt.Errorf("cli: write deleted agent output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().String("workspace", "", "Resolve the effective agent from a workspace")
	cmd.Flags().BoolVar(&yes, yesFlagName, false, "Confirm durable deletion without prompting")
	return cmd
}

func newAgentDuplicateCommand(deps commandDeps) *cobra.Command {
	var flags agentDefinitionFlags
	var scope string
	cmd := &cobra.Command{
		Use:   "duplicate <source> <new-name>",
		Short: "Duplicate an agent definition without exposing secrets",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return fmt.Errorf("cli: initialize agent client: %w", err)
			}
			request, err := duplicateAgentRequestFromFlags(cmd, args[1], scope, flags)
			if err != nil {
				return fmt.Errorf("cli: build duplicate-agent request: %w", err)
			}
			agent, err := client.DuplicateAgent(cmd.Context(), args[0], request)
			if err != nil {
				return fmt.Errorf("cli: duplicate agent: %w", err)
			}
			if err := writeCommandOutput(cmd, agentBundle(agent)); err != nil {
				return fmt.Errorf("cli: write duplicated agent output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().String("workspace", "", "Resolve the source or target workspace")
	cmd.Flags().StringVar(&scope, "scope", "", "Target scope: global or workspace (default: source origin)")
	addAgentDefinitionFlags(cmd, &flags)
	return cmd
}

func confirmAgentDelete(
	cmd *cobra.Command,
	name string,
	yes bool,
	isTerminal func(io.Reader) bool,
) error {
	if yes {
		return nil
	}
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputHuman {
		return errors.New("cli: agent deletion requires --yes for structured output")
	}
	input := cmd.InOrStdin()
	if isTerminal == nil || !isTerminal(input) {
		return errors.New("cli: agent deletion requires --yes when stdin is not interactive")
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Delete agent %q? [y/N] ", strings.TrimSpace(name)); err != nil {
		return fmt.Errorf("cli: write agent deletion prompt: %w", err)
	}
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("cli: read agent deletion consent: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != yesFlagName {
		return errors.New("cli: agent deletion declined")
	}
	return nil
}

func agentDeleteBundle(result contract.DeleteAgentResponse) outputBundle {
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			return renderHumanSection("Agent deleted", []keyValue{
				{Label: automationNameValue, Value: result.Name},
				{Label: taskOriginValue, Value: string(result.Origin)},
				{Label: "Unshadowed Origin", Value: stringOrDash(string(result.UnshadowedOrigin))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_deleted", []string{
				automationNameKey, taskOriginKey, "unshadowed_origin",
			}, []string{
				result.Name, string(result.Origin), string(result.UnshadowedOrigin),
			}), nil
		},
	}
}

func normalizePublicAgentName(name string) (string, error) {
	name = aghconfig.NormalizeAgentName(name)
	if err := aghconfig.ValidateAgentName(name); err != nil {
		return "", err
	}
	return name, nil
}

func validateAgentReasoningEffort(value string) error {
	if err := session.ValidateReasoningEffort(strings.TrimSpace(value)); err != nil {
		return fmt.Errorf("cli: invalid agent reasoning effort: %w", err)
	}
	return nil
}

func parseAgentCreateScope(value string) (contract.AgentCreateScope, error) {
	scope := contract.AgentCreateScope(strings.ToLower(strings.TrimSpace(value)))
	switch scope {
	case "", contract.AgentCreateScopeGlobal, contract.AgentCreateScopeWorkspace:
		return scope, nil
	default:
		return "", fmt.Errorf("cli: invalid agent scope %q", value)
	}
}
