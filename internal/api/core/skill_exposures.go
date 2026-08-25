package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

const exposureFailedCode = "expose_failed"

type skillExposureOperation struct {
	skill            *skills.Skill
	manager          *skills.ExposeManager
	actor            taskpkg.ActorContext
	workspaceID      string
	configGeneration int64
}

const workspaceScopeValue = "workspace"

// ExposeSkill creates provider-root links for one skill.
func (h *BaseHandlers) ExposeSkill(c *gin.Context) {
	h.handleSkillExposureMutation(c, true)
}

// UnexposeSkill removes provider-root links proven to be owned by CompozyOS.
func (h *BaseHandlers) UnexposeSkill(c *gin.Context) {
	h.handleSkillExposureMutation(c, false)
}

func (h *BaseHandlers) handleSkillExposureMutation(c *gin.Context, expose bool) {
	name := strings.TrimSpace(c.Param("name"))
	var request contract.SkillExposureRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondSkillExposureInputFailure(
			c,
			name,
			request,
			fmt.Errorf("decode skill exposure request: %w", err),
			expose,
		)
		return
	}
	operation, err := h.resolveSkillExposureOperation(c, name, request.WorkspaceID, expose)
	if err != nil {
		h.respondSkillExposureInputFailure(c, name, request, err, expose)
		return
	}
	ctx := skillExposureEventContext(c, operation)
	var results []skills.TargetResult
	if expose {
		results, err = operation.manager.Expose(ctx, operation.skill, request.Targets)
	} else {
		results, err = operation.manager.Unexpose(ctx, operation.skill, request.Targets)
	}
	payloads := skillExposureTargetResultsFromDomain(results)
	if err != nil {
		h.respondSkillExposureFailure(c, operation.skill.Meta.Name, operation.workspaceID, payloads, err, expose)
		return
	}
	if expose {
		c.JSON(http.StatusOK, contract.SkillExposeResponse{
			Name: operation.skill.Meta.Name, WorkspaceID: operation.workspaceID,
			Results: payloads, RolledBack: false,
		})
		return
	}
	c.JSON(http.StatusOK, contract.SkillUnexposeResponse{
		Name: operation.skill.Meta.Name, WorkspaceID: operation.workspaceID, Results: payloads,
	})
}

func (h *BaseHandlers) resolveSkillExposureOperation(
	c *gin.Context,
	name string,
	requestedWorkspaceID string,
	expose bool,
) (skillExposureOperation, error) {
	if err := h.prepareSkillExposure(c, name); err != nil {
		return skillExposureOperation{}, err
	}
	action := "skills.unexpose"
	if expose {
		action = "skills.expose"
	}
	actor, err := h.taskActorContext(c, action)
	if err != nil {
		return skillExposureOperation{}, err
	}

	resolved, canonicalWorkspaceID, err := h.resolveSkillExposureScope(c, requestedWorkspaceID, actor)
	if err != nil {
		return skillExposureOperation{}, err
	}

	agentName, err := skillAgentScope(c)
	if err != nil {
		return skillExposureOperation{}, err
	}
	skillList, err := h.resolveScopedSkills(c, resolved, agentName)
	if err != nil {
		return skillExposureOperation{}, err
	}
	skill := findSkillByName(skillList, name)
	if skill == nil {
		return skillExposureOperation{}, fmt.Errorf("%w: %q", ErrSkillNotFound, name)
	}
	responseWorkspaceID := skillExposureResponseWorkspaceID(skill, canonicalWorkspaceID)
	manager, err := h.newSkillExposureManager(resolved)
	if err != nil {
		return skillExposureOperation{}, err
	}
	return skillExposureOperation{
		skill: skill, actor: actor, workspaceID: responseWorkspaceID,
		configGeneration: skillExposureConfigGeneration(h.SkillsRegistry),
		manager:          manager,
	}, nil
}

func (h *BaseHandlers) prepareSkillExposure(c *gin.Context, name string) error {
	if h == nil || h.SkillsRegistry == nil {
		return errors.New("skills registry is not configured")
	}
	if h.SkillExposures == nil {
		return errors.New("skill exposure repository is not configured")
	}
	if name == "" {
		return fmt.Errorf("%w: skill name is required", ErrSkillValidation)
	}
	if h.SkillResources == nil {
		return nil
	}
	if err := h.SkillResources.SyncSkills(c.Request.Context()); err != nil {
		return fmt.Errorf("sync skill resources before exposure: %w", err)
	}
	return nil
}

