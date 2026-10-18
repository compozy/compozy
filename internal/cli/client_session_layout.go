package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

// SessionLayoutClient manages workspace session folders and manual session order.
type SessionLayoutClient interface {
	ListSessionFolders(ctx context.Context, workspace string) (contract.SessionFoldersResponse, error)
	CreateSessionFolder(ctx context.Context, workspace string, name string) (contract.SessionFolderResponse, error)
	RenameSessionFolder(
		ctx context.Context,
		workspace string,
		folderID string,
		name string,
	) (contract.SessionFolderResponse, error)
	DeleteSessionFolder(ctx context.Context, workspace string, folderID string) (contract.SessionFoldersResponse, error)
	ReorderSessionFolders(
		ctx context.Context,
		workspace string,
		folderIDs []string,
	) (contract.SessionFoldersResponse, error)
	MoveSession(ctx context.Context, sessionID string, folderID string) (contract.SessionFoldersResponse, error)
	ReorderSessions(
		ctx context.Context,
		workspace string,
		request contract.ReorderSessionsRequest,
	) (contract.SessionFoldersResponse, error)
}

var _ SessionLayoutClient = (*daemonClient)(nil)

func sessionFoldersPath(workspace string) string {
	return "/api/workspaces/" + url.PathEscape(workspace) + "/session-folders"
}

func sessionOrderPath(workspace string, target string) string {
	return "/api/workspaces/" + url.PathEscape(workspace) + "/session-order/" + target
}

func (c *daemonClient) ListSessionFolders(
	ctx context.Context,
	workspace string,
) (contract.SessionFoldersResponse, error) {
	var response contract.SessionFoldersResponse
	err := c.doJSON(ctx, http.MethodGet, sessionFoldersPath(workspace), nil, nil, &response)
	return response, err
}

func (c *daemonClient) CreateSessionFolder(
	ctx context.Context,
	workspace string,
	name string,
) (contract.SessionFolderResponse, error) {
	var response contract.SessionFolderResponse
	err := c.doJSON(ctx, http.MethodPost, sessionFoldersPath(workspace), nil,
		contract.CreateSessionFolderRequest{Name: name}, &response)
	return response, err
}

func (c *daemonClient) RenameSessionFolder(
	ctx context.Context,
	workspace string,
	folderID string,
	name string,
) (contract.SessionFolderResponse, error) {
	var response contract.SessionFolderResponse
	err := c.doJSON(ctx, http.MethodPatch, sessionFoldersPath(workspace)+"/"+url.PathEscape(folderID), nil,
		contract.RenameSessionFolderRequest{Name: name}, &response)
	return response, err
}

func (c *daemonClient) DeleteSessionFolder(
	ctx context.Context,
	workspace string,
	folderID string,
) (contract.SessionFoldersResponse, error) {
	var response contract.SessionFoldersResponse
	err := c.doJSON(ctx, http.MethodDelete, sessionFoldersPath(workspace)+"/"+url.PathEscape(folderID), nil, nil,
		&response)
	return response, err
}

func (c *daemonClient) ReorderSessionFolders(
	ctx context.Context,
	workspace string,
	folderIDs []string,
) (contract.SessionFoldersResponse, error) {
	var response contract.SessionFoldersResponse
	err := c.doJSON(ctx, http.MethodPut, sessionOrderPath(workspace, "folders"), nil,
		contract.ReorderSessionFoldersRequest{FolderIDs: folderIDs}, &response)
	return response, err
}

func (c *daemonClient) MoveSession(
	ctx context.Context,
	sessionID string,
	folderID string,
) (contract.SessionFoldersResponse, error) {
	var response contract.SessionFoldersResponse
	path, err := c.sessionScopedPath(ctx, sessionID, "/move")
	if err != nil {
		return response, err
	}
	err = c.doJSON(ctx, http.MethodPost, path, nil, contract.MoveSessionRequest{FolderID: folderID}, &response)
	return response, err
}

func (c *daemonClient) ReorderSessions(
	ctx context.Context,
	workspace string,
	request contract.ReorderSessionsRequest,
) (contract.SessionFoldersResponse, error) {
	var response contract.SessionFoldersResponse
	err := c.doJSON(ctx, http.MethodPut, sessionOrderPath(workspace, "sessions"), nil, request, &response)
	return response, err
}
