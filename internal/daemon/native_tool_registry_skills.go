package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	resourceScopeUserKind      = "user"
	resourceScopeWorkspaceKind = "workspace"
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
	offset, limit, err := normalizeNativeToolListInput(req.ToolID, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	views, err := n.registry().List(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	page := nativeToolListPageFromViews(views, offset, limit)
	return structuredResult(
		page,
		fmt.Sprintf("%d of %d tools", page.Count, page.Total),
	)
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
		if file != "" {
			return toolspkg.ToolResult{}, skillViewResourceError(req.ToolID, file, err)
		}
		if errors.Is(err, skillspkg.ErrInvalidDefinition) {
			return toolspkg.ToolResult{}, skillViewDefinitionError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, err
	}
	payload := map[string]any{
		"skill":               skillViewPayloadWithoutExposure(skill),
		nativeToolsContentKey: content,
	}
	resourceScope := skill.ResourceScope.Normalize()
	if resourceScope.Kind == resourceScopeUserKind || resourceScope.Kind == resourceScopeWorkspaceKind {
		skillPayload, exposureErr := n.skillViewPayload(ctx, scope, input, skill)
		if exposureErr != nil {
			return toolspkg.ToolResult{}, exposureErr
		}
		payload["skill"] = skillPayload
	}
	if commandID := strings.TrimSpace(input.CommandID); commandID != "" {
		payload["command_id"] = commandID
		exposures := []contract.SkillExposurePayload{}
		if projected, ok := payload["skill"].(contract.SkillPayload); ok && projected.Exposures != nil {
			exposures = append(exposures, (*projected.Exposures)...)
		}
		payload["skill"] = sourceQualifiedSkillViewPayload(skill, exposures)
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

func skillViewResourceError(id toolspkg.ToolID, file string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		message := fmt.Sprintf("skill resource %q not found", file)
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			message,
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonSkillResourceNotFound,
		)
	case errors.Is(err, skillspkg.ErrResourcePathRequired),
		errors.Is(err, skillspkg.ErrResourcePathRelative),
		errors.Is(err, skillspkg.ErrResourcePathOutside):
		return nativeCommandInvalidInputError(id, fmt.Sprintf("skill resource path %q is invalid", file))
	default:
		return fmt.Errorf("daemon: load skill resource %q: %w", file, err)
	}
}

func skillViewDefinitionError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewOperatorToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"skill definition is invalid",
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		err.Error(),
		"Fix the SKILL.md YAML frontmatter and retry skill_view.",
		toolspkg.ReasonSkillDefinitionInvalid,
	)
}

func sourceQualifiedSkillViewPayload(
	skill *skillspkg.Skill,
	exposures []contract.SkillExposurePayload,
) map[string]any {
	if skill == nil {
		return map[string]any{}
	}
	projectedExposures := make([]contract.SkillExposurePayload, len(exposures))
	copy(projectedExposures, exposures)
	owner := core.SkillPayloadFromSkill(skill)
	return map[string]any{
		bootNameKey:   skill.Meta.Name,
		"description": skill.Meta.Description,
		"version":     skill.Meta.Version,
		bootSourceKey: skillspkg.CommandSourceForSkill(skill),
		"origin":      strings.TrimSpace(skill.Origin),
		"owner_scope": owner.OwnerScope,
		"owner_id":    owner.OwnerID,
		"exposures":   projectedExposures,
		"enabled":     skill.Enabled,
	}
}

func skillViewPayloadWithoutExposure(skill *skillspkg.Skill) contract.SkillPayload {
	payload := core.SkillPayloadFromSkill(skill)
	exposures := []contract.SkillExposurePayload{}
	payload.Exposures = &exposures
	return payload
}

func (n *daemonNativeTools) skillViewPayload(
	ctx context.Context,
	scope toolspkg.Scope,
	input skillViewInput,
	skill *skillspkg.Skill,
) (contract.SkillPayload, error) {
	payload := core.SkillPayloadFromSkill(skill)
	if n == nil || n.deps == nil || n.deps.SkillExposures == nil {
		return payload, errors.New("daemon: skill exposure repository is required")
	}
	provider, ok := n.deps.Skills.(core.SkillExposureRootsProvider)
	if !ok {
		return payload, errors.New("daemon: skill exposure roots are required")
	}
	var resolved *workspacepkg.ResolvedWorkspace
	workspaceRef := nativeCallerWorkspaceInput(input.WorkspaceID, scope)
	if workspaceRef != "" {
		if n.deps.WorkspaceResolver == nil {
			return payload, errors.New("daemon: workspace resolver is required for workspace skill exposure")
		}
		workspace, err := n.deps.WorkspaceResolver.Resolve(ctx, workspaceRef)
		if err != nil {
			return payload, err
		}
		resolved = &workspace
	}
	logger := n.deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	options := []skillspkg.ExposeManagerOption{skillspkg.WithExposureLogger(logger)}
	if n.deps.SkillExposureEvents != nil {
		options = append(options, skillspkg.WithExposureEventStore(n.deps.SkillExposureEvents))
	}
	manager := skillspkg.NewExposeManager(n.deps.SkillExposures, provider.ExposureRoots(resolved), options...)
	states, err := manager.Exposures(ctx, skill)
	if err != nil {
		return payload, err
	}
	exposures := core.SkillExposurePayloadsFromDomain(states)
	payload.Exposures = &exposures
	return payload, nil
}

// resolveSkillViewTarget resolves the skill a skill_view call targets: a workspace-fenced skill
// by name, or a source-qualified skill by command_id within one session's command catalog. The
// command_id path uses the same concrete session-agent snapshot and lazy catalog fallback as the
// session command service, preserving workspace fencing and authored-agent validation.
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
		func() session.AgentResolver { return n.deps.AgentResolver },
		func() promptSkillsWorkspaceResolver { return n.deps.WorkspaceResolver },
		n.deps.Profiles,
	)
	_, byCommandID, err := service.project(ctx, info, agent)
	if err != nil {
		return nil, err
	}
	candidate, found := byCommandID[commandID]
	if !found || candidate.Skill == nil {
		return nil, nativeCommandNotFoundError(
			toolspkg.ToolIDSkillView,
			fmt.Sprintf("skill command %q not found", commandID),
		)
	}
	return candidate.Skill, nil
}
