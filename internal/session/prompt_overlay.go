package session

import (
	"context"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/soul"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// StartupPromptContext carries the durable session metadata available during
// startup prompt assembly and overlay selection.
type StartupPromptContext struct {
	SessionID            string
	SessionName          string
	AgentName            string
	Provider             string
	WorkspaceID          string
	Workspace            string
	WorktreeID           string
	NetworkParticipation participation.Spec
	SessionType          Type
	SoulSnapshot         *soul.Snapshot
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// StartupPromptAssembler optionally extends PromptAssembler with durable
// startup context so daemon-owned assemblers can select sections before the
// final system prompt is concatenated.
type StartupPromptAssembler interface {
	AssembleStartup(
		ctx context.Context,
		startup StartupPromptContext,
		agent compozyconfig.AgentDef,
		workspace *workspacepkg.ResolvedWorkspace,
	) (string, error)
}

// StartupPromptOverlay applies daemon-owned startup prompt overlays after the
// base assembler has produced the startup prompt.
type StartupPromptOverlay interface {
	Apply(ctx context.Context, startup StartupPromptContext, prompt string) (string, error)
}