func (h *BaseHandlers) resolveSkillExposureScope(
	c *gin.Context,
	requestedWorkspaceID string,
	actor taskpkg.ActorContext,
) (*workspacepkg.ResolvedWorkspace, string, error) {
	requestedWorkspaceID = strings.TrimSpace(requestedWorkspaceID)
	if requestedWorkspaceID != "" {
		resolved, err := h.resolveSkillExposureWorkspace(c, requestedWorkspaceID, actor)
		if err != nil {
			return nil, "", err
		}
		canonicalID := canonicalResolvedWorkspaceID(&resolved)
		if canonicalID != requestedWorkspaceID {
			return nil, "", fmt.Errorf(
				"%w: workspace_id must be the canonical workspace id", ErrSkillValidation,
			)
		}
		return &resolved, canonicalID, nil
	}
	profileName, err := h.agentResourceProfileNameForScope(c.Request.Context(), actor.ReadScope)
	if err != nil {
		return nil, "", err
	}
	if profileName != "" && profileName != compozyconfig.DefaultProfileDirName {
		return h.profileOnlySkillScope(profileName), "", nil
	}
	return nil, "", nil
}

func skillExposureResponseWorkspaceID(skill *skills.Skill, fallback string) string {
	if skill == nil || skill.ResourceScope.Normalize().Kind != workspaceScopeValue {
		return ""
	}
	if workspaceID := strings.TrimSpace(skill.ResourceScope.ID); workspaceID != "" {
		return workspaceID
	}
	return fallback
}

func skillExposureConfigGeneration(registry SkillsRegistry) int64 {
	provider, ok := registry.(interface{ ConfigGeneration() int64 })
	if !ok {
		return 0
	}
	return provider.ConfigGeneration()
}

func (h *BaseHandlers) newSkillExposureManager(
	resolved *workspacepkg.ResolvedWorkspace,
) (*skills.ExposeManager, error) {
	if h == nil || h.SkillExposures == nil {
		return nil, errors.New("skill exposure repository is not configured")
	}
	rootsProvider, ok := h.SkillsRegistry.(SkillExposureRootsProvider)
	if !ok {
		return nil, errors.New("skill exposure roots are not configured")
	}
	options := []skills.ExposeManagerOption{skills.WithExposureLogger(h.Logger)}
	if h.SkillExposureEvents != nil {
		options = append(options, skills.WithExposureEventStore(h.SkillExposureEvents))
	}
	return skills.NewExposeManager(h.SkillExposures, rootsProvider.ExposureRoots(resolved), options...), nil
}

func (h *BaseHandlers) resolveSkillExposureWorkspace(
	c *gin.Context,
	workspaceID string,
	actor taskpkg.ActorContext,
) (workspacepkg.ResolvedWorkspace, error) {
	if h.Workspaces == nil {
		return workspacepkg.ResolvedWorkspace{}, errors.New("workspace resolver is not configured")
	}
	profileName, err := h.agentResourceProfileNameForScope(c.Request.Context(), actor.ReadScope)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	return resolveWorkspaceAgentProfile(c.Request.Context(), h.Workspaces, workspaceID, profileName)
}

// canonicalResolvedWorkspaceID returns the workspace id public callers hand out and receive back.
// That is the registered `ws_` id (ResolvedWorkspace.ID) — ResolvedWorkspace.WorkspaceID carries the
// durable identity stamped in <root>/.compozy/workspace.toml, which no public surface ever emits, so
// comparing a caller's `workspace_id` against it can never match. An unregistered workspace resolved
// by path has no registered id, and there the durable identity is the only identity available.
func canonicalResolvedWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	if workspaceID := strings.TrimSpace(resolved.ID); workspaceID != "" {
		return workspaceID
	}
	return strings.TrimSpace(resolved.WorkspaceID)
}

func findSkillByName(skillList []*skills.Skill, name string) *skills.Skill {
	for _, skill := range skillList {
		if skill != nil && skill.Meta.Name == name {
			return skill
		}
	}
	return nil
}

func skillExposureEventContext(c *gin.Context, operation skillExposureOperation) context.Context {
	ctx := skills.WithSourceEventCorrelation(c.Request.Context(), skills.SourceEventCorrelation{
		Scope:       string(operation.skill.ResourceScope.Normalize().Kind),
		ProfileID:   strings.TrimSpace(operation.actor.ReadScope.ProfileID),
		WorkspaceID: operation.workspaceID,
		ActorKind:   string(operation.actor.Actor.Kind.Normalize()),
		ActorID:     strings.TrimSpace(operation.actor.Actor.Ref),
	})
	ctx = skills.WithConfigGeneration(ctx, operation.configGeneration)
	return ctx
}

