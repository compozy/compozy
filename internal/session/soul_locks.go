package session

import (
	"path/filepath"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/soul"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *Session) updateSoulSnapshot(snapshot *soul.Snapshot, parentSoulDigest string, updatedAt time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SoulSnapshotID = ""
	s.SoulDigest = ""
	if snapshot != nil {
		s.SoulSnapshotID = strings.TrimSpace(snapshot.ID)
		s.SoulDigest = strings.TrimSpace(snapshot.Digest)
	}
	s.ParentSoulDigest = strings.TrimSpace(parentSoulDigest)
	if !updatedAt.IsZero() {
		s.UpdatedAt = updatedAt.UTC()
	}
}

func soulAgentPath(agentDef aghconfig.AgentDef, workspaceSnapshot *workspacepkg.ResolvedWorkspace) string {
	if sourcePath := strings.TrimSpace(agentDef.SourcePath); sourcePath != "" {
		return sourcePath
	}
	if workspaceSnapshot != nil {
		for _, candidate := range workspaceSnapshot.Agents {
			if strings.TrimSpace(candidate.Name) == strings.TrimSpace(agentDef.Name) &&
				strings.TrimSpace(candidate.SourcePath) != "" {
				return strings.TrimSpace(candidate.SourcePath)
			}
		}
		if root := strings.TrimSpace(workspaceSnapshot.RootDir); root != "" && strings.TrimSpace(agentDef.Name) != "" {
			return filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, agentDef.Name, "AGENT.md")
		}
	}
	return ""
}

func cloneSoulSnapshotPointer(snapshot *soul.Snapshot) *soul.Snapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.ProfileJSON = append([]byte(nil), snapshot.ProfileJSON...)
	return &clone
}

func cloneResolvedSoulPointer(resolved *soul.ResolvedSoul) *soul.ResolvedSoul {
	if resolved == nil {
		return nil
	}
	clone := *resolved
	clone.Profile.Tone = append([]string(nil), resolved.Profile.Tone...)
	clone.Profile.Principles = append([]string(nil), resolved.Profile.Principles...)
	clone.Profile.Constraints = append([]string(nil), resolved.Profile.Constraints...)
	clone.Profile.Collaboration = append([]string(nil), resolved.Profile.Collaboration...)
	clone.Profile.MemoryPolicy = append([]string(nil), resolved.Profile.MemoryPolicy...)
	clone.Profile.Tags = append([]string(nil), resolved.Profile.Tags...)
	clone.Compact.Tone = append([]string(nil), resolved.Compact.Tone...)
	clone.Compact.Principles = append([]string(nil), resolved.Compact.Principles...)
	clone.ReadModel.Frontmatter.Tone = append([]string(nil), resolved.ReadModel.Frontmatter.Tone...)
	clone.ReadModel.Frontmatter.Principles = append([]string(nil), resolved.ReadModel.Frontmatter.Principles...)
	clone.ReadModel.Frontmatter.Constraints = append([]string(nil), resolved.ReadModel.Frontmatter.Constraints...)
	clone.ReadModel.Frontmatter.Collaboration = append([]string(nil), resolved.ReadModel.Frontmatter.Collaboration...)
	clone.ReadModel.Frontmatter.MemoryPolicy = append([]string(nil), resolved.ReadModel.Frontmatter.MemoryPolicy...)
	clone.ReadModel.Frontmatter.Tags = append([]string(nil), resolved.ReadModel.Frontmatter.Tags...)
	clone.ReadModel.Diagnostics = append([]soul.Diagnostic(nil), resolved.ReadModel.Diagnostics...)
	clone.Diagnostics = append([]soul.Diagnostic(nil), resolved.Diagnostics...)
	return &clone
}

func (m *Manager) tryAcquireSoulLock(sessionID string) (func(), bool) {
	lock := m.sessionSoulLock(sessionID)
	select {
	case <-lock:
		return func() { releaseSoulLock(lock) }, true
	default:
		return nil, false
	}
}

func (m *Manager) sessionSoulLock(sessionID string) chan struct{} {
	target := strings.TrimSpace(sessionID)
	m.soulLocksMu.Lock()
	defer m.soulLocksMu.Unlock()
	if m.soulLocks == nil {
		m.soulLocks = make(map[string]chan struct{})
	}
	lock, ok := m.soulLocks[target]
	if ok {
		return lock
	}
	lock = make(chan struct{}, 1)
	lock <- struct{}{}
	m.soulLocks[target] = lock
	return lock
}

func releaseSoulLock(lock chan struct{}) {
	if lock == nil {
		return
	}
	select {
	case lock <- struct{}{}:
	default:
	}
}
