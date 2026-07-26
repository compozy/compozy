package session

import (
	"context"
	"errors"
	"fmt"

	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/soul"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (m *Manager) persistResolvedSoul(
	ctx context.Context,
	workspaceID string,
	agentName string,
	resolved *soul.ResolvedSoul,
	config aghconfig.SoulConfig,
	source string,
	now time.Time,
) (*soul.Snapshot, error) {
	if resolved == nil {
		return nil, errors.New("session: resolved soul is required")
	}
	config = sessionSoulConfig(config)
	if !resolved.Present || !resolved.Active {
		return nil, nil
	}
	if !resolved.Valid {
		return nil, fmt.Errorf("session: resolved soul for %q is invalid", agentName)
	}
	if m.soulStore == nil {
		return nil, errors.New("session: soul snapshot store is required")
	}
	provenance, err := soul.NewConfigProvenance(config, source)
	if err != nil {
		return nil, err
	}
	snapshot, err := soul.SnapshotFromResolved(
		newID("soul"),
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(agentName),
		resolved,
		provenance,
		now.UTC(),
	)
	if err != nil {
		return nil, err
	}
	persisted, err := m.soulStore.UpsertSoulSnapshot(ctx, snapshot)
	if err != nil {
		return nil, fmt.Errorf("session: persist soul snapshot for %q: %w", agentName, err)
	}
	return &persisted, nil
}

func (m *Manager) resolveSoulRefreshTarget(
	ctx context.Context,
	info *Info,
) (workspacepkg.ResolvedWorkspace, AgentArtifacts, error) {
	if info == nil {
		return workspacepkg.ResolvedWorkspace{}, AgentArtifacts{}, errors.New("session: session info is required")
	}
	resolvedWorkspace, err := m.resolveSoulRefreshWorkspace(ctx, info)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, AgentArtifacts{}, err
	}
	artifacts, err := m.resolveWorkspaceAgentArtifactsForSession(info.AgentName, info.Type, &resolvedWorkspace)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, AgentArtifacts{}, fmt.Errorf(
			"session: resolve workspace agent %q for soul refresh: %w",
			info.AgentName,
			err,
		)
	}
	return resolvedWorkspace, artifacts, nil
}

func (m *Manager) resolveSoulRefreshWorkspace(
	ctx context.Context,
	info *Info,
) (workspacepkg.ResolvedWorkspace, error) {
	target := firstNonEmpty(strings.TrimSpace(info.WorkspaceID), strings.TrimSpace(info.Workspace))
	if m.workspace != nil && target != "" {
		resolved, err := m.workspace.Resolve(ctx, target)
		if err != nil {
			return workspacepkg.ResolvedWorkspace{}, fmt.Errorf("session: resolve workspace for soul refresh: %w", err)
		}
		return resolved, nil
	}
	if strings.TrimSpace(info.Workspace) == "" {
		return workspacepkg.ResolvedWorkspace{}, errors.New("session: workspace is required for soul refresh")
	}
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      strings.TrimSpace(info.WorkspaceID),
			RootDir: strings.TrimSpace(info.Workspace),
		},
		Config: aghconfig.DefaultWithHome(m.homePaths),
	}, nil
}

func (m *Manager) hasActiveSoulRun(ctx context.Context, sessionID string, now time.Time) (bool, error) {
	if m == nil || m.soulRunChecker == nil {
		return false, nil
	}
	active, err := m.soulRunChecker.HasActiveRunForSession(ctx, strings.TrimSpace(sessionID), now.UTC())
	if err != nil {
		return false, fmt.Errorf("session: check active task runs for soul refresh: %w", err)
	}
	return active, nil
}

func sessionSoulConfig(config aghconfig.SoulConfig) aghconfig.SoulConfig {
	if config == (aghconfig.SoulConfig{}) {
		return aghconfig.DefaultSoulConfig()
	}
	return config
}

func (s *sessionStartSpec) applySoulSnapshot(snapshot *soul.Snapshot) {
	if s == nil {
		return
	}
	s.soulSnapshot = cloneSoulSnapshotPointer(snapshot)
	s.soulSnapshotID = ""
	s.soulDigest = ""
	if snapshot != nil {
		s.soulSnapshotID = strings.TrimSpace(snapshot.ID)
		s.soulDigest = strings.TrimSpace(snapshot.Digest)
	}
}
