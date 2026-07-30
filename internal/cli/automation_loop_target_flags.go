package cli

import (
	"errors"
	"fmt"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/spf13/cobra"
)

const (
	automationAgentFlag            = "agent"
	automationLoopFlag             = "loop"
	automationLoopWorkspaceFlag    = "loop-workspace"
	automationLoopInputFlag        = "loop-input"
	automationLoopInputMappingFlag = "loop-input-mapping"
)

type automationCreateTargetInput struct {
	AgentName             string
	Prompt                string
	LoopName              string
	LoopWorkspaceRef      string
	LoopInputFlags        []string
	LoopInputMappingFlags []string
	NetworkFlags          networkParticipationFlags
}

type automationCreateTarget struct {
	Kind       automationpkg.TargetKind
	AgentName  string
	Prompt     string
	LoopTarget *automationpkg.LoopTarget
}

func bindAutomationCreateTargetFlags(
	cmd *cobra.Command,
	input *automationCreateTargetInput,
	allowInputMapping bool,
) {
	cmd.Flags().StringVar(
		&input.AgentName,
		automationAgentFlag,
		"",
		"Agent definition name; choose either --agent or --loop",
	)
	cmd.Flags().StringVar(
		&input.Prompt,
		automationPromptKey,
		"",
		"Agent prompt body; must be empty for Loop targets",
	)
	cmd.Flags().StringVar(
		&input.LoopName,
		automationLoopFlag,
		"",
		"Loop name; choose either --loop or --agent",
	)
	cmd.Flags().StringVar(
		&input.LoopWorkspaceRef,
		automationLoopWorkspaceFlag,
		"",
		"Workspace that owns the Loop; required for global automation",
	)
	cmd.Flags().StringArrayVar(
		&input.LoopInputFlags,
		automationLoopInputFlag,
		nil,
		"Static Loop input as key=value; repeat for multiple inputs",
	)
	if allowInputMapping {
		cmd.Flags().StringArrayVar(
			&input.LoopInputMappingFlags,
			automationLoopInputMappingFlag,
			nil,
			"Loop input mapping as key=template; repeat for multiple mappings",
		)
	}
	bindNetworkParticipationFlags(cmd, &input.NetworkFlags)
}

func buildAutomationCreateTarget(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	scope automationpkg.Scope,
	scopeWorkspaceID string,
	input automationCreateTargetInput,
	allowInputMapping bool,
) (automationCreateTarget, error) {
	agentName := strings.TrimSpace(input.AgentName)
	loopName := strings.TrimSpace(input.LoopName)
	switch {
	case agentName != "" && loopName != "":
		return automationCreateTarget{}, errors.New("cli: --agent and --loop cannot be combined")
	case agentName != "":
		return buildAutomationAgentTarget(agentName, input)
	case loopName != "":
		return buildAutomationLoopTarget(
			cmd,
			deps,
			client,
			scope,
			scopeWorkspaceID,
			loopName,
			input,
			allowInputMapping,
		)
	case automationLoopTargetOptionsPresent(input):
		return automationCreateTarget{}, errors.New("cli: --loop is required when Loop target flags are set")
	default:
		return automationCreateTarget{}, errors.New("cli: either --agent or --loop is required")
	}
}

func buildAutomationAgentTarget(
	agentName string,
	input automationCreateTargetInput,
) (automationCreateTarget, error) {
	if automationLoopTargetOptionsPresent(input) {
		return automationCreateTarget{}, errors.New("cli: Loop target flags cannot be used with --agent")
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return automationCreateTarget{}, errors.New("cli: --prompt is required with --agent")
	}
	return automationCreateTarget{
		Kind:      automationpkg.TargetKindAgent,
		AgentName: agentName,
		Prompt:    prompt,
	}, nil
}

