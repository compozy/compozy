package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
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
	reviews := provider.SkillReviews()
	api.SkillProposals = &agentinstance.SkillProposalService{
		Store: reviews, Evidence: reviews, Authority: api.Authority,
		DraftRoot: filepath.Join(d.homePaths.HomeDir, "skill-reviews"),
		HomeID:    skillInstallHomeID(d.homePaths.HomeDir),
		Publisher: skillCatalogPublisher{state: state},
		Locks:     agentinstance.NewSkillInstallLocks(),
	}
	api.SkillTarget = func(ctx context.Context, instance agentinstance.Instance) (string, []string, error) {
		resolved, target, err := definitions.skillTargetRoot(ctx, instance)
		if err != nil {
			return "", nil, err
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

// reconcileSkillInstalls runs the skill install Reconcile for every target root of this home's
// instances. Failures are logged per root and never block startup; a retained conflict stays
// blocked until the operator resolves it.
func reconcileSkillInstalls(ctx context.Context, state *bootState, definitions instanceDefinitions) {
	api := state.agentInstances
	seen := map[string]bool{}
	query := agentinstance.ListQuery{AllProfiles: true, AllWorkspaces: true, Limit: 100}
	for {
		page, err := api.Store.List(ctx, query)
		if err != nil {
			state.logger.ErrorContext(ctx, "daemon: list instances for skill install reconcile", "error", err)
			return
		}
		for _, instance := range page {
			_, root, err := definitions.skillTargetRoot(ctx, instance)
			if err != nil || seen[root] {
				continue
			}
			seen[root] = true
			if err := api.SkillProposals.ReconcileTargetRoot(ctx, root); err != nil {
				state.logger.ErrorContext(ctx, "daemon: reconcile skill installs at startup",
					"target_root", root, "error", err)
			}
		}
		if len(page) < query.Limit {
			return
		}
		query.AfterID = page[len(page)-1].ID
	}
}

// skillTargetRoot is the root an instance installs approved skills into: the workspace skills root,
// or the definition's agent-local skills directory for a global instance.
func (r instanceDefinitions) skillTargetRoot(
	ctx context.Context,
	instance agentinstance.Instance,
) (workspacepkg.ResolvedWorkspace, string, error) {
	resolved, err := r.resolveWorkspace(ctx, instance.ProfileID, instance.WorkspaceID)
	if err != nil {
		return resolved, "", err
	}
	agent, err := r.agents.ResolveAgent(instance.AgentName, &resolved)
	if err != nil {
		return resolved, "", err
	}
	target := filepath.Join(resolved.RootDir, compozyconfig.DirName, compozyconfig.SkillsDirName)
	if instance.WorkspaceID == "" {
		if !filepath.IsAbs(agent.SourcePath) {
			return resolved, "", agentinstance.ErrSkillReviewNoTarget
		}
		target = filepath.Join(filepath.Dir(agent.SourcePath), compozyconfig.SkillsDirName)
	}
	return resolved, target, nil
}

// skillCatalogPublisher refreshes the skill catalog after a committed skill change.
type skillCatalogPublisher struct{ state *bootState }

var _ agentinstance.SkillPublisher = skillCatalogPublisher{}

func (p skillCatalogPublisher) SkillPublished(ctx context.Context) error {
	if p.state.agentSkillResources == nil {
		return errors.New("skill resource publication unavailable")
	}
	return p.state.agentSkillResources.SyncSkills(ctx)
}

// skillInstallHomeID is the SHA-256 of the canonical home directory, written into install-log
// entries so each home projects only its own reviews.
func skillInstallHomeID(homeDir string) string {
	if canonical, err := fileutil.CanonicalPathWithExistingPrefix(homeDir); err == nil {
		homeDir = canonical
	}
	sum := sha256.Sum256([]byte(homeDir))
	return hex.EncodeToString(sum[:])
}
