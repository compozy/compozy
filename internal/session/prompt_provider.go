package session

import (
	"context"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// PromptProvider returns one workspace-scoped prompt section for composed
// system-prompt assembly.
type PromptProvider interface {
	PromptSection(ctx context.Context, workspace *workspacepkg.ResolvedWorkspace) (string, error)
}
