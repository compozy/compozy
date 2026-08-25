package cli

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TerminalControllerRecord struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type TerminalRecord struct {
	ID          string                    `json:"id"`
	WorkspaceID string                    `json:"workspace_id"`
	ProfileID   string                    `json:"profile_id"`
	ProfileName string                    `json:"profile_name"`
	Title       string                    `json:"title"`
	Shell       string                    `json:"shell"`
	Cwd         string                    `json:"cwd"`
	Mode        string                    `json:"mode"`
	State       string                    `json:"state"`
	Controller  *TerminalControllerRecord `json:"controller"`
	Lease       string                    `json:"lease"`
	Viewers     int                       `json:"viewers"`
	CreatedAt   time.Time                 `json:"created_at"`
}

type TerminalCreateRequest struct {
	Cwd   string `json:"cwd,omitempty"`
	Shell string `json:"shell,omitempty"`
	Cols  uint16 `json:"cols,omitempty"`
	Rows  uint16 `json:"rows,omitempty"`
	Title string `json:"title,omitempty"`
}

type TerminalExitRecord struct {
	Cause  string  `json:"cause"`
	Code   *int    `json:"code,omitempty"`
	Signal *string `json:"signal,omitempty"`
}

type TerminalAttachOptions struct {
	Mode     string
	Flow     string
	AfterSeq uint64
	Cols     uint16
	Rows     uint16
}

type TerminalClient interface {
	workspaceLookupClient
	CreateTerminal(context.Context, string, TerminalCreateRequest) (TerminalRecord, error)
	ListTerminals(context.Context, string) ([]TerminalRecord, error)
	GetTerminal(context.Context, string, string) (TerminalRecord, error)
	DeleteTerminal(context.Context, string, string, string) (TerminalExitRecord, error)
	AttachTerminal(context.Context, string, string, TerminalAttachOptions, io.Reader, io.Writer) error
}

var _ TerminalClient = (*daemonClient)(nil)

func (c *daemonClient) CreateTerminal(
	ctx context.Context,
	workspace string,
	request TerminalCreateRequest,
) (TerminalRecord, error) {
	var response struct {
		Terminal TerminalRecord `json:"terminal"`
	}
	if err := c.doJSON(ctx, http.MethodPost, terminalClientPath(workspace), nil, request, &response); err != nil {
		return TerminalRecord{}, err
	}
	return response.Terminal, nil
}

func (c *daemonClient) ListTerminals(ctx context.Context, workspace string) ([]TerminalRecord, error) {
	var response struct {
		Terminals []TerminalRecord `json:"terminals"`
	}
	if err := c.doJSON(ctx, http.MethodGet, terminalClientPath(workspace), nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Terminals, nil
}

func (c *daemonClient) GetTerminal(ctx context.Context, workspace, id string) (TerminalRecord, error) {
	var response struct {
		Terminal TerminalRecord `json:"terminal"`
	}
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TerminalRecord{}, err
	}
	return response.Terminal, nil
}

func (c *daemonClient) DeleteTerminal(
	ctx context.Context,
	workspace, id, signal string,
) (TerminalExitRecord, error) {
	var response struct {
		Exit TerminalExitRecord `json:"exit"`
	}
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, map[string]string{"signal": signal}, &response); err != nil {
		return TerminalExitRecord{}, err
	}
	return response.Exit, nil
}

func terminalClientPath(workspace string) string {
	return "/api/workspaces/" + url.PathEscape(strings.TrimSpace(workspace)) + "/terminals"
}
