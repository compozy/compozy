package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentDefinitionFlags struct {
	provider        string
	command         string
	model           string
	reasoningEffort string
	prompt          string
	promptFile      string
	tools           []string
	toolsets        []string
	denyTools       []string
	permissions     string
	categoryPath    []string
	disabledSkills  []string
}

func addAgentDefinitionFlags(cmd *cobra.Command, flags *agentDefinitionFlags) {
	cmd.Flags().StringVar(&flags.provider, cliProviderKey, "", "Provider name for sessions using this agent")
	cmd.Flags().StringVar(&flags.command, agentCommandKey, "", "Optional provider command override")
	cmd.Flags().StringVar(&flags.model, agentModelKey, "", "Optional provider model")
	cmd.Flags().StringVar(&flags.reasoningEffort, agentReasoningEffortKey, "", "Optional default reasoning effort")
	cmd.Flags().StringVar(&flags.prompt, "prompt", "", "Agent system prompt body")
	cmd.Flags().StringVar(&flags.promptFile, "prompt-file", "", "Read the agent system prompt body from a file")
	cmd.Flags().StringArrayVar(&flags.tools, "tool", nil, "Allowed tool pattern (repeatable)")
	cmd.Flags().StringArrayVar(&flags.toolsets, "toolset", nil, "Allowed toolset reference (repeatable)")
	cmd.Flags().StringArrayVar(&flags.denyTools, "deny-tool", nil, "Denied tool pattern (repeatable)")
	cmd.Flags().StringVar(&flags.permissions, configPermissionsKey, "", "Optional permission mode")
	cmd.Flags().StringArrayVar(&flags.categoryPath, agentCategoryKey, nil, "Agent category path segment (repeatable)")
	cmd.Flags().StringArrayVar(
		&flags.disabledSkills,
		agentDisableSkillFlag,
		nil,
		"Disabled skill name (repeatable)",
	)
}

func createAgentRequestFromFlags(
	cmd *cobra.Command,
	name string,
	flags agentDefinitionFlags,
) (contract.CreateAgentRequest, error) {
	name, err := normalizePublicAgentName(name)
	if err != nil {
		return contract.CreateAgentRequest{}, err
	}
	prompt, _, err := agentPromptFromFlags(cmd, flags, true)
	if err != nil {
		return contract.CreateAgentRequest{}, err
	}
	if err := validateAgentReasoningEffort(flags.reasoningEffort); err != nil {
		return contract.CreateAgentRequest{}, err
	}
	workspace, err := commandWorkspaceFlag(cmd)
	if err != nil {
		return contract.CreateAgentRequest{}, err
	}
	scope := contract.AgentCreateScopeGlobal
	if workspace != "" {
		scope = contract.AgentCreateScopeWorkspace
	}
	disabledSkills := normalizeAgentFlagValues(flags.disabledSkills)
	var skills *contract.CreateAgentSkillsConfig
	if len(disabledSkills) > 0 {
		skills = &contract.CreateAgentSkillsConfig{Disabled: disabledSkills}
	}
	return contract.CreateAgentRequest{
		Scope:     scope,
		Workspace: workspace,
		Agent: contract.CreateAgentPayload{
			Name:            name,
			Provider:        strings.TrimSpace(flags.provider),
			Command:         strings.TrimSpace(flags.command),
			Model:           strings.TrimSpace(flags.model),
			ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(flags.reasoningEffort)),
			Tools:           normalizeAgentFlagValues(flags.tools),
			Toolsets:        normalizeAgentFlagValues(flags.toolsets),
			DenyTools:       normalizeAgentFlagValues(flags.denyTools),
			Permissions:     contract.SettingsPermissionMode(strings.TrimSpace(flags.permissions)),
			CategoryPath:    normalizeAgentFlagValues(flags.categoryPath),
			Skills:          skills,
			Prompt:          prompt,
		},
	}, nil
}

func updateAgentRequestFromFlags(
	cmd *cobra.Command,
	client DaemonClient,
	name string,
	workspace string,
	expectedDigest string,
	flags agentDefinitionFlags,
) (contract.UpdateAgentRequest, error) {
	name, err := normalizePublicAgentName(name)
	if err != nil {
		return contract.UpdateAgentRequest{}, err
	}
	expectedDigest = strings.TrimSpace(expectedDigest)
	if expectedDigest == "" {
		return contract.UpdateAgentRequest{}, errors.New("cli: --expected-digest is required")
	}
	current, err := client.GetAgent(cmd.Context(), name, AgentQuery{Workspace: workspace})
	if err != nil {
		return contract.UpdateAgentRequest{}, err
	}
	payload := createAgentPayloadFromRecord(current)
	if err := applyAgentDefinitionOverrides(cmd, &payload, flags); err != nil {
		return contract.UpdateAgentRequest{}, err
	}
	payload.Name = name
	return contract.UpdateAgentRequest{
		Workspace:      workspace,
		Agent:          payload,
		ExpectedDigest: expectedDigest,
	}, nil
}

func duplicateAgentRequestFromFlags(
	cmd *cobra.Command,
	name string,
	scope string,
	flags agentDefinitionFlags,
) (contract.DuplicateAgentRequest, error) {
	name, err := normalizePublicAgentName(name)
	if err != nil {
		return contract.DuplicateAgentRequest{}, err
	}
	workspace, err := commandWorkspaceFlag(cmd)
	if err != nil {
		return contract.DuplicateAgentRequest{}, err
	}
	createScope, err := parseAgentCreateScope(scope)
	if err != nil {
		return contract.DuplicateAgentRequest{}, err
	}
	if createScope == contract.AgentCreateScopeWorkspace && workspace == "" {
		return contract.DuplicateAgentRequest{}, errors.New("cli: --workspace is required for workspace scope")
	}
	overrides, err := duplicateAgentOverridesFromFlags(cmd, flags)
	if err != nil {
		return contract.DuplicateAgentRequest{}, err
	}
	return contract.DuplicateAgentRequest{
		Name:      name,
		Scope:     createScope,
		Workspace: workspace,
		Overrides: overrides,
	}, nil
}

