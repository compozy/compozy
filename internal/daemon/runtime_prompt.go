package daemon

import (
	"context"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	aghRuntimeEnvelopeStart = "<agh-runtime-context>"
	aghRuntimeEnvelopeEnd   = "</agh-runtime-context>"
)

var (
	_ session.PromptProvider       = aghRuntimePromptProvider{}
	_ session.StartupPromptOverlay = aghRuntimePromptOverlay{}
)

type aghRuntimePromptProvider struct{}

func (aghRuntimePromptProvider) PromptSection(
	_ context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	startup := session.StartupPromptContext{}
	if workspace != nil {
		startup.WorkspaceID = strings.TrimSpace(workspace.ID)
		startup.Workspace = strings.TrimSpace(workspace.RootDir)
	}
	return renderAGHRuntimeEnvelope(startup), nil
}

func (aghRuntimePromptProvider) PromptStartupSection(
	_ context.Context,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	startup = hydrateAGHRuntimeEnvelopeContext(startup, agent, workspace)
	return renderAGHRuntimeEnvelope(startup), nil
}

type aghRuntimePromptOverlay struct{}

func (aghRuntimePromptOverlay) Apply(
	_ context.Context,
	startup session.StartupPromptContext,
	prompt string,
) (string, error) {
	envelope := renderAGHRuntimeEnvelope(startup)
	body := stripAGHRuntimeEnvelope(prompt)
	if body == "" {
		return envelope, nil
	}
	return envelope + "\n\n" + body, nil
}

func hydrateAGHRuntimeEnvelopeContext(
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
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

func renderAGHRuntimeEnvelope(startup session.StartupPromptContext) string {
	var builder strings.Builder
	builder.WriteString(aghRuntimeEnvelopeStart)
	builder.WriteString("\n# AGH Runtime\n\n")
	builder.WriteString(
		"You are running inside AGH. AGH is a local-first daemon and agent operating system " +
			"that launched and supervises this agent session. AGH owns the session lifecycle, " +
			"workspace context, memory and situation prompt sections, native tool gateway, and " +
			"observable event stream.\n\n",
	)
	builder.WriteString(
		"Treat AGH-provided startup sections, live turn context, and metadata as daemon-owned " +
			"runtime guidance. Prefer AGH-native tools when they are visible and callable; " +
			"otherwise use AGH CLI, HTTP, or UDS surfaces for AGH runtime operations.\n\n",
	)
	builder.WriteString(
		"AGH native tool IDs such as `agh__tool_search` are canonical registry IDs, not guaranteed " +
			"direct harness invocation names. When you need a native tool, search or load it by " +
			"capability and canonical ID, then call the tool reference surfaced by the active harness " +
			"exactly as returned. Use bare `agh__*` IDs inside AGH descriptor fields, policy config, " +
			"CLI/API commands, and `tool_id` inputs.\n\n",
	)
	builder.WriteString("Current session facts:\n")
	writeAGHRuntimeFact(&builder, "session_id", startup.SessionID)
	writeAGHRuntimeFact(&builder, "session_name", startup.SessionName)
	writeAGHRuntimeFact(&builder, "session_type", string(startup.SessionType))
	writeAGHRuntimeFact(&builder, "agent_name", startup.AgentName)
	writeAGHRuntimeFact(&builder, "provider", startup.Provider)
	writeAGHRuntimeFact(&builder, "workspace_id", startup.WorkspaceID)
	writeAGHRuntimeFact(&builder, "workspace", startup.Workspace)
	if startup.NetworkParticipation.Mode == participation.ModeLive {
		writeAGHRuntimeFact(&builder, "channel", startup.NetworkParticipation.ChannelID)
	}
	builder.WriteString(aghRuntimeEnvelopeEnd)
	return strings.TrimSpace(builder.String())
}

func writeAGHRuntimeFact(builder *strings.Builder, name string, value string) {
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

func stripAGHRuntimeEnvelope(prompt string) string {
	text := strings.TrimSpace(prompt)
	for {
		start := strings.Index(text, aghRuntimeEnvelopeStart)
		if start < 0 {
			return strings.TrimSpace(text)
		}

		searchFrom := start + len(aghRuntimeEnvelopeStart)
		endOffset := strings.Index(text[searchFrom:], aghRuntimeEnvelopeEnd)
		if endOffset < 0 {
			text = strings.TrimSpace(text[:start] + text[searchFrom:])
			continue
		}

		end := searchFrom + endOffset + len(aghRuntimeEnvelopeEnd)
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
