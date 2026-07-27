package daemon

import (
	"context"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	compozyRuntimeEnvelopeStart = "<compozy-runtime-context>"
	compozyRuntimeEnvelopeEnd   = "</compozy-runtime-context>"
)

var (
	_ session.PromptProvider       = compozyRuntimePromptProvider{}
	_ session.StartupPromptOverlay = compozyRuntimePromptOverlay{}
)

type compozyRuntimePromptProvider struct{}

func (compozyRuntimePromptProvider) PromptSection(
	_ context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	startup := session.StartupPromptContext{}
	if workspace != nil {
		startup.WorkspaceID = strings.TrimSpace(workspace.ID)
		startup.Workspace = strings.TrimSpace(workspace.RootDir)
	}
	return renderCompozyRuntimeEnvelope(startup), nil
}

func (compozyRuntimePromptProvider) PromptStartupSection(
	_ context.Context,
	startup session.StartupPromptContext,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	startup = hydrateCompozyRuntimeEnvelopeContext(startup, agent, workspace)
	return renderCompozyRuntimeEnvelope(startup), nil
}

type compozyRuntimePromptOverlay struct{}

func (compozyRuntimePromptOverlay) Apply(
	_ context.Context,
	startup session.StartupPromptContext,
	prompt string,
) (string, error) {
	envelope := renderCompozyRuntimeEnvelope(startup)
	body := stripCompozyRuntimeEnvelope(prompt)
	if body == "" {
		return envelope, nil
	}
	return envelope + "\n\n" + body, nil
}

func hydrateCompozyRuntimeEnvelopeContext(
	startup session.StartupPromptContext,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) session.StartupPromptContext {
	if strings.TrimSpace(startup.AgentName) == "" {
		startup.AgentName = strings.TrimSpace(agent.Name)
	}
	if strings.TrimSpace(startup.Provider) == "" {
		startup.Provider = strings.TrimSpace(agent.Provider)
	}
	if workspace != nil {
		if strings.TrimSpace(startup.WorkspaceID) == "" {
			startup.WorkspaceID = strings.TrimSpace(workspace.ID)
		}
		if strings.TrimSpace(startup.Workspace) == "" {
			startup.Workspace = strings.TrimSpace(workspace.RootDir)
		}
	}
	return startup
}

func renderCompozyRuntimeEnvelope(startup session.StartupPromptContext) string {
	var builder strings.Builder
	builder.WriteString(compozyRuntimeEnvelopeStart)
	builder.WriteString("\n# Compozy Runtime\n\n")
	builder.WriteString(
		"You are running inside Compozy. Compozy is a local-first daemon and agent operating system " +
			"that launched and supervises this agent session. Compozy owns the session lifecycle, " +
			"workspace context, memory and situation prompt sections, native tool gateway, and " +
			"observable event stream.\n\n",
	)
	builder.WriteString(
		"Treat Compozy-provided startup sections, live turn context, and metadata as daemon-owned " +
			"runtime guidance. Prefer Compozy-native tools when they are visible and callable; " +
			"otherwise use Compozy CLI, HTTP, or UDS surfaces for Compozy runtime operations.\n\n",
	)
	builder.WriteString(
		"Compozy native tool IDs such as `compozy__tool_search` are canonical registry IDs, not guaranteed " +
			"direct harness invocation names. When you need a native tool, search or load it by " +
			"capability and canonical ID, then call the tool reference surfaced by the active harness " +
			"exactly as returned. Use bare `compozy__*` IDs inside Compozy descriptor fields, policy config, " +
			"CLI/API commands, and `tool_id` inputs.\n\n",
	)
	builder.WriteString("Current session facts:\n")
	writeCompozyRuntimeFact(&builder, "session_id", startup.SessionID)
	writeCompozyRuntimeFact(&builder, "session_name", startup.SessionName)
	writeCompozyRuntimeFact(&builder, "session_type", string(startup.SessionType))
	writeCompozyRuntimeFact(&builder, "agent_name", startup.AgentName)
	writeCompozyRuntimeFact(&builder, "provider", startup.Provider)
	writeCompozyRuntimeFact(&builder, "workspace_id", startup.WorkspaceID)
	writeCompozyRuntimeFact(&builder, "workspace", startup.Workspace)
	if startup.NetworkParticipation.Mode == participation.ModeLive {
		writeCompozyRuntimeFact(&builder, "channel", startup.NetworkParticipation.ChannelID)
	}
	builder.WriteString(compozyRuntimeEnvelopeEnd)
	return strings.TrimSpace(builder.String())
}

func writeCompozyRuntimeFact(builder *strings.Builder, name string, value string) {
	if builder == nil {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString("- ")
	builder.WriteString(name)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteString("\n")
}

func stripCompozyRuntimeEnvelope(prompt string) string {
	text := strings.TrimSpace(prompt)
	for {
		start := strings.Index(text, compozyRuntimeEnvelopeStart)
		if start < 0 {
			return strings.TrimSpace(text)
		}

		searchFrom := start + len(compozyRuntimeEnvelopeStart)
		endOffset := strings.Index(text[searchFrom:], compozyRuntimeEnvelopeEnd)
		if endOffset < 0 {
			text = strings.TrimSpace(text[:start] + text[searchFrom:])
			continue
		}

		end := searchFrom + endOffset + len(compozyRuntimeEnvelopeEnd)
		before := strings.TrimSpace(text[:start])
		after := strings.TrimSpace(text[end:])
		switch {
		case before == "":
			text = after
		case after == "":
			text = before
		default:
			text = before + "\n\n" + after
		}
	}
}
