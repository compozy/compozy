package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/mention"
)

// MentionReferenceRenderer renders the session-references text block for one dispatch.
// humanIngress reports whether the prompt came from human operator ingress.
// It returns "" when no referenced session is still available.
type MentionReferenceRenderer func(
	ctx context.Context,
	target *Session,
	refs []mention.Mention,
	humanIngress bool,
) (string, error)

// WithMentionReferenceRenderer installs the daemon renderer for session mentions.
func WithMentionReferenceRenderer(renderer MentionReferenceRenderer) Option {
	return func(manager *Manager) {
		manager.mentionRenderer = renderer
	}
}

// WorkingDir returns the directory the session's agent runs in, using the launch
// precedence (CreationProfile.CWD, else persisted cwd, else execution root)
// normalized by ResolveSessionCWD.
func (m *Manager) WorkingDir(ctx context.Context, sessionID string) (string, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, sessionID)
	if err != nil {
		return "", err
	}
	workspace, err := m.resolveResumeWorkspace(ctx, &meta)
	if err != nil {
		return "", fmt.Errorf("session: resolve working-directory workspace for %q: %w", meta.ID, err)
	}
	_, worktreeRoot, err := m.resolveSessionWorktree(ctx, workspace.ID, meta.WorktreeIDValue())
	if err != nil {
		return "", fmt.Errorf("session: resolve working-directory worktree for %q: %w", meta.ID, err)
	}
	executionRoot := workspace.RootDir
	if worktreeRoot != "" {
		executionRoot = worktreeRoot
	}
	return resumeSessionCWD(&meta, executionRoot)
}
