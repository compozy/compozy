package daemon

import (
	"context"

	"fmt"

	"strings"

	core "github.com/compozy/agh/internal/api/core"

	toolspkg "github.com/compozy/agh/internal/tools"
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
	skillList, err := n.skillsFor(ctx, scope, req.ToolID, input.WorkspaceID)
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
	skillList, err := n.skillsFor(ctx, scope, req.ToolID, input.WorkspaceID)
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
	skill, err := n.resolveSkill(ctx, scope, req.ToolID, input.WorkspaceID, input.Name)
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
