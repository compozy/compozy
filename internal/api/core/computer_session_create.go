package core

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (h *BaseHandlers) placeSessionCreation(
	c *gin.Context,
	req *contract.CreateSessionRequest,
) (remoteproto.ComputerID, bool) {
	if req.Computer != "auto" {
		if (req.Computer != "" && req.Computer != string(computers.LocalID)) || req.ProjectComputerID != "" {
			invalidComputerBody(c)
			return "", false
		}
		return "", h.admitLocalSessionCreation(c)
	}
	if !h.validateAutoSessionCaller(c, req) {
		return "", false
	}
	placement, err := h.computerPlacement(c.Request.Context(), contract.ComputerPlacementRequest{
		Workspace: req.Workspace, ProjectComputerID: req.ProjectComputerID,
	})
	if err != nil {
		h.computerResult(c, 0, nil, err)
		return "", false
	}
	if placement.ComputerID != cmp.Or(req.ProjectComputerID, computers.LocalID) &&
		(req.ParentSessionID != "" || req.FolderID != "" || req.Worktree != "") {
		invalidComputerBody(c)
		return "", false
	}
	req.Computer, req.ProjectComputerID, req.Workspace, req.WorkspacePath = "", "", placement.WorkspaceID, ""
	if placement.ComputerID == computers.LocalID {
		return placement.ComputerID, h.admitLocalSessionCreation(c)
	}
	h.createPlacedRemoteSession(c, placement.ComputerID, req)
	return "", false
}

func (h *BaseHandlers) admitLocalSessionCreation(c *gin.Context) bool {
	if h.Computers != nil && !h.Computers.LocalSettings().LocalRunAgents {
		h.respondError(c, http.StatusConflict, admission.ErrLocalExecutionDisabled)
		return false
	}
	return true
}

func (h *BaseHandlers) validateAutoSessionCaller(c *gin.Context, req *contract.CreateSessionRequest) bool {
	ctx := c.Request.Context()
	if p, remote := remotehost.PrincipalFromContext(ctx); remote {
		RespondRemoteError(c, remotehost.OperatorRouteRejection("/c/auto/api/sessions", p.Browser))
		return false
	}
	_, app := AppTokenFromContext(ctx)
	if app || hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		c.AbortWithStatusJSON(http.StatusForbidden, contract.ErrorPayload{
			Code: "computer_placement_denied", Error: "Computer placement requires a local operator.",
		})
		return false
	}
	if computers.CarriesClaimToken(c.Request) {
		h.computerResult(c, 0, nil, computers.ErrClaimToken)
		return false
	}
	if strings.TrimSpace(req.Workspace) == "" {
		invalidComputerBody(c)
		return false
	}
	return h.computersReady(c)
}

func (h *BaseHandlers) createPlacedRemoteSession(
	c *gin.Context,
	id remoteproto.ComputerID,
	req *contract.CreateSessionRequest,
) {
	body, err := json.Marshal(req)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/sessions", bytes.NewReader(body))
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := h.Computers.RoundTrip(ctx, id, request)
	if err != nil {
		h.computerResult(c, 0, nil, err)
		return
	}
	defer DrainComputerResponse(response.Body)
	if response.StatusCode != http.StatusCreated {
		var failure contract.ErrorPayload
		if err := json.UnmarshalRead(response.Body, &failure); err != nil {
			h.respondError(c, http.StatusBadGateway, err)
			return
		}
		c.JSON(response.StatusCode, failure)
		return
	}
	var result contract.SessionResponse
	if err := json.UnmarshalRead(response.Body, &result); err != nil {
		h.respondError(c, http.StatusBadGateway, err)
		return
	}
	result.ComputerID = id
	c.JSON(response.StatusCode, result)
}
