package daemon

import (
	"context"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type soulPromptSectionProvider struct{}

func (soulPromptSectionProvider) PromptSection(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
) (string, error) {
	return "", nil
}

func (soulPromptSectionProvider) PromptStartupSection(
	_ context.Context,
	startup session.StartupPromptContext,
	_ aghconfig.AgentDef,
	_ *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if startup.SoulSnapshot == nil {
		return "", nil
	}
	profile, err := startup.SoulSnapshot.ProfileEnvelope()
	if err != nil {
		return "", fmt.Errorf("daemon: decode startup soul snapshot: %w", err)
	}
	if !profile.Present || !profile.Active || !profile.Valid {
		return "", nil
	}
	return renderSoulPromptSection(startup.SoulSnapshot, &profile), nil
}

func startupHasSoulSnapshot(startup session.StartupPromptContext) bool {
	return startup.SoulSnapshot != nil && strings.TrimSpace(startup.SoulSnapshot.ID) != ""
}

func renderSoulPromptSection(snapshot *soul.Snapshot, profile *soul.SnapshotProfile) string {
	if snapshot == nil || profile == nil {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("<agh-agent-soul>\n")
	builder.WriteString("# Agent Soul\n\n")
	writeSoulPromptLine(&builder, "Snapshot ID", snapshot.ID)
	writeSoulPromptLine(&builder, "Digest", snapshot.Digest)
	writeSoulPromptLine(&builder, "Source", snapshot.SourcePath)
	writeSoulPromptLine(&builder, "Role", profile.Profile.Role)
	writeSoulPromptList(&builder, "Tone", profile.Profile.Tone)
	writeSoulPromptList(&builder, "Principles", profile.Profile.Principles)
	writeSoulPromptList(&builder, "Constraints", profile.Profile.Constraints)
	writeSoulPromptList(&builder, "Collaboration", profile.Profile.Collaboration)
	writeSoulPromptList(&builder, "Memory policy", profile.Profile.MemoryPolicy)
	writeSoulPromptList(&builder, "Tags", profile.Profile.Tags)
	if strings.TrimSpace(profile.Profile.Body) != "" {
		builder.WriteString("\n## Body\n")
		builder.WriteString(strings.TrimSpace(profile.Profile.Body))
		builder.WriteString("\n")
	}
	if snapshot.Truncated || profile.Profile.Truncated || profile.ReadModel.Truncated {
		builder.WriteString("\nTruncated: true\n")
	}
	builder.WriteString("</agh-agent-soul>")
	return strings.TrimSpace(builder.String())
}

func writeSoulPromptLine(builder *strings.Builder, label string, value string) {
	if builder == nil {
		return
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	builder.WriteString(label)
	builder.WriteString(": ")
	builder.WriteString(trimmed)
	builder.WriteString("\n")
}

func writeSoulPromptList(builder *strings.Builder, label string, values []string) {
	if builder == nil || len(values) == 0 {
		return
	}
	builder.WriteString(label)
	builder.WriteString(":\n")
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(trimmed)
		builder.WriteString("\n")
	}
}