func (h *BaseHandlers) skillExposureInspectionContext(
	c *gin.Context,
	skill *skills.Skill,
) (context.Context, error) {
	actor, err := h.taskActorContext(c, "skills.get")
	if err != nil {
		return nil, err
	}
	workspaceID := ""
	if skill != nil && skill.ResourceScope.Normalize().Kind == workspaceScopeValue {
		workspaceID = strings.TrimSpace(skill.ResourceScope.ID)
	}
	generation := int64(0)
	if provider, ok := h.SkillsRegistry.(interface{ ConfigGeneration() int64 }); ok {
		generation = provider.ConfigGeneration()
	}
	return skillExposureEventContext(c, skillExposureOperation{
		skill: skill, actor: actor, workspaceID: workspaceID, configGeneration: generation,
	}), nil
}

func skillExposureTargetResultsFromDomain(results []skills.TargetResult) []contract.SkillExposureTargetResultPayload {
	payloads := make([]contract.SkillExposureTargetResultPayload, 0, len(results))
	for _, result := range results {
		payload := contract.SkillExposureTargetResultPayload{Target: result.Target, OK: result.OK}
		if result.Exposure != nil {
			exposure := SkillExposurePayloadsFromDomain([]skills.ExposureState{*result.Exposure})
			if len(exposure) == 1 {
				payload.Exposure = &exposure[0]
			}
		}
		payload.Error = skillExposureErrorPayload(result.Err)
		payload.CleanupError = skillExposureErrorPayload(result.CleanupErr)
		payloads = append(payloads, payload)
	}
	return payloads
}

func skillExposureErrorPayload(err error) *contract.SkillExposureErrorPayload {
	if err == nil {
		return nil
	}
	payload := &contract.SkillExposureErrorPayload{Code: skills.ExposureCodeSkillNotExposable, Message: err.Error()}
	var exposureErr *skills.ExposureError
	if errors.As(err, &exposureErr) {
		payload.Code = exposureErr.Code
		payload.Message = exposureErr.Message
		if exposureErr.Code == skills.ExposureCodeNameConflict || exposureErr.Code == skills.ExposureCodeForeignLink {
			payload.OccupiedBy = strings.TrimSpace(exposureErr.Path)
		}
	}
	return payload
}

func (h *BaseHandlers) respondSkillExposureInputFailure(
	c *gin.Context,
	name string,
	request contract.SkillExposureRequest,
	err error,
	expose bool,
) {
	results := make([]contract.SkillExposureTargetResultPayload, 0, len(request.Targets))
	for _, target := range request.Targets {
		results = append(results, contract.SkillExposureTargetResultPayload{
			Target: strings.TrimSpace(target),
			Error: &contract.SkillExposureErrorPayload{
				Code: skills.ExposureCodeTargetInvalid, Message: err.Error(),
			},
		})
	}
	if len(results) == 0 {
		results = append(results, contract.SkillExposureTargetResultPayload{
			Error: &contract.SkillExposureErrorPayload{Code: skills.ExposureCodeTargetInvalid, Message: err.Error()},
		})
	}
	h.respondSkillExposureFailure(c, name, strings.TrimSpace(request.WorkspaceID), results, err, expose)
}

func (h *BaseHandlers) respondSkillExposureFailure(
	c *gin.Context,
	name string,
	workspaceID string,
	results []contract.SkillExposureTargetResultPayload,
	err error,
	expose bool,
) {
	rolledBack := false
	var batchErr *skills.ExposureBatchError
	if errors.As(err, &batchErr) {
		rolledBack = batchErr.RolledBack
	}
	failureCount := 0
	for _, result := range results {
		if !result.OK && (result.Error == nil ||
			(result.Error.Code != skills.ExposureCodeRolledBack && result.Error.Code != skills.ExposureCodeNotApplied)) {
			failureCount++
		}
	}
	message := fmt.Sprintf("%d of %d targets failed", failureCount, len(results))
	if rolledBack {
		message += "; completed targets rolled back"
	}
	payload := contract.SkillExposureFailureResponse{
		Error: contract.SkillExposureFailureErrorPayload{Code: exposureFailedCode, Message: message},
		Name:  strings.TrimSpace(name), WorkspaceID: strings.TrimSpace(workspaceID), Results: results,
	}
	if expose {
		payload.RolledBack = &rolledBack
	}
	c.JSON(http.StatusConflict, payload)
}
