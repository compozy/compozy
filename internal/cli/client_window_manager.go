package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
)

// WindowManagerStreamHandlers receives the initial fence and later topology or client updates.
type WindowManagerStreamHandlers struct {
	Snapshot func(contract.WindowManagerSnapshotFrame) error
	Event    func(contract.WindowManagerEventFrame) error
	Client   func(contract.WindowManagerClientFrame) error
}

// WindowManagerClient is the CLI transport contract for daemon-authoritative window management.
type WindowManagerClient interface {
	GetWindowManagerSnapshot(context.Context, string) (contract.WindowManagerSnapshot, error)
	PreviewWindowManagerCommand(
		context.Context,
		string,
		contract.WindowManagerCommandRequest,
	) (contract.WindowManagerPreview, error)
	ExecuteWindowManagerCommand(
		context.Context,
		string,
		contract.WindowManagerCommandRequest,
	) (contract.WindowManagerResult, error)
	ListWindowManagerClients(context.Context, string) (contract.WindowManagerClientsResponse, error)
	RegisterWindowManagerClient(
		context.Context,
		string,
		contract.WindowManagerClientRegistration,
	) (contract.WindowManagerClientView, error)
	UnregisterWindowManagerClient(context.Context, string, string) error
	ExportWindowManagerLayout(context.Context, string) (contract.WindowManagerLayoutDocument, error)
	ValidateWindowManagerLayout(
		context.Context,
		string,
		contract.WindowManagerLayoutValidationRequest,
	) (contract.WindowManagerLayoutValidationResponse, error)
	ApplyWindowManagerLayout(
		context.Context,
		string,
		contract.WindowManagerLayoutReplaceRequest,
	) (contract.WindowManagerResult, error)
	ListWindowManagerLayoutProfiles(context.Context, string) (contract.ResourcesResponse, error)
	PutWindowManagerLayoutProfile(
		context.Context,
		string,
		string,
		contract.PutResourceRequest,
	) (contract.ResourceResponse, error)
	DeleteWindowManagerLayoutProfile(
		context.Context,
		string,
		string,
		contract.DeleteResourceRequest,
	) error
	WatchWindowManager(
		context.Context,
		string,
		*windowmanager.ClientID,
		*contract.WindowManagerRevision,
		WindowManagerStreamHandlers,
	) error
}

var _ WindowManagerClient = (*unixSocketClient)(nil)

func (c *unixSocketClient) GetWindowManagerSnapshot(
	ctx context.Context,
	workspace string,
) (contract.WindowManagerSnapshot, error) {
	var response contract.WindowManagerSnapshot
	if err := c.doJSON(ctx, http.MethodGet, windowManagerClientPath(workspace), nil, nil, &response); err != nil {
		return contract.WindowManagerSnapshot{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PreviewWindowManagerCommand(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerCommandRequest,
) (contract.WindowManagerPreview, error) {
	var response contract.WindowManagerPreview
	path := windowManagerClientPath(workspace) + "/preview"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.WindowManagerPreview{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ExecuteWindowManagerCommand(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerCommandRequest,
) (contract.WindowManagerResult, error) {
	var response contract.WindowManagerResult
	path := windowManagerClientPath(workspace) + "/commands"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.WindowManagerResult{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListWindowManagerClients(
	ctx context.Context,
	workspace string,
) (contract.WindowManagerClientsResponse, error) {
	var response contract.WindowManagerClientsResponse
	path := windowManagerClientPath(workspace) + "/clients"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.WindowManagerClientsResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RegisterWindowManagerClient(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerClientRegistration,
) (contract.WindowManagerClientView, error) {
	var response contract.WindowManagerClientView
	path := windowManagerClientPath(workspace) + "/clients"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.WindowManagerClientView{}, err
	}
	return response, nil
}

func (c *unixSocketClient) UnregisterWindowManagerClient(
	ctx context.Context,
	workspace string,
	clientID string,
) error {
	path := windowManagerClientPath(workspace) + "/clients/" + url.PathEscape(strings.TrimSpace(clientID))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *unixSocketClient) ExportWindowManagerLayout(
	ctx context.Context,
	workspace string,
) (contract.WindowManagerLayoutDocument, error) {
	var response contract.WindowManagerLayoutDocument
	path := windowManagerClientPath(workspace) + "/layout"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.WindowManagerLayoutDocument{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ValidateWindowManagerLayout(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerLayoutValidationRequest,
) (contract.WindowManagerLayoutValidationResponse, error) {
	var response contract.WindowManagerLayoutValidationResponse
	path := windowManagerClientPath(workspace) + "/layout/validate"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.WindowManagerLayoutValidationResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ApplyWindowManagerLayout(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerLayoutReplaceRequest,
) (contract.WindowManagerResult, error) {
	var response contract.WindowManagerResult
	path := windowManagerClientPath(workspace) + "/layout"
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return contract.WindowManagerResult{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListWindowManagerLayoutProfiles(
	ctx context.Context,
	workspace string,
) (contract.ResourcesResponse, error) {
	var response contract.ResourcesResponse
	path := windowManagerClientPath(workspace) + "/layout-profiles"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.ResourcesResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PutWindowManagerLayoutProfile(
	ctx context.Context,
	workspace string,
	profileID string,
	request contract.PutResourceRequest,
) (contract.ResourceResponse, error) {
	var response contract.ResourceResponse
	path := windowManagerLayoutProfilePath(workspace, profileID)
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return contract.ResourceResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DeleteWindowManagerLayoutProfile(
	ctx context.Context,
	workspace string,
	profileID string,
	request contract.DeleteResourceRequest,
) error {
	path := windowManagerLayoutProfilePath(workspace, profileID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, request, nil)
}

func windowManagerLayoutProfilePath(workspace string, profileID string) string {
	return windowManagerClientPath(workspace) +
		"/layout-profiles/" + url.PathEscape(strings.TrimSpace(profileID))
}

func windowManagerClientPath(workspace string) string {
	workspaceID := windowmanager.WorkspaceID(strings.TrimSpace(workspace))
	return "/api/workspaces/" + url.PathEscape(string(workspaceID)) + "/window-manager"
}
