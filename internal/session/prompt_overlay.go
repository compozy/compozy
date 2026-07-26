package session

import (
	"context"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
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
		agent aghconfig.AgentDef,
		workspace *workspacepkg.ResolvedWorkspace,
	) (string, error)
}

// StartupPromptOverlay applies daemon-owned startup prompt overlays after the
// base assembler has produced the startup prompt.
type StartupPromptOverlay interface {
	Apply(ctx context.Context, startup StartupPromptContext, prompt string) (string, error)
}
