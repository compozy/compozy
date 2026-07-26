package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/acp"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
)

func agentProbeTargetSource(
	cfg *aghconfig.Config,
	catalog core.AgentCatalog,
	logger *slog.Logger,
) func(context.Context) ([]acp.ProbeTarget, error) {
	return func(ctx context.Context) ([]acp.ProbeTarget, error) {
		return collectAgentProbeTargets(ctx, cfg, catalog, logger)
	}
}

func collectAgentProbeTargets(
	ctx context.Context,
	cfg *aghconfig.Config,
	catalog core.AgentCatalog,
	logger *slog.Logger,
) ([]acp.ProbeTarget, error) {
	if cfg == nil {
		cfg = &aghconfig.Config{}
	}
	targets := make([]acp.ProbeTarget, 0)
	if catalog != nil {
		agents, err := catalog.ListAgents(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list agents for probe health: %w", err)
		}
		sort.SliceStable(agents, func(i, j int) bool {
			return strings.TrimSpace(agents[i].Def.Name) < strings.TrimSpace(agents[j].Def.Name)
		})
		for _, entry := range agents {
			agent := entry.Def
			resolved, err := cfg.ResolveAgent(agent)
			if err != nil {
				if logger != nil {
					logger.Warn(
						"daemon: resolve agent for probe health failed",
						"agent_name", strings.TrimSpace(agent.Name),
						"provider", strings.TrimSpace(agent.Provider),
						"error", err,
					)
				}
				targets = append(targets, acp.ProbeTarget{
					AgentName: strings.TrimSpace(agent.Name),
					Provider:  strings.TrimSpace(agent.Provider),
					Command:   strings.TrimSpace(agent.Command),
				})
				continue
			}
			targets = append(targets, acp.ProbeTarget{
				AgentName: strings.TrimSpace(resolved.Name),
				Provider:  strings.TrimSpace(resolved.Provider),
				Command:   strings.TrimSpace(resolved.Command),
			})
		}
	}

	providerNames := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		providerNames = append(providerNames, strings.TrimSpace(name))
	}
	sort.Strings(providerNames)
	for _, name := range providerNames {
		if name == "" {
			continue
		}
		provider, err := cfg.ResolveProvider(name)
		if err != nil {
			if logger != nil {
				logger.Warn("daemon: resolve provider for probe health failed", "provider", name, "error", err)
			}
			continue
		}
		targets = append(targets, acp.ProbeTarget{
			Provider: name,
			Command:  strings.TrimSpace(provider.Command),
		})
	}

	return dedupeAgentProbeTargets(targets), nil
}

func dedupeAgentProbeTargets(targets []acp.ProbeTarget) []acp.ProbeTarget {
	if len(targets) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(targets))
	deduped := make([]acp.ProbeTarget, 0, len(targets))
	for _, target := range targets {
		target.AgentName = strings.TrimSpace(target.AgentName)
		target.Provider = strings.TrimSpace(target.Provider)
		target.Command = strings.TrimSpace(target.Command)
		key := strings.Join([]string{target.AgentName, target.Provider, target.Command}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, target)
	}
	return deduped
}
