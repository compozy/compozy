package daemon

import (
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type sessionAgentProjection[T any] struct {
	agent                 compozyconfig.AgentDef
	hasConcreteDefinition bool
	workspace             *workspacepkg.ResolvedWorkspace
	agentResolver         func() session.AgentResolver
	byName                func(string) (T, error)
	byDefinition          func(compozyconfig.AgentDef) (T, error)
}

func resolveSessionAgentProjection[T any](request sessionAgentProjection[T]) (T, error) {
	if request.hasConcreteDefinition && strings.TrimSpace(request.agent.SourcePath) == "" {
		return request.byDefinition(request.agent)
	}

	projection, err := request.byName(request.agent.Name)
	if err == nil {
		return projection, nil
	}
	if !errors.Is(err, skillspkg.ErrAgentNotFound) {
		return projection, err
	}

	resolved, found, resolveErr := resolveSessionAgentDefinition(
		request.agent.Name,
		request.workspace,
		request.agentResolver,
	)
	if resolveErr != nil {
		return projection, resolveErr
	}
	if !found {
		return projection, err
	}
	return request.byDefinition(resolved)
}

func resolveSessionAgentDefinition(
	agentName string,
	workspace *workspacepkg.ResolvedWorkspace,
	agentResolver func() session.AgentResolver,
) (compozyconfig.AgentDef, bool, error) {
	if agentResolver == nil || strings.TrimSpace(agentName) == "" {
		return compozyconfig.AgentDef{}, false, nil
	}
	resolver := agentResolver()
	if resolver == nil {
		return compozyconfig.AgentDef{}, false, nil
	}
	resolved, err := resolver.ResolveAgent(agentName, workspace)
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return compozyconfig.AgentDef{}, false, nil
	}
	if err != nil {
		return compozyconfig.AgentDef{}, false, fmt.Errorf("daemon: resolve session agent %q: %w", agentName, err)
	}
	return resolved, true, nil
}
