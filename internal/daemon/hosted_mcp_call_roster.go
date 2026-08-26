package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/core"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	callRosterMaxAgents      = 32
	callRosterDescriptionMax = 120
)

func hostedCallRosterDecorator(state *bootState) mcppkg.HostedProjectionDecorator {
	return func(ctx context.Context, scope toolspkg.Scope, views []toolspkg.ToolView) ([]toolspkg.ToolView, error) {
		if !slices.ContainsFunc(views, func(view toolspkg.ToolView) bool {
			return view.Descriptor.ID == toolspkg.ToolIDAgentCall
		}) {
			return views, nil
		}
		remaining, err := callDepthRemaining(ctx, state, scope.SessionID)
		if err != nil {
			return nil, err
		}
		entries, err := callRosterEntries(ctx, state, scope.ProfileID, scope.WorkspaceID)
		if err != nil {
			return nil, err
		}
		roster := renderCallRoster(entries) + fmt.Sprintf("\nDelegation depth remaining: %d.", remaining)
		projected := make([]toolspkg.ToolView, 0, len(views))
		for index := range views {
			if views[index].Descriptor.ID != toolspkg.ToolIDAgentCall {
				projected = append(projected, views[index])
				continue
			}
			if remaining == 0 {
				continue
			}
			schema, schemaErr := injectCallRosterSchema(views[index].Descriptor.InputSchema, roster)
			if schemaErr != nil {
				return nil, schemaErr
			}
			views[index].Descriptor.InputSchema = schema
			views[index].Descriptor.InputSchemaDigest = ""
			views[index].Descriptor, schemaErr = toolspkg.DescriptorWithSchemaDigests(views[index].Descriptor)
			if schemaErr != nil {
				return nil, fmt.Errorf("daemon: digest roster-projected call tool: %w", schemaErr)
			}
			projected = append(projected, views[index])
		}
		return projected, nil
	}
}

func callDepthRemaining(ctx context.Context, state *bootState, sessionID string) (int, error) {
	maximum := 0
	if state != nil {
		maximum = state.cfg.Calls.MaxDepth
	}
	sessionID = strings.TrimSpace(sessionID)
	if state == nil || state.sessions == nil || sessionID == "" {
		return maximum, nil
	}
	info, err := state.sessions.Status(ctx, sessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return maximum, nil
	}
	if err != nil {
		return 0, fmt.Errorf("daemon: inspect call roster session: %w", err)
	}
	depth := 0
	if info != nil && info.Lineage != nil {
		depth = info.Lineage.SpawnDepth
	}
	return max(maximum-depth, 0), nil
}

func callRosterEntries(
	ctx context.Context,
	state *bootState,
	profileID string,
	workspaceID string,
) ([]core.AgentCatalogEntry, error) {
	if state == nil {
		return nil, nil
	}
	catalog := agentCatalogDependency(state.agentCatalog)
	if catalog == nil {
		return nil, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	profileName, err := callRosterProfileName(ctx, state, profileID)
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		return catalog.ListAgentsForProfile(ctx, profileID, profileName)
	}
	if state.workspaceResolver == nil {
		return nil, fmt.Errorf("daemon: workspace resolver is unavailable for call roster")
	}
	resolved, err := state.workspaceResolver.ResolveForProfile(ctx, workspaceID, profileName)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve call roster workspace: %w", err)
	}
	return catalog.ListAgentsForWorkspace(ctx, &resolved)
}

func callRosterProfileName(ctx context.Context, state *bootState, profileID string) (string, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || profileID == store.DefaultProfileID {
		return daemonDefaultProfileName, nil
	}
	if state == nil || state.profiles == nil {
		return "", errors.New("daemon: profile manager is unavailable for call roster")
	}
	name, err := state.profiles.ProfileName(ctx, profileID)
	if err != nil {
		return "", fmt.Errorf("daemon: resolve call roster profile: %w", err)
	}
	return name, nil
}

func renderCallRoster(entries []core.AgentCatalogEntry) string {
	entries = append([]core.AgentCatalogEntry(nil), entries...)
	slices.SortFunc(entries, func(left, right core.AgentCatalogEntry) int {
		return strings.Compare(strings.TrimSpace(left.Def.Name), strings.TrimSpace(right.Def.Name))
	})
	if len(entries) == 0 {
		return "No agents are available. Create one with `compozy agent create`."
	}
	total := len(entries)
	if len(entries) > callRosterMaxAgents {
		entries = entries[:callRosterMaxAgents]
	}
	lines := make([]string, 0, len(entries)+2)
	lines = append(lines, "Available agents for this profile and workspace:")
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Def.Name)
		description := inertRosterText(entry.Def.Description)
		if description == "" {
			lines = append(lines, "- "+name)
			continue
		}
		lines = append(lines, "- "+name+" — "+description)
	}
	if omitted := total - len(entries); omitted > 0 {
		lines = append(lines, fmt.Sprintf("- %d more agents. Use `compozy__agent_list` to see all.", omitted))
	}
	return strings.Join(lines, "\n")
}

func inertRosterText(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	value = strings.NewReplacer(
		"\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_",
		"[", "\\[", "]", "\\]", "<", "\\<", ">", "\\>", "#", "\\#",
	).Replace(value)
	if utf8.RuneCountInString(value) <= callRosterDescriptionMax {
		return value
	}
	runes := []rune(value)
	truncated := strings.TrimSpace(string(runes[:callRosterDescriptionMax-1]))
	truncated = strings.TrimSuffix(truncated, "\\")
	return truncated + "…"
}

func injectCallRosterSchema(raw json.RawMessage, roster string) (json.RawMessage, error) {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("daemon: decode call schema for roster: %w", err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("daemon: call schema properties are missing")
	}
	if err := setCallAgentSchemaDescription(properties, roster); err != nil {
		return nil, err
	}
	tasks, ok := properties["tasks"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("daemon: call tasks schema is missing")
	}
	items, ok := tasks["items"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("daemon: call task item schema is missing")
	}
	itemProperties, ok := items["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("daemon: call task item properties are missing")
	}
	if err := setCallAgentSchemaDescription(itemProperties, roster); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("daemon: encode call schema with roster: %w", err)
	}
	return encoded, nil
}

func setCallAgentSchemaDescription(properties map[string]any, roster string) error {
	agent, ok := properties["agent"].(map[string]any)
	if !ok {
		return fmt.Errorf("daemon: call agent schema is missing")
	}
	agent["description"] = roster
	return nil
}
