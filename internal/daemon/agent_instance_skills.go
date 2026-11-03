package daemon

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (r instanceDefinitions) resolveWorkspace(
	ctx context.Context,
	profileID, workspaceID string,
) (workspacepkg.ResolvedWorkspace, error) {
	if workspaceID == "" {
		return r.workspaces.ResolveUser(ctx, profileID)
	}
	name, err := r.profiles.ProfileName(ctx, profileID)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	return r.workspaces.ResolveForProfile(ctx, workspaceID, name)
}

func (d *Daemon) bindInstanceSkillReviews(
	state *bootState,
	provider instanceRepositoryProvider,
	definitions instanceDefinitions,
) {
	api := state.agentInstances
	api.SkillPublished = func(ctx context.Context) error {
		if state.agentSkillResources == nil {
			return errors.New("skill resource publication unavailable")
		}
		return state.agentSkillResources.SyncSkills(ctx)
	}
	api.SkillProposals = &agentinstance.SkillProposalService{
		Store: provider.SkillReviews(), Authority: api.Authority,
		DraftRoot: filepath.Join(d.homePaths.HomeDir, "skill-reviews"),
	}
	api.SkillTarget = func(ctx context.Context, instance agentinstance.Instance) (string, []string, error) {
		resolved, err := definitions.resolveWorkspace(ctx, instance.ProfileID, instance.WorkspaceID)
		if err != nil {
			return "", nil, err
		}
		agent, err := definitions.agents.ResolveAgent(instance.AgentName, &resolved)
		if err != nil {
			return "", nil, err
		}
		target := filepath.Join(resolved.RootDir, compozyconfig.DirName, compozyconfig.SkillsDirName)
		if instance.WorkspaceID == "" {
			if !filepath.IsAbs(agent.SourcePath) {
				return "", nil, agentinstance.ErrSkillReviewNoTarget
			}
			target = filepath.Join(filepath.Dir(agent.SourcePath), compozyconfig.SkillsDirName)
		}
		if state.skillsRegistry == nil {
			return "", nil, errors.New("skill source roots unavailable")
		}
		roots, err := state.skillsRegistry.SkillSourceRoots(ctx, &resolved)
		if err != nil {
			return "", nil, err
		}
		scans := []string{target}
		for _, root := range roots {
			scans = append(scans, root.Spec.Dir)
		}
		return target, scans, nil
	}
}
