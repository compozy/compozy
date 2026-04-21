package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
)

// DaemonStatus loads the primary daemon status payload.
func (c *Client) DaemonStatus(ctx context.Context) (apicore.DaemonStatus, error) {
	if c == nil {
		return apicore.DaemonStatus{}, ErrDaemonClientRequired
	}

	var response contract.DaemonStatusResponse
	if _, err := c.doJSON(ctx, http.MethodGet, "/api/daemon/status", nil, &response); err != nil {
		return apicore.DaemonStatus{}, err
	}
	return response.Daemon, nil
}

// StopDaemon requests daemon shutdown.
func (c *Client) StopDaemon(ctx context.Context, force bool) error {
	if c == nil {
		return ErrDaemonClientRequired
	}

	path := "/api/daemon/stop"
	if force {
		path += "?force=true"
	}
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, nil)
	return err
}

// RegisterWorkspace registers one workspace explicitly.
func (c *Client) RegisterWorkspace(
	ctx context.Context,
	path string,
	name string,
) (apicore.WorkspaceRegisterResult, error) {
	if c == nil {
		return apicore.WorkspaceRegisterResult{}, ErrDaemonClientRequired
	}

	var response contract.WorkspaceResponse
	statusCode, err := c.doJSON(ctx, http.MethodPost, "/api/workspaces", contract.WorkspaceRegisterRequest{
		Path: strings.TrimSpace(path),
		Name: strings.TrimSpace(name),
	}, &response)
	if err != nil {
		return apicore.WorkspaceRegisterResult{}, err
	}
	return apicore.WorkspaceRegisterResult{
		Workspace: response.Workspace,
		Created:   statusCode == http.StatusCreated,
	}, nil
}

// ListWorkspaces loads registered workspaces.
func (c *Client) ListWorkspaces(ctx context.Context) ([]apicore.Workspace, error) {
	if c == nil {
		return nil, ErrDaemonClientRequired
	}

	var response contract.WorkspaceListResponse
	if _, err := c.doJSON(ctx, http.MethodGet, "/api/workspaces", nil, &response); err != nil {
		return nil, err
	}
	return response.Workspaces, nil
}

// GetWorkspace loads one workspace by id or path key.
func (c *Client) GetWorkspace(ctx context.Context, ref string) (apicore.Workspace, error) {
	if c == nil {
		return apicore.Workspace{}, ErrDaemonClientRequired
	}

	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return apicore.Workspace{}, errors.New("workspace ref is required")
	}

	var response contract.WorkspaceResponse
	path := "/api/workspaces/" + url.PathEscape(trimmedRef)
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return apicore.Workspace{}, err
	}
	return response.Workspace, nil
}

// DeleteWorkspace unregisters one workspace.
func (c *Client) DeleteWorkspace(ctx context.Context, ref string) error {
	if c == nil {
		return ErrDaemonClientRequired
	}

	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return errors.New("workspace ref is required")
	}

	path := "/api/workspaces/" + url.PathEscape(trimmedRef)
	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// ResolveWorkspace resolves or lazily registers one workspace path.
func (c *Client) ResolveWorkspace(ctx context.Context, path string) (apicore.Workspace, error) {
	if c == nil {
		return apicore.Workspace{}, ErrDaemonClientRequired
	}

	var response contract.WorkspaceResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/api/workspaces/resolve", contract.WorkspaceResolveRequest{
		Path: strings.TrimSpace(path),
	}, &response); err != nil {
		return apicore.Workspace{}, err
	}
	return response.Workspace, nil
}

// ListTaskWorkflows loads synced workflow summaries for one workspace.
func (c *Client) ListTaskWorkflows(ctx context.Context, workspace string) ([]apicore.WorkflowSummary, error) {
	if c == nil {
		return nil, ErrDaemonClientRequired
	}

	values := url.Values{}
	if trimmedWorkspace := strings.TrimSpace(workspace); trimmedWorkspace != "" {
		values.Set("workspace", trimmedWorkspace)
	}

	var response contract.TaskWorkflowListResponse
	path := "/api/tasks"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Workflows, nil
}

// ArchiveTaskWorkflow archives one workflow through the daemon API.
func (c *Client) ArchiveTaskWorkflow(
	ctx context.Context,
	workspace string,
	slug string,
) (apicore.ArchiveResult, error) {
	if c == nil {
		return apicore.ArchiveResult{}, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return apicore.ArchiveResult{}, ErrWorkflowSlugRequired
	}

	var response contract.ArchiveResponse
	path := "/api/tasks/" + url.PathEscape(trimmedSlug) + "/archive"
	if _, err := c.doJSON(ctx, http.MethodPost, path, contract.WorkflowRefRequest{
		Workspace: strings.TrimSpace(workspace),
	}, &response); err != nil {
		return apicore.ArchiveResult{}, err
	}
	return response, nil
}

// SyncWorkflow runs explicit daemon-backed reconciliation.
func (c *Client) SyncWorkflow(ctx context.Context, req apicore.SyncRequest) (apicore.SyncResult, error) {
	if c == nil {
		return apicore.SyncResult{}, ErrDaemonClientRequired
	}

	var response contract.SyncResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/api/sync", req, &response); err != nil {
		return apicore.SyncResult{}, err
	}
	return response, nil
}