func createAgentPayloadFromRecord(agent AgentRecord) contract.CreateAgentPayload {
	return contract.CreateAgentPayload{
		Name:            agent.Name,
		Provider:        agent.Provider,
		Command:         agent.Command,
		Model:           agent.Model,
		ReasoningEffort: agent.ReasoningEffort,
		Tools:           cloneStrings(agent.Tools),
		Toolsets:        cloneStrings(agent.Toolsets),
		DenyTools:       cloneStrings(agent.DenyTools),
		Permissions:     contract.SettingsPermissionMode(agent.Permissions),
		CategoryPath:    cloneStrings(agent.CategoryPath),
		Skills:          cloneAgentSkills(agent.Skills),
		Prompt:          agent.Prompt,
	}
}

func applyAgentDefinitionOverrides(
	cmd *cobra.Command,
	payload *contract.CreateAgentPayload,
	flags agentDefinitionFlags,
) error {
	if cmd.Flags().Changed(cliProviderKey) {
		payload.Provider = strings.TrimSpace(flags.provider)
	}
	if cmd.Flags().Changed(agentCommandKey) {
		payload.Command = strings.TrimSpace(flags.command)
	}
	if cmd.Flags().Changed(agentModelKey) {
		payload.Model = strings.TrimSpace(flags.model)
	}
	if cmd.Flags().Changed(agentReasoningEffortKey) {
		if err := validateAgentReasoningEffort(flags.reasoningEffort); err != nil {
			return err
		}
		payload.ReasoningEffort = contract.ReasoningEffort(strings.TrimSpace(flags.reasoningEffort))
	}
	if cmd.Flags().Changed("tool") {
		payload.Tools = normalizeAgentFlagValues(flags.tools)
	}
	if cmd.Flags().Changed("toolset") {
		payload.Toolsets = normalizeAgentFlagValues(flags.toolsets)
	}
	if cmd.Flags().Changed("deny-tool") {
		payload.DenyTools = normalizeAgentFlagValues(flags.denyTools)
	}
	if cmd.Flags().Changed(configPermissionsKey) {
		payload.Permissions = contract.SettingsPermissionMode(strings.TrimSpace(flags.permissions))
	}
	if cmd.Flags().Changed(agentCategoryKey) {
		payload.CategoryPath = normalizeAgentFlagValues(flags.categoryPath)
	}
	if cmd.Flags().Changed(agentDisableSkillFlag) {
		payload.Skills = &contract.CreateAgentSkillsConfig{
			Disabled: normalizeAgentFlagValues(flags.disabledSkills),
		}
	}
	prompt, changed, err := agentPromptFromFlags(cmd, flags, false)
	if err != nil {
		return err
	}
	if changed {
		payload.Prompt = prompt
	}
	return nil
}

func duplicateAgentOverridesFromFlags(
	cmd *cobra.Command,
	flags agentDefinitionFlags,
) (*contract.DuplicateAgentOverrides, error) {
	var payload contract.CreateAgentPayload
	if err := applyAgentDefinitionOverrides(cmd, &payload, flags); err != nil {
		return nil, err
	}
	changed := false
	for _, name := range []string{
		cliProviderKey, agentCommandKey, agentModelKey, agentReasoningEffortKey,
		clientToolsPromptKey, "prompt-file", toolToolKey, "toolset", "deny-tool", configPermissionsKey, agentCategoryKey,
		agentDisableSkillFlag,
	} {
		changed = changed || cmd.Flags().Changed(name)
	}
	if !changed {
		return nil, nil
	}
	return &contract.DuplicateAgentOverrides{
		Provider:        payload.Provider,
		Command:         payload.Command,
		Model:           payload.Model,
		ReasoningEffort: payload.ReasoningEffort,
		Tools:           payload.Tools,
		Toolsets:        payload.Toolsets,
		DenyTools:       payload.DenyTools,
		Permissions:     payload.Permissions,
		CategoryPath:    payload.CategoryPath,
		Skills:          payload.Skills,
		Prompt:          payload.Prompt,
	}, nil
}

func agentPromptFromFlags(
	cmd *cobra.Command,
	flags agentDefinitionFlags,
	required bool,
) (string, bool, error) {
	promptChanged := cmd.Flags().Changed("prompt")
	fileChanged := cmd.Flags().Changed("prompt-file")
	if promptChanged && fileChanged {
		return "", false, errors.New("cli: use either --prompt or --prompt-file, not both")
	}
	if !promptChanged && !fileChanged {
		if required {
			return "", false, errors.New("cli: --prompt or --prompt-file is required")
		}
		return "", false, nil
	}
	prompt := strings.TrimSpace(flags.prompt)
	if fileChanged {
		path := strings.TrimSpace(flags.promptFile)
		if path == "" {
			return "", false, errors.New("cli: --prompt-file cannot be empty")
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("cli: read prompt file %q: %w", path, err)
		}
		prompt = strings.TrimSpace(string(contents))
	}
	if prompt == "" {
		return "", false, errors.New("cli: agent prompt cannot be empty")
	}
	return prompt, true, nil
}
