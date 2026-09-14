package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (m *Manager) startupPrompt(
	ctx context.Context,
	sessionCtx hookspkg.SessionContext,
	startupCtx StartupPromptContext,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, acp.StartupManifest, error) {
	prompt, manifest, err := assembleStartupPrompt(ctx, m.assembler, startupCtx, agent, workspace)
	if err != nil {
		return "", acp.StartupManifest{}, fmt.Errorf("session: assemble prompt for %q: %w", agent.Name, err)
	}
	final, err := m.dispatchPromptPostAssemble(ctx, sessionCtx, prompt)
	if err != nil {
		return "", acp.StartupManifest{}, err
	}
	if final != prompt {
		manifest = acp.OpaqueStartupManifest(final, true)
	}
	return final, manifest, nil
}

func assembleStartupPrompt(
	ctx context.Context,
	assembler PromptAssembler,
	startupCtx StartupPromptContext,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, acp.StartupManifest, error) {
	base := strings.TrimSpace(agent.Prompt)
	if assembler == nil {
		manifest := acp.StartupManifest{}
		if base != "" {
			manifest.Spans = []acp.DeliveredSpan{acp.TextSpan("agent_prompt", base)}
		}
		return base, manifest, nil
	}
	if typed, ok := assembler.(StartupManifestAssembler); ok {
		return typed.AssembleStartupWithManifest(ctx, startupCtx, agent, workspace)
	}
	var prompt string
	var err error
	if typed, ok := assembler.(StartupPromptAssembler); ok {
		prompt, err = typed.AssembleStartup(ctx, startupCtx, agent, workspace)
	} else {
		prompt, err = assembler.Assemble(ctx, agent, workspace)
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = base
	}
	return prompt, acp.OpaqueStartupManifest(prompt, false), err
}
