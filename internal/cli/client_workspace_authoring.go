package cli

import (
	"context"

	"errors"
	"fmt"

	"net/http"
	"net/url"
	"path/filepath"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type workspacePathNotRegisteredError struct {
	path string
}

func (e *workspacePathNotRegisteredError) Error() string {
	if e == nil {
		return "cli: workspace path is not registered"
	}
	return fmt.Sprintf("cli: workspace path %q is not registered", e.path)
}

func (c *unixSocketClient) CreateWorkspace(
	ctx context.Context,
	request WorkspaceCreateRequest,
) (WorkspaceRecord, error) {
	var response struct {
		Workspace WorkspaceRecord `json:"workspace"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/workspaces", nil, request, &response); err != nil {
		return WorkspaceRecord{}, err
	}
	return response.Workspace, nil
}

func (c *unixSocketClient) ListWorkspaces(ctx context.Context) ([]WorkspaceRecord, error) {
	var response struct {
		Workspaces []WorkspaceRecord `json:"workspaces"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/workspaces", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Workspaces, nil
}

func (c *unixSocketClient) GetWorkspace(ctx context.Context, ref string) (WorkspaceDetailRecord, error) {
	routeRef, err := c.workspaceRouteRef(ctx, ref)
	if err != nil {
		return WorkspaceDetailRecord{}, err
	}
	var response WorkspaceDetailRecord
	path := "/api/workspaces/" + url.PathEscape(routeRef)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return WorkspaceDetailRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) workspaceRouteRef(ctx context.Context, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errWorkspaceReferenceRequired
	}
	if !workspaceRefLooksLikePath(trimmed) {
		return trimmed, nil
	}
	target, err := canonicalCLIWorkspacePath(trimmed)
	if err != nil {
		return "", err
	}
	workspaces, err := c.ListWorkspaces(ctx)
	if err != nil {
		return "", fmt.Errorf("cli: list workspaces before resolving path %q: %w", target, err)
	}
	return nearestCLIWorkspaceRoute(target, workspaces)
}

func nearestCLIWorkspaceRoute(target string, workspaces []WorkspaceRecord) (string, error) {
	target, err := canonicalCLIWorkspacePath(target)
	if err != nil {
		return "", err
	}
	var (
		selectedID   string
		selectedRoot string
	)
	for _, workspace := range workspaces {
		root, err := canonicalCLIWorkspacePath(workspace.RootDir)
		if err != nil {
			continue
		}
		contains, err := cliWorkspaceRootContains(root, target)
		if err != nil || !contains {
			continue
		}
		id := strings.TrimSpace(workspace.ID)
		if id == "" {
			continue
		}
		if len(root) > len(selectedRoot) || (len(root) == len(selectedRoot) && id < selectedID) {
			selectedID = id
			selectedRoot = root
		}
	}
	if selectedID != "" {
		return selectedID, nil
	}
	return "", &workspacePathNotRegisteredError{path: target}
}

func cliWorkspaceRootContains(root string, target string) (bool, error) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false, fmt.Errorf("cli: compare workspace root %q with path %q: %w", root, target, err)
	}
	return relative == "." ||
		(relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))), nil
}

func workspaceRefLooksLikePath(ref string) bool {
	return filepath.IsAbs(ref) ||
		strings.HasPrefix(ref, ".") ||
		strings.Contains(ref, string(filepath.Separator))
}

func canonicalCLIWorkspacePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("cli: workspace path is required")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("cli: resolve workspace path %q: %w", path, err)
	}
	cleaned := filepath.Clean(abs)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err == nil {
		return resolved, nil
	}
	return "", fmt.Errorf("cli: resolve workspace path %q: %w", cleaned, err)
}

func (c *unixSocketClient) UpdateWorkspace(
	ctx context.Context,
	ref string,
	request WorkspaceUpdateRequest,
) (WorkspaceRecord, error) {
	var response struct {
		Workspace WorkspaceRecord `json:"workspace"`
	}
	path := "/api/workspaces/" + url.PathEscape(strings.TrimSpace(ref))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return WorkspaceRecord{}, err
	}
	return response.Workspace, nil
}

