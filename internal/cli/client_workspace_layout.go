package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

// WorkspaceLayoutClient manages root workspace folders and manual workspace order.
type WorkspaceLayoutClient interface {
	WorkspaceLayout(ctx context.Context) (contract.WorkspaceLayoutResponse, error)
	CreateWorkspaceFolder(ctx context.Context, name string) (contract.WorkspaceFolderResponse, error)
	RenameWorkspaceFolder(ctx context.Context, folderID string, name string) (contract.WorkspaceFolderResponse, error)
	DeleteWorkspaceFolder(ctx context.Context, folderID string) (contract.WorkspaceLayoutResponse, error)
	ReorderWorkspaceFolders(ctx context.Context, folderIDs []string) (contract.WorkspaceLayoutResponse, error)
	MoveWorkspace(ctx context.Context, request contract.MoveWorkspaceRequest) (contract.WorkspaceLayoutResponse, error)
	ReorderWorkspaces(
		ctx context.Context,
		request contract.ReorderWorkspacesRequest,
	) (contract.WorkspaceLayoutResponse, error)
}

var _ WorkspaceLayoutClient = (*daemonClient)(nil)

const (
	workspaceFoldersPath = "/api/workspace-folders"
	workspaceOrderPath   = "/api/workspace-order/"
)

func (c *daemonClient) WorkspaceLayout(ctx context.Context) (contract.WorkspaceLayoutResponse, error) {
	var response contract.WorkspaceLayoutResponse
	err := c.doJSON(ctx, http.MethodGet, workspaceFoldersPath, nil, nil, &response)
	return response, err
}

func (c *daemonClient) CreateWorkspaceFolder(
	ctx context.Context,
	name string,
) (contract.WorkspaceFolderResponse, error) {
	var response contract.WorkspaceFolderResponse
	err := c.doJSON(ctx, http.MethodPost, workspaceFoldersPath, nil,
		contract.CreateWorkspaceFolderRequest{Name: name}, &response)
	return response, err
}

func (c *daemonClient) RenameWorkspaceFolder(
	ctx context.Context,
	folderID string,
	name string,
) (contract.WorkspaceFolderResponse, error) {
	var response contract.WorkspaceFolderResponse
	err := c.doJSON(ctx, http.MethodPatch, workspaceFoldersPath+"/"+url.PathEscape(folderID), nil,
		contract.RenameWorkspaceFolderRequest{Name: name}, &response)
	return response, err
}

func (c *daemonClient) DeleteWorkspaceFolder(
	ctx context.Context,
	folderID string,
) (contract.WorkspaceLayoutResponse, error) {
	var response contract.WorkspaceLayoutResponse
	err := c.doJSON(ctx, http.MethodDelete, workspaceFoldersPath+"/"+url.PathEscape(folderID), nil, nil, &response)
	return response, err
}

func (c *daemonClient) ReorderWorkspaceFolders(
	ctx context.Context,
	folderIDs []string,
) (contract.WorkspaceLayoutResponse, error) {
	var response contract.WorkspaceLayoutResponse
	err := c.doJSON(ctx, http.MethodPut, workspaceOrderPath+"folders", nil,
		contract.ReorderWorkspaceFoldersRequest{FolderIDs: folderIDs}, &response)
	return response, err
}

func (c *daemonClient) MoveWorkspace(
	ctx context.Context,
	request contract.MoveWorkspaceRequest,
) (contract.WorkspaceLayoutResponse, error) {
	var response contract.WorkspaceLayoutResponse
	err := c.doJSON(ctx, http.MethodPost, workspaceOrderPath+"move", nil, request, &response)
	return response, err
}

func (c *daemonClient) ReorderWorkspaces(
	ctx context.Context,
	request contract.ReorderWorkspacesRequest,
) (contract.WorkspaceLayoutResponse, error) {
	var response contract.WorkspaceLayoutResponse
	err := c.doJSON(ctx, http.MethodPut, workspaceOrderPath+"workspaces", nil, request, &response)
	return response, err
}
