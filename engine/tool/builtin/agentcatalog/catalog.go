package agentcatalog

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
)

// Catalog provides read access to agent definitions stored in the project resource store.
type Catalog struct {
	store resources.ResourceStore
	cache map[string]*agent.Config
}

// NewCatalog returns a catalog backed by the provided resource store.
func NewCatalog(store resources.ResourceStore) *Catalog {
	return &Catalog{store: store, cache: make(map[string]*agent.Config)}
}

// AgentInfo summarizes available agents with their exposed action identifiers.
type AgentInfo struct {
	ID      string
	Actions []string
}

// AgentDescription provides detailed metadata for an agent, including action prompts.
type AgentDescription struct {
	ID      string
	Actions []AgentAction
}

// AgentAction describes an individual action exposed by an agent.
type AgentAction struct {
	ID     string
	Prompt string
}

// ListAgents returns all agents and action identifiers for the provided project.
func (c *Catalog) ListAgents(ctx context.Context, project string) ([]AgentInfo, error) {
	if c.store == nil {
		return nil, fmt.Errorf("resource store unavailable")
	}
	items, err := c.store.ListWithValues(ctx, project, resources.ResourceAgent)
	if err != nil {
		return nil, err
	}
	infos := make([]AgentInfo, 0, len(items))
	for _, item := range items {
		if item.Key.ID == "" {
			continue
		}
		cfg, cfgErr := c.agentConfigFromValue(item.Key.ID, item.Value)
		if cfgErr != nil {
			continue
		}
		info := AgentInfo{ID: item.Key.ID}
		for _, action := range cfg.Actions {
			if action != nil && action.ID != "" {
				info.Actions = append(info.Actions, action.ID)
			}
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})
	return infos, nil
}

// DescribeAgent loads a single agent configuration and extracts action prompts.
func (c *Catalog) DescribeAgent(ctx context.Context, project, agentID string) (AgentDescription, error) {
	cfg, err := c.Config(ctx, project, agentID)
	if err != nil {
		return AgentDescription{}, err
	}
	actions := make([]AgentAction, 0, len(cfg.Actions))
	for _, action := range cfg.Actions {
		if action == nil || strings.TrimSpace(action.ID) == "" {
			continue
		}
		actions = append(actions, AgentAction{ID: action.ID, Prompt: action.Prompt})
	}
	return AgentDescription{ID: agentID, Actions: actions}, nil
}

// Config fetches and caches the agent configuration for the given identifier.
func (c *Catalog) Config(ctx context.Context, project, agentID string) (*agent.Config, error) {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if cfg, ok := c.cache[trimmed]; ok {
		return cfg, nil
	}
	if c.store == nil {
		return nil, fmt.Errorf("agent %s not found", trimmed)
	}
	key := resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: trimmed}
	value, _, err := c.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	cfg, err := c.agentConfigFromValue(trimmed, value)
	if err != nil {
		return nil, err
	}
	c.cache[trimmed] = cfg
	return cfg, nil
}

func (c *Catalog) agentConfigFromValue(agentID string, value any) (*agent.Config, error) {
	if cfg, ok := c.cache[agentID]; ok {
		return cfg, nil
	}
	cfg := &agent.Config{}
	switch raw := value.(type) {
	case map[string]any:
		if err := cfg.FromMap(raw); err != nil {
			return nil, fmt.Errorf("decode agent %s: %w", agentID, err)
		}
	default:
		mapped, err := core.AsMapDefault(value)
		if err != nil {
			return nil, fmt.Errorf("marshal agent %s: %w", agentID, err)
		}
		if err := cfg.FromMap(mapped); err != nil {
			return nil, fmt.Errorf("decode agent %s: %w", agentID, err)
		}
	}
	return cfg, nil
}
