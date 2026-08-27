package cli

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

type TerminalCreateRequest struct {
	Cwd   string `json:"cwd,omitempty"`
	Shell string `json:"shell,omitempty"`
	Cols  uint16 `json:"cols,omitempty"`
	Rows  uint16 `json:"rows,omitempty"`
	Title string `json:"title,omitempty"`
}

type TerminalExitRecord struct {
	Cause  string    `json:"cause"`
	Code   *int      `json:"code,omitempty"`
	Signal *string   `json:"signal,omitempty"`
	At     time.Time `json:"at"`
}

type TerminalAttachOptions struct {
	Mode     string
	Flow     string
	AfterSeq uint64
	Cols     uint16
	Rows     uint16
	Takeover bool
	Force    bool
}

type TerminalClient interface {
	workspaceLookupClient
	CreateTerminal(context.Context, string, TerminalCreateRequest) (contract.TerminalInfoPayload, error)
	ListTerminals(context.Context, string) ([]contract.TerminalInfoPayload, error)
	GetTerminal(context.Context, string, string) (contract.TerminalInfoPayload, error)
	DeleteTerminal(context.Context, string, string, string) (TerminalExitRecord, error)
	AttachTerminal(context.Context, string, string, TerminalAttachOptions, io.Reader, io.Writer) error
}

var _ TerminalClient = (*daemonClient)(nil)

func (c *daemonClient) CreateTerminal(
	ctx context.Context,
	workspace string,
	request TerminalCreateRequest,
) (contract.TerminalInfoPayload, error) {
	var response contract.TerminalResponse
	if err := c.doJSON(ctx, http.MethodPost, terminalClientPath(workspace), nil, request, &response); err != nil {
		return contract.TerminalInfoPayload{}, err
	}
	return response.Terminal, nil
}

func (c *daemonClient) ListTerminals(
	ctx context.Context,
	workspace string,
) ([]contract.TerminalInfoPayload, error) {
	var response contract.TerminalListResponse
	if err := c.doJSON(ctx, http.MethodGet, terminalClientPath(workspace), nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Terminals, nil
}

func (c *daemonClient) GetTerminal(
	ctx context.Context,
	workspace, id string,
) (contract.TerminalInfoPayload, error) {
	var response contract.TerminalResponse
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.TerminalInfoPayload{}, err
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
