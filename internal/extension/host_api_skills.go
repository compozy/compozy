package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	skillspkg "github.com/compozy/agh/internal/skills"
)

func (h *HostAPIHandler) handleSkillsList(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.skills == nil {
		return nil, errors.New("extension: skills registry is not configured")
	}

	var params hostAPISkillsListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	var (
		skills []*skillspkg.Skill
		err    error
	)
	if workspaceRef := strings.TrimSpace(params.Workspace); workspaceRef != "" {
		if h.workspaces == nil {
			return nil, errors.New("extension: workspace resolver is not configured")
		}
		resolved, resolveErr := h.workspaces.Resolve(ctx, workspaceRef)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if agentName := strings.TrimSpace(params.ForAgent); agentName != "" {
			skills, err = h.skills.ForAgent(ctx, &resolved, agentName)
		} else {
			skills, err = h.skills.ForWorkspace(ctx, &resolved)
		}
	} else if agentName := strings.TrimSpace(params.ForAgent); agentName != "" {
		skills, err = h.skills.ForAgent(ctx, nil, agentName)
	} else {
		skills, err = h.skills.ForWorkspace(ctx, nil)
	}
	if err != nil {
		return nil, err
	}

	result := make([]hostAPISkillSummary, 0, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		result = append(result, hostAPISkillSummary{
			Name:        skill.Meta.Name,
			Description: skill.Meta.Description,
			Source:      skillspkg.SkillSourceName(skill.Source),
			Enabled:     skill.Enabled,
			Activation:  hostAPISkillActivation(skill),
		})
	}
	return result, nil
}

func hostAPISkillActivation(skill *skillspkg.Skill) apicontract.SkillActivationPayload {
	if skill == nil {
		return apicontract.SkillActivationPayload{}
	}
	reasons := make([]apicontract.SkillActivationReasonPayload, 0, len(skill.Activation.Reasons))
	for _, reason := range skill.Activation.Reasons {
		reasons = append(reasons, apicontract.SkillActivationReasonPayload{
			Gate:    string(reason.Gate),
			Code:    apicontract.SkillActivationReasonCode(reason.Code),
			Missing: append([]string(nil), reason.Missing...),
			Message: reason.Message,
		})
	}
	return apicontract.SkillActivationPayload{
		Active:  !skill.Activation.Evaluated || skill.Activation.Active,
		Reasons: reasons,
	}
}
