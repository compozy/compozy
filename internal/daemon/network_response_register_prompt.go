package daemon

import (
	"context"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
	_ compozyconfig.AgentDef,
	_ *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return renderNetworkResponseRegisterStartupSection(startup), nil
}

func renderNetworkResponseRegisterStartupSection(startup session.StartupPromptContext) string {
	var builder strings.Builder
	builder.WriteString("# Compozy Network Response Register\n\n")
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