func buildAutomationLoopTarget(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	scope automationpkg.Scope,
	scopeWorkspaceID string,
	loopName string,
	input automationCreateTargetInput,
	allowInputMapping bool,
) (automationCreateTarget, error) {
	if strings.TrimSpace(input.AgentName) != "" || strings.TrimSpace(input.Prompt) != "" ||
		cmd.Flags().Changed(automationPromptKey) {
		return automationCreateTarget{}, errors.New("cli: --agent and --prompt must be empty with --loop")
	}
	if !allowInputMapping && len(input.LoopInputMappingFlags) > 0 {
		return automationCreateTarget{}, errors.New("cli: scheduled Loop targets do not support input mapping")
	}
	workspaceID, err := resolveAutomationLoopWorkspace(
		cmd,
		deps,
		client,
		scope,
		scopeWorkspaceID,
		input.LoopWorkspaceRef,
	)
	if err != nil {
		return automationCreateTarget{}, err
	}
	inputs, err := parseAutomationLoopInputs(input.LoopInputFlags)
	if err != nil {
		return automationCreateTarget{}, err
	}
	inputMapping, err := parseAutomationLoopInputMappings(input.LoopInputMappingFlags)
	if err != nil {
		return automationCreateTarget{}, err
	}
	networkRequest, err := input.NetworkFlags.request()
	if err != nil {
		return automationCreateTarget{}, err
	}
	return automationCreateTarget{
		Kind: automationpkg.TargetKindLoop,
		LoopTarget: &automationpkg.LoopTarget{
			WorkspaceID:          workspaceID,
			LoopName:             loopName,
			Inputs:               inputs,
			InputMapping:         inputMapping,
			NetworkParticipation: networkRequest,
		},
	}, nil
}

func automationLoopTargetOptionsPresent(input automationCreateTargetInput) bool {
	return strings.TrimSpace(input.LoopWorkspaceRef) != "" ||
		len(input.LoopInputFlags) > 0 ||
		len(input.LoopInputMappingFlags) > 0 ||
		strings.TrimSpace(input.NetworkFlags.mode) != "" ||
		strings.TrimSpace(input.NetworkFlags.channelStrategy) != "" ||
		strings.TrimSpace(input.NetworkFlags.channel) != "" ||
		strings.TrimSpace(input.NetworkFlags.boundsJSON) != ""
}

func resolveAutomationLoopWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	scope automationpkg.Scope,
	scopeWorkspaceID string,
	workspaceRef string,
) (string, error) {
	trimmedRef := strings.TrimSpace(workspaceRef)
	if scope == automationpkg.AutomationScopeGlobal && trimmedRef == "" {
		return "", errors.New("cli: --loop-workspace is required for a global Loop target")
	}
	if trimmedRef == "" {
		return strings.TrimSpace(scopeWorkspaceID), nil
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: trimmedRef},
	)
	if err != nil {
		return "", fmt.Errorf("cli: resolve Loop workspace: %w", err)
	}
	if scope == automationpkg.AutomationScopeWorkspace && resolution.ID != strings.TrimSpace(scopeWorkspaceID) {
		return "", errors.New("cli: --loop-workspace must match the workspace automation scope")
	}
	return resolution.ID, nil
}

func parseAutomationLoopInputs(flags []string) (map[string]any, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	values := make(map[string]any, len(flags))
	for _, flag := range flags {
		key, rawValue, err := splitLoopKeyValue(flag)
		if err != nil {
			return nil, fmt.Errorf("cli: parse --loop-input: %w", err)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("cli: duplicate --loop-input key %q", key)
		}
		values[key] = parseLoopValue(rawValue)
	}
	return values, nil
}

func parseAutomationLoopInputMappings(flags []string) (map[string]string, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	values := make(map[string]string, len(flags))
	for _, flag := range flags {
		key, value, err := splitLoopKeyValue(flag)
		if err != nil {
			return nil, fmt.Errorf("cli: parse --loop-input-mapping: %w", err)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("cli: duplicate --loop-input-mapping key %q", key)
		}
		values[key] = value
	}
	return values, nil
}
