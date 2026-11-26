package cli

import (
	"cmp"
	"context"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/session"
)

const (
	completionSessionCap = 50
	completionTaskCap    = 50
	completionLoopCap    = 50
	completionJobCap     = 50
)

func (r *completionRuntime) workspaces(ctx context.Context) []completionCandidate {
	items, err := r.client.ListWorkspaces(ctx)
	if err != nil {
		return nil
	}
	return mapWorkspaceCandidates(items)
}

func mapWorkspaceCandidates(items []WorkspaceRecord) []completionCandidate {
	counts := make(map[string]int, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			counts[name]++
		}
	}
	candidates := make([]completionCandidate, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		id := strings.TrimSpace(item.ID)
		useID := name == "" || counts[name] > 1 || containsWhitespace(name) || workspaceRefLooksLikePath(name) ||
			strings.HasPrefix(name, "ws_") || strings.HasPrefix(name, "ws-")
		value := name
		if useID {
			value = id
		}
		if value == "" {
			continue
		}
		description := strings.TrimSpace(item.RootDir)
		if useID && name != "" {
			description = cmp.Or(name, description)
		}
		candidates = append(candidates, completionCandidate{value: value, description: description})
	}
	return candidates
}

func containsWhitespace(s string) bool {
	return strings.IndexFunc(s, unicode.IsSpace) >= 0
}

func (r *completionRuntime) profiles(ctx context.Context) []completionCandidate {
	profileClient, ok := r.client.(profileResolutionClient)
	if !ok {
		return nil
	}
	items, err := profileClient.ListProfiles(ctx)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		candidates = append(candidates, completionCandidate{
			value:       name,
			description: strings.TrimSpace(item.State),
		})
	}
	return candidates
}

func (r *completionRuntime) agents(ctx context.Context, workspaceRef string) []completionCandidate {
	items, err := r.client.ListAgents(ctx, AgentQuery{Workspace: workspaceRef})
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(items))
	for index := range items {
		item := &items[index]
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		description := cmp.Or(
			strings.TrimSpace(item.DisplayName),
			strings.TrimSpace(item.Description),
			strings.TrimSpace(item.Provider),
		)
		candidates = append(candidates, completionCandidate{value: name, description: description})
	}
	return candidates
}

func (r *completionRuntime) providers(ctx context.Context) []completionCandidate {
	resp, err := r.client.ListProviders(ctx)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Providers))
	for _, item := range resp.Providers {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.DisplayName), strings.TrimSpace(item.AuthStatus.State))
		candidates = append(candidates, completionCandidate{value: name, description: description})
	}
	return candidates
}

func (r *completionRuntime) models(ctx context.Context, providerID string) []completionCandidate {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil
	}
	resp, err := r.client.ListProviderModels(ctx, ProviderModelListQuery{ProviderID: providerID})
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Models))
	for _, item := range resp.Models {
		modelID := strings.TrimSpace(item.ModelID)
		if modelID == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.DisplayName), strings.TrimSpace(item.AvailabilityState))
		candidates = append(candidates, completionCandidate{value: modelID, description: description})
	}
	return candidates
}

func (r *completionRuntime) sessions(ctx context.Context, workspaceRef string) []completionCandidate {
	page, err := r.client.ListSessions(ctx, SessionListQuery{
		Workspace: workspaceRef,
		Sort:      session.ListSortLastActivity,
		Limit:     completionSessionCap,
	})
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(page.Sessions))
	for index := range page.Sessions {
		item := &page.Sessions[index]
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.Name), strings.TrimSpace(item.AgentName))
		candidates = append(candidates, completionCandidate{value: id, description: description})
	}
	return candidates
}

func (r *completionRuntime) tasks(ctx context.Context, workspaceRef string) []completionCandidate {
	query := TaskListQuery{Limit: completionTaskCap}
	if workspaceRef != "" {
		query.Workspace = workspaceRef
	} else {
		query.AllWorkspaces = true
	}
	resp, err := r.client.ListTasks(ctx, query)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Tasks))
	for _, item := range resp.Tasks {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		description := cmp.Or(
			strings.TrimSpace(item.Title),
			strings.TrimSpace(item.Identifier),
			strings.TrimSpace(string(item.Status)),
		)
		candidates = append(candidates, completionCandidate{value: id, description: description})
	}
	return candidates
}

func (r *completionRuntime) loops(ctx context.Context, workspaceRef string) []completionCandidate {
	query := LoopListQuery{Limit: completionLoopCap}
	if workspaceRef == "" {
		query.AllWorkspaces = true
	}
	loopClient, ok := r.client.(loopCommandClient)
	if !ok {
		return nil
	}
	resp, err := loopClient.ListLoops(ctx, workspaceRef, query)
	if err != nil {
		return nil
	}
	var candidates []completionCandidate
	for _, item := range resp.Loops {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.Description), strings.TrimSpace(item.Catalog.Category))
		candidates = append(candidates, completionCandidate{value: name, description: description})
	}
	if resp.Aggregate != nil {
		for _, item := range resp.Aggregate.Entries {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			description := cmp.Or(strings.TrimSpace(item.Description), strings.TrimSpace(item.Catalog.Category))
			candidates = append(candidates, completionCandidate{value: name, description: description})
		}
	}
	return candidates
}

