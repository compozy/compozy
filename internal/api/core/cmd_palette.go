package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

const (
	cmdPaletteClientAttachmentHeader = "X-Compozy-Client-Token"
	cmdPaletteInvalidRequestError    = "invalid_request"
)

func (h *BaseHandlers) ListCmdPaletteCommands(c *gin.Context) {
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errors.New("cmd palette service is unavailable"))
		return
	}
	catalog, err := h.CmdPalette.Catalog(
		c.Request.Context(), workspaceID, cmdpalette.ClientID(strings.TrimSpace(c.Query("client"))),
	)
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPaletteCommandsFromDomain(catalog))
}

func (h *BaseHandlers) ListCmdPaletteClients(c *gin.Context) {
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errors.New("cmd palette service is unavailable"))
		return
	}
	clients, err := h.CmdPalette.Clients(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPaletteClientsFromDomain(clients))
}

func (h *BaseHandlers) InvokeCmdPaletteCommand(c *gin.Context) {
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, "", errors.New("cmd palette service is unavailable"))
		return
	}
	var body contract.CmdPaletteInvokeRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		c.JSON(
			http.StatusBadRequest,
			contract.CmdPaletteError{Error: cmdPaletteInvalidRequestError, Message: err.Error()},
		)
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, body.Workspace)
	if !ok {
		return
	}
	token := strings.TrimSpace(c.GetHeader(cmdPaletteClientAttachmentHeader))
	caller := cmdpalette.CallerControlPlane
	if token != "" {
		caller = cmdpalette.CallerAttachedClient
	}
	result, err := h.CmdPalette.Invoke(c.Request.Context(), cmdpalette.InvokeRequest{
		WorkspaceID: workspaceID, CommandID: cmdpalette.CommandID(strings.TrimSpace(c.Param("id"))),
		Args: body.Args, ClientID: cmdpalette.ClientID(strings.TrimSpace(body.Client)),
		ClientToken: token, Caller: caller,
	})
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	status := http.StatusOK
	if result.Status == cmdpalette.InvokeStatusApprovalPending {
		status = http.StatusAccepted
	}
	c.JSON(status, contract.CmdPaletteInvokeResult{
		Status: result.Status, Result: result.Result, ApprovalID: result.ApprovalID,
	})
}

func (h *BaseHandlers) resolveCmdPaletteWorkspace(
	c *gin.Context,
	raw string,
) (cmdpalette.WorkspaceID, bool) {
	workspaceRef := strings.TrimSpace(raw)
	if workspaceRef == "" {
		c.JSON(http.StatusBadRequest, contract.CmdPaletteError{
			Error: "invalid_workspace", Message: "workspace is required",
		})
		return "", false
	}
	if h.Workspaces == nil {
		h.respondError(c, http.StatusServiceUnavailable, workspacepkg.ErrWorkspaceResolverUnavailable)
		return "", false
	}
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRef)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return "", false
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		h.respondError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("%s: resolved workspace id is empty", h.transportName()),
		)
		return "", false
	}
	return cmdpalette.WorkspaceID(workspaceID), true
}
