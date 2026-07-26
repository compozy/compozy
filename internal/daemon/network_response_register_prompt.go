package daemon

import (
	"context"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var (
	_ session.PromptProvider       = networkResponseRegisterPromptSectionProvider{}
	_ startupPromptSectionProvider = networkResponseRegisterPromptSectionProvider{}
)

type networkResponseRegisterPromptSectionProvider struct{}

func (networkResponseRegisterPromptSectionProvider) PromptSection(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
) (string, error) {
	return "", nil
}

func (networkResponseRegisterPromptSectionProvider) PromptStartupSection(
	_ context.Context,
	startup session.StartupPromptContext,
	_ aghconfig.AgentDef,
	_ *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return renderNetworkResponseRegisterStartupSection(startup), nil
}

func renderNetworkResponseRegisterStartupSection(startup session.StartupPromptContext) string {
	var builder strings.Builder
	builder.WriteString("# AGH Network Response Register\n\n")
	builder.WriteString(
		"Threads decide and discuss; actionable work is promoted to tasks before execution. " +
			"When network prompts arrive, reply briefly only when addressed, mentioned, activated, or adding value.",
	)
	if channel := strings.TrimSpace(startup.NetworkParticipation.ChannelID); channel != "" {
		builder.WriteString(" Current channel: `")
		builder.WriteString(channel)
		builder.WriteString("`.")
	}
	return builder.String()
}