func (r *completionRuntime) automations(ctx context.Context, workspaceRef string) []completionCandidate {
	resp, err := r.client.ListAutomationJobs(ctx, AutomationJobQuery{
		WorkspaceID:   workspaceRef,
		AllWorkspaces: workspaceRef == "",
		Limit:         completionJobCap,
	})
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Jobs))
	for _, item := range resp.Jobs {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.Name), strings.TrimSpace(item.AgentName))
		candidates = append(candidates, completionCandidate{value: id, description: description})
	}
	return candidates
}

func (r *completionRuntime) worktrees(ctx context.Context, workspaceRef string) []completionCandidate {
	if workspaceRef == "" {
		return nil
	}
	resp, err := r.client.ListWorktrees(ctx, workspaceRef, false, false)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Worktrees))
	for _, item := range resp.Worktrees {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		branch := strings.TrimSpace(item.Branch)
		description := cmp.Or(name, branch, strings.TrimSpace(item.State))
		if name != "" && branch != "" && name != branch {
			description = name + " (" + branch + ")"
		}
		candidates = append(candidates, completionCandidate{value: id, description: description})
	}
	return candidates
}

func (r *completionRuntime) sessionFolders(ctx context.Context, workspaceRef string) []completionCandidate {
	if workspaceRef == "" {
		return nil
	}
	layoutClient, ok := r.client.(SessionLayoutClient)
	if !ok {
		return nil
	}
	resp, err := layoutClient.ListSessionFolders(ctx, workspaceRef)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Folders))
	for _, folder := range resp.Folders {
		id := strings.TrimSpace(folder.ID)
		if id == "" {
			continue
		}
		candidates = append(candidates, completionCandidate{
			value:       id,
			description: strings.TrimSpace(folder.Name),
		})
	}
	return candidates
}

func (r *completionRuntime) workspaceFolders(ctx context.Context) []completionCandidate {
	layoutClient, ok := r.client.(WorkspaceLayoutClient)
	if !ok {
		return nil
	}
	resp, err := layoutClient.WorkspaceLayout(ctx)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Folders))
	for _, folder := range resp.Folders {
		id := strings.TrimSpace(folder.ID)
		if id == "" {
			continue
		}
		candidates = append(candidates, completionCandidate{
			value:       id,
			description: strings.TrimSpace(folder.Name),
		})
	}
	return candidates
}

func (r *completionRuntime) extensions(ctx context.Context) []completionCandidate {
	items, err := r.client.ListExtensions(ctx)
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(items))
	for index := range items {
		item := &items[index]
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.Description), strings.TrimSpace(item.Version))
		candidates = append(candidates, completionCandidate{value: name, description: description})
	}
	return candidates
}

func (r *completionRuntime) computers(ctx context.Context) []completionCandidate {
	computers, ok := r.client.(computerListClient)
	if !ok {
		return nil
	}
	resp, err := computers.ListComputers(ctx)
	if err != nil {
		return nil
	}
	counts := make(map[string]int, len(resp.Computers))
	for _, item := range resp.Computers {
		if name := strings.TrimSpace(item.Name); name != "" {
			counts[name]++
		}
	}
	candidates := make([]completionCandidate, 0, len(resp.Computers))
	for _, item := range resp.Computers {
		name := strings.TrimSpace(item.Name)
		id := strings.TrimSpace(string(item.ID))
		useID := name == "" || counts[name] > 1 || containsWhitespace(name)
		value := name
		if useID {
			value = id
		}
		if value == "" {
			continue
		}
		description := cmp.Or(strings.TrimSpace(item.Status), id)
		if useID && name != "" {
			description = cmp.Or(name, description)
		}
		candidates = append(candidates, completionCandidate{value: value, description: description})
	}
	return candidates
}

func (r *completionRuntime) vaultSecrets(ctx context.Context) []completionCandidate {
	items, err := r.client.ListVaultSecrets(ctx, VaultListQuery{})
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(items))
	for _, item := range items {
		ref := strings.TrimSpace(item.Ref)
		if ref == "" {
			continue
		}
		namespace := strings.TrimSpace(item.Namespace)
		kind := strings.TrimSpace(item.Kind)
		description := namespace
		if namespace != "" && kind != "" {
			description = namespace + " (" + kind + ")"
		} else if kind != "" {
			description = kind
		}
		candidates = append(candidates, completionCandidate{value: ref, description: description})
	}
	return candidates
}

func (r *completionRuntime) toolsets(ctx context.Context, workspaceRef string) []completionCandidate {
	resp, err := r.client.ListToolsets(ctx, ToolQuery{WorkspaceID: workspaceRef})
	if err != nil {
		return nil
	}
	candidates := make([]completionCandidate, 0, len(resp.Toolsets))
	for _, item := range resp.Toolsets {
		id := strings.TrimSpace(string(item.ID))
		if id == "" {
			continue
		}
		candidates = append(candidates, completionCandidate{
			value:       id,
			description: strings.TrimSpace(item.Status),
		})
	}
	return candidates
}
