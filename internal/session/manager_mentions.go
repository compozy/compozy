package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/mention"
)

// MentionReferenceForm names the session-references block form chosen for a dispatch.
type MentionReferenceForm string

const (
	// MentionReferenceFormReference tells the agent to read each session with compozy__session_history.
	MentionReferenceFormReference MentionReferenceForm = "reference"
	// MentionReferenceFormInline carries a budgeted history tail per referenced session.
	MentionReferenceFormInline MentionReferenceForm = "inline"
	// MentionReferenceFormUnavailable lists the references without any history access.
	MentionReferenceFormUnavailable MentionReferenceForm = "unavailable"
)

// MentionReferences is the rendered session-references block for one dispatch.
// Text is empty when no referenced session is still available.
type MentionReferences struct {
	Text     string
	Form     MentionReferenceForm
	Sessions int // referenced sessions still available and rendered
	// SessionIDs lists the rendered sessions in block order; it feeds the delivery manifest.
	SessionIDs []string
}

// MentionReferenceRenderer renders the session-references text block for one dispatch.
// humanIngress reports whether the prompt came from human operator ingress.
type MentionReferenceRenderer func(
	ctx context.Context,
	target *Session,
	refs []mention.Mention,
	humanIngress bool,
) (MentionReferences, error)

// WithMentionReferenceRenderer installs the daemon renderer for session mentions.
func WithMentionReferenceRenderer(renderer MentionReferenceRenderer) Option {
	return func(manager *Manager) {
		manager.mentionRenderer = renderer
	}
}

// WorkingDir resolves the current execution directory, falling back to creation metadata for older sessions.
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