func (c *unixSocketClient) DeleteWorkspace(ctx context.Context, ref string) error {
	path := "/api/workspaces/" + url.PathEscape(strings.TrimSpace(ref))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *unixSocketClient) GetAgentSoul(
	ctx context.Context,
	name string,
	query AgentQuery,
) (AgentSoulRecord, error) {
	var response AgentSoulRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/soul"
	if err := c.doJSON(ctx, http.MethodGet, path, agentValues(query), nil, &response); err != nil {
		return AgentSoulRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ValidateAgentSoul(
	ctx context.Context,
	name string,
	request AgentSoulValidateRequest,
) (AgentSoulRecord, error) {
	var response AgentSoulRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/soul/validate"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return AgentSoulRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PutAgentSoul(
	ctx context.Context,
	name string,
	request AgentSoulPutRequest,
) (AgentSoulMutationRecord, error) {
	var response AgentSoulMutationRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/soul"
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return AgentSoulMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DeleteAgentSoul(
	ctx context.Context,
	name string,
	request AgentSoulDeleteRequest,
) (AgentSoulMutationRecord, error) {
	var response AgentSoulMutationRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/soul"
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, request, &response); err != nil {
		return AgentSoulMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListAgentSoulHistory(
	ctx context.Context,
	name string,
	request AgentSoulHistoryRequest,
) (AgentSoulHistoryRecord, error) {
	var response AgentSoulHistoryRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/soul/history"
	if err := c.doJSON(ctx, http.MethodGet, path, agentSoulHistoryValues(request), nil, &response); err != nil {
		return AgentSoulHistoryRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RollbackAgentSoul(
	ctx context.Context,
	name string,
	request AgentSoulRollbackRequest,
) (AgentSoulMutationRecord, error) {
	var response AgentSoulMutationRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/soul/rollback"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return AgentSoulMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetAgentHeartbeat(
	ctx context.Context,
	name string,
	query AgentQuery,
) (AgentHeartbeatRecord, error) {
	var response AgentHeartbeatRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat"
	if err := c.doJSON(ctx, http.MethodGet, path, agentValues(query), nil, &response); err != nil {
		return AgentHeartbeatRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ValidateAgentHeartbeat(
	ctx context.Context,
	name string,
	request AgentHeartbeatValidateRequest,
) (AgentHeartbeatRecord, error) {
	var response AgentHeartbeatRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat/validate"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return AgentHeartbeatRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PutAgentHeartbeat(
	ctx context.Context,
	name string,
	request AgentHeartbeatPutRequest,
) (AgentHeartbeatMutationRecord, error) {
	var response AgentHeartbeatMutationRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat"
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return AgentHeartbeatMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DeleteAgentHeartbeat(
	ctx context.Context,
	name string,
	request AgentHeartbeatDeleteRequest,
) (AgentHeartbeatMutationRecord, error) {
	var response AgentHeartbeatMutationRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat"
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, request, &response); err != nil {
		return AgentHeartbeatMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListAgentHeartbeatHistory(
	ctx context.Context,
	name string,
	request AgentHeartbeatHistoryRequest,
) (AgentHeartbeatHistoryRecord, error) {
	var response AgentHeartbeatHistoryRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat/history"
	if err := c.doJSON(ctx, http.MethodGet, path, agentHeartbeatHistoryValues(request), nil, &response); err != nil {
		return AgentHeartbeatHistoryRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RollbackAgentHeartbeat(
	ctx context.Context,
	name string,
	request AgentHeartbeatRollbackRequest,
) (AgentHeartbeatMutationRecord, error) {
	var response AgentHeartbeatMutationRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat/rollback"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return AgentHeartbeatMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetAgentHeartbeatStatus(
	ctx context.Context,
	name string,
	request AgentHeartbeatStatusRequest,
) (AgentHeartbeatStatusRecord, error) {
	var response AgentHeartbeatStatusRecord
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat/status"
	if err := c.doJSON(ctx, http.MethodGet, path, agentHeartbeatStatusValues(request), nil, &response); err != nil {
		return AgentHeartbeatStatusRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) WakeAgentHeartbeat(
	ctx context.Context,
	name string,
	request AgentHeartbeatWakeRequest,
) (AgentHeartbeatWakeDecisionRecord, error) {
	var response contract.HeartbeatWakeResponse
	path := "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/heartbeat/wake"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return AgentHeartbeatWakeDecisionRecord{}, err
	}
	return response.Decision, nil
}
