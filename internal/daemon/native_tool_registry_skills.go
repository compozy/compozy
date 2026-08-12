package daemon

import (
	"context"
	"fmt"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	skillspkg "github.com/compozy/compozy/internal/skills"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) toolList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input toolListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	views, err := n.registry().List(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	views = limitToolViews(views, input.Limit)
	return structuredResult(map[string]any{"tools": views}, fmt.Sprintf("%d tools", len(views)))
}

func (n *daemonNativeTools) toolSearch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input toolSearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	registry, ok := n.registry().(nativeToolDiagnosticRegistry)
	if !ok {
		return toolspkg.ToolResult{}, unavailableToolDiagnosticRegistryError(req.ToolID)
	}
	views, err := registry.DiagnosticSearch(ctx, scope, toolspkg.SearchQuery{
		Query: input.Query,
		Limit: input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"tools": views}, fmt.Sprintf("%d tools", len(views)))
}

func (n *daemonNativeTools) toolInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input toolInfoInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	id := toolspkg.ToolID(strings.TrimSpace(input.ToolID))
	registry, ok := n.registry().(nativeToolDiagnosticRegistry)
	if !ok {
		return toolspkg.ToolResult{}, unavailableToolDiagnosticRegistryError(req.ToolID)
	}
	view, err := registry.DiagnosticGet(ctx, scope, id)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"tool": view}, view.Descriptor.ID.String())
}

func (n *daemonNativeTools) skillList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input skillListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	skillList, err := n.skillsFor(ctx, scope, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SkillPayloadsFromSkills(limitSkills(skillList, input.Limit))
	return structuredResult(
		map[string]any{nativeToolsSkillsKey: payload},
		fmt.Sprintf("%d skills", len(payload)),
	)
}

func (n *daemonNativeTools) skillSearch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input skillSearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	skillList, err := n.skillsFor(ctx, scope, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	filtered := searchSkills(skillList, input.Query)
	payload := core.SkillPayloadsFromSkills(limitSkills(filtered, input.Limit))
	return structuredResult(
		map[string]any{nativeToolsSkillsKey: payload},
		fmt.Sprintf("%d skills", len(payload)),
	)
}

func (n *daemonNativeTools) skillView(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input skillViewInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	skill, err := n.resolveSkillViewTarget(ctx, scope, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	file := strings.TrimSpace(input.File)
	var content string
	if file != "" {
		content, err = n.deps.Skills.LoadResource(ctx, skill, file)
	} else {
		content, err = n.deps.Skills.LoadContent(ctx, skill)
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := map[string]any{
		"skill":   core.SkillPayloadFromSkill(skill),
		"content": content,
	}
	if commandID := strings.TrimSpace(input.CommandID); commandID != "" {
		payload["command_id"] = commandID
		payload["skill"] = sourceQualifiedSkillViewPayload(skill)
	}
	if file != "" {
		payload["file"] = file
	}
	result, err := structuredResult(payload, content)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result.Content = []toolspkg.ToolContent{{Type: nativeToolsTextKey, Text: content}}
	return result, nil
}

func sourceQualifiedSkillViewPayload(skill *skillspkg.Skill) map[string]any {
	if skill == nil {
		return map[string]any{}
	}
	source := skillspkg.CommandSourceForSkill(skill)
	return map[string]any{
		bootNameKey:   skill.Meta.Name,
		"description": skill.Meta.Description,
		"version":     skill.Meta.Version,
		bootSourceKey: source,
		"enabled":     skill.Enabled,
	}
}

func (n *daemonNativeTools) resolveSkillViewTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	input skillViewInput,
) (*skillspkg.Skill, error) {
	commandID := strings.TrimSpace(input.CommandID)
	name := strings.TrimSpace(input.Name)
	if (commandID == "") == (name == "") {
		return nil, nativeCommandInvalidInputError(
			toolspkg.ToolIDSkillView,
			"exactly one of skill name or command_id is required",
		)
	}
	if commandID == "" {
		return n.resolveSkill(ctx, scope, input.WorkspaceID, name)
	}
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return nil, nativeCommandInvalidInputError(
			toolspkg.ToolIDSkillView,
			"command_id skill view requires a session-scoped caller",
		)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, toolspkg.ToolIDSkillView, input.WorkspaceID, scope)
	if err != nil {
		return nil, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return nil, nativeNetworkInputError(toolspkg.ToolIDSkillView, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, toolspkg.ToolIDSkillView, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	agent, ok, err := n.sessionAgentDefinition(sessionID)
	if err != nil {
		return nil, nativeCommandDependencyError(
			toolspkg.ToolIDSkillView,
			err.Error(),
		)
	}
	if !ok {
		return nil, nativeCommandDependencyError(
			toolspkg.ToolIDSkillView,
			"command_id skill view requires a concrete session agent",
		)
	}
	registry, ok := n.deps.Skills.(sessionCommandSkills)
	if !ok {
		return nil, nativeCommandDependencyError(
			toolspkg.ToolIDSkillView,
			"source-qualified skill registry is unavailable",
		)
	}
	service := newSessionCommandService(
		registry,
		func() sessionCommandAgentResolver {
			agentResolver, _ := n.deps.AgentCatalog.(sessionCommandAgentResolver)
			return agentResolver
		},
		func() promptSkillsWorkspaceResolver { return n.deps.WorkspaceResolver },
	)
	_, byCommandID, err := service.project(ctx, info, agent)
	if err != nil {
		return nil, err
	}
	skill := byCommandID[commandID]
	if skill == nil {
		return nil, nativeCommandNotFoundError(
			toolspkg.ToolIDSkillView,
			fmt.Sprintf("skill command %q not found", commandID),
		)
	}
	return skill, nil
}
