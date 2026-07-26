package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/agh/internal/soul"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// WithSoulClaimLock runs fn while holding the session Soul lock used by refresh.
func (m *Manager) WithSoulClaimLock(ctx context.Context, sessionID string, fn func() error) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: soul claim lock context is required")
	}
	if fn == nil {
		return errors.New("session: soul claim lock function is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return errors.New("session: session id is required")
	}
	lock := m.sessionSoulLock(target)
	select {
	case <-lock:
		defer releaseSoulLock(lock)
		return fn()
	case <-ctx.Done():
		return fmt.Errorf("session: acquire soul claim lock for %q: %w", target, ctx.Err())
	}
}

func (m *Manager) prepareSessionStartSoul(
	ctx context.Context,
	spec *sessionStartSpec,
	artifacts AgentArtifacts,
	now time.Time,
) error {
	if spec == nil {
		return errors.New("session: start spec is required")
	}
	if spec.startAction == sessionStartActionResume {
		return m.prepareResumeSoul(ctx, spec)
	}

	resolved, err := m.resolveSoul(ctx, artifacts, &spec.workspace)
	if err != nil {
		return fmt.Errorf("session: resolve soul for agent %q: %w", spec.agentName, err)
	}
	snapshot, err := m.persistResolvedSoul(
		ctx,
		spec.workspace.ID,
		spec.agentName,
		&resolved,
		spec.workspace.Config.Agents.Soul,
		"session_start",
		now,
	)
	if err != nil {
		return err
	}
	spec.applySoulSnapshot(snapshot)
	return nil
}

func (m *Manager) prepareResumeSoul(ctx context.Context, spec *sessionStartSpec) error {
	if spec == nil || strings.TrimSpace(spec.soulSnapshotID) == "" {
		return nil
	}
	if m.soulStore == nil {
		return errors.New("session: soul snapshot store is required to resume session soul")
	}
	snapshot, err := m.soulStore.GetSoulSnapshot(ctx, spec.soulSnapshotID)
	if err != nil {
		return fmt.Errorf("session: load soul snapshot %q: %w", spec.soulSnapshotID, err)
	}
	if spec.soulDigest != "" && strings.TrimSpace(snapshot.Digest) != spec.soulDigest {
		return fmt.Errorf("session: soul snapshot %q digest mismatch", spec.soulSnapshotID)
	}
	spec.soulSnapshot = cloneSoulSnapshotPointer(&snapshot)
	return nil
}

func (m *Manager) resolveSoul(
	ctx context.Context,
	artifacts AgentArtifacts,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (soul.ResolvedSoul, error) {
	if workspaceSnapshot == nil {
		return soul.ResolvedSoul{}, errors.New("session: resolved workspace is required for soul")
	}
	config := sessionSoulConfig(workspaceSnapshot.Config.Agents.Soul)
	if strings.TrimSpace(artifacts.SoulBody) != "" {
		return soul.Parse(ctx, soul.ParseRequest{
			SourcePath:    strings.TrimSpace(artifacts.SoulSourcePath),
			WorkspaceRoot: m.soulSourceRoot(strings.TrimSpace(artifacts.SoulSourcePath), workspaceSnapshot),
			Content:       []byte(artifacts.SoulBody),
			Config:        config,
		})
	}
	if artifacts.PackageOwned {
		return soul.Empty(config, strings.TrimSpace(artifacts.SoulSourcePath))
	}
	agentPath := soulAgentPath(artifacts.Agent, workspaceSnapshot)
	return soul.Resolve(ctx, soul.ResolveRequest{
		AgentPath:     agentPath,
		WorkspaceRoot: m.soulSourceRoot(agentPath, workspaceSnapshot),
		Config:        config,
	})
}

func (m *Manager) soulSourceRoot(
	sourcePath string,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) string {
	workspaceRoot := ""
	if workspaceSnapshot != nil {
		workspaceRoot = strings.TrimSpace(workspaceSnapshot.RootDir)
	}
	trimmedSource := strings.TrimSpace(sourcePath)
	if trimmedSource == "" || !filepath.IsAbs(trimmedSource) {
		return workspaceRoot
	}

	for _, root := range trustedSoulSourceRoots(m, workspaceSnapshot) {
		if pathWithinRoot(root, trimmedSource) {
			return strings.TrimSpace(root)
		}
	}
	return workspaceRoot
}

func trustedSoulSourceRoots(
	manager *Manager,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) []string {
	roots := make([]string, 0, 3)
	if workspaceSnapshot != nil {
		if root := strings.TrimSpace(workspaceSnapshot.RootDir); root != "" {
			roots = append(roots, root)
		}
		for _, root := range workspaceSnapshot.AdditionalDirs {
			if trimmed := strings.TrimSpace(root); trimmed != "" {
				roots = append(roots, trimmed)
			}
		}
	}
	if manager != nil {
		if home := strings.TrimSpace(manager.homePaths.HomeDir); home != "" {
			roots = append(roots, home)
		}
	}
	return roots
}

func pathWithinRoot(root string, sourcePath string) bool {
	trimmedRoot := strings.TrimSpace(root)
	trimmedSource := strings.TrimSpace(sourcePath)
	if trimmedRoot == "" || trimmedSource == "" {
		return false
	}
	absRoot, err := filepath.Abs(filepath.Clean(trimmedRoot))
	if err != nil {
		return false
	}
	sourceForRoot := filepath.Clean(trimmedSource)
	if !filepath.IsAbs(sourceForRoot) {
		sourceForRoot = filepath.Join(absRoot, sourceForRoot)
	}
	absSource, err := filepath.Abs(sourceForRoot)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absSource)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
