package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	skillspkg "github.com/compozy/agh/internal/skills"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *BaseHandlers) skillRuntimeStatusPayload(
	ctx context.Context,
	workspaceRef string,
) (contract.SkillRuntimeStatusPayload, error) {
	if h.SkillsRegistry == nil {
		return contract.SkillRuntimeStatusPayload{RuntimeAvailable: false}, nil
	}
	resolved, err := h.resolveSkillStatusWorkspace(ctx, workspaceRef)
	if err != nil {
		return contract.SkillRuntimeStatusPayload{}, err
	}
	skills, err := h.SkillsRegistry.ForWorkspace(ctx, resolved)
	if err != nil {
		return contract.SkillRuntimeStatusPayload{}, err
	}
	payload := contract.SkillRuntimeStatusPayload{
		RuntimeAvailable: true,
		DiscoveredCount:  len(skills),
	}
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		if !skill.Enabled {
			payload.DisabledCount++
		}
		payload.Diagnostics = append(
			payload.Diagnostics,
			SkillDiagnosticPayloadsFromDiagnostics(skillspkg.DiagnosticsForSkill(skill))...,
		)
	}
	return payload, nil
}

func (h *BaseHandlers) resolveSkillStatusWorkspace(
	ctx context.Context,
	workspaceRef string,
) (*workspacepkg.ResolvedWorkspace, error) {
	workspaceRef = strings.TrimSpace(workspaceRef)
	if workspaceRef == "" {
		return nil, nil
	}
	if h.Workspaces == nil {
		return nil, workspacepkg.ErrWorkspaceResolverUnavailable
	}
	resolved, err := h.Workspaces.Resolve(ctx, workspaceRef)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace for skill status: %w", err)
	}
	if strings.TrimSpace(resolved.WorkspaceID) == "" {
		return nil, errors.New("api: resolved workspace id is required for skill status")
	}
	return &resolved, nil
}
