package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetCmdPaletteRankSignals(c *gin.Context) {
	profileLens, ok := h.resolveCmdPaletteProfileLens(c, false)
	if !ok {
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errCmdPaletteServiceUnavailable)
		return
	}
	snapshot, err := h.CmdPalette.Personalization(c.Request.Context(), profileLens, workspaceID)
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPaletteRankSignalsFromDomain(snapshot))
}

func (h *BaseHandlers) RecordCmdPaletteUsage(c *gin.Context) {
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, "", errCmdPaletteServiceUnavailable)
		return
	}
	var body contract.CmdPaletteUsageRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		c.JSON(http.StatusBadRequest, contract.CmdPaletteError{
			Error: cmdPaletteInvalidRequestError, Message: err.Error(),
		})
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, body.Workspace)
	if !ok {
		return
	}
	profileLens, ok := h.resolveCmdPaletteProfileLens(c, false)
	if !ok {
		return
	}
	if err := h.CmdPalette.RecordUsage(c.Request.Context(), cmdpalette.Usage{
		ProfileLensID: profileLens.ID,
		WorkspaceID:   workspaceID,
		CommandID:     cmdpalette.CommandID(strings.TrimSpace(string(body.CommandID))),
		Query:         body.Query,
	}); err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) PinCmdPaletteCommand(c *gin.Context) {
	h.changeCmdPalettePin(c, true)
}

func (h *BaseHandlers) UnpinCmdPaletteCommand(c *gin.Context) {
	h.changeCmdPalettePin(c, false)
}

func (h *BaseHandlers) changeCmdPalettePin(c *gin.Context, pinned bool) {
	profileLens, ok := h.resolveCmdPaletteProfileLens(c, true)
	if !ok {
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errCmdPaletteServiceUnavailable)
		return
	}
	commandID := cmdpalette.CommandID(strings.TrimSpace(c.Param("id")))
	var err error
	if pinned {
		err = h.CmdPalette.Pin(c.Request.Context(), profileLens, workspaceID, commandID)
	} else {
		err = h.CmdPalette.Unpin(c.Request.Context(), profileLens, workspaceID, commandID)
	}
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPalettePinResponse{Pinned: pinned})
}

func (h *BaseHandlers) GetCmdPalettePersonalization(c *gin.Context) {
	profileLens, ok := h.resolveCmdPaletteProfileLens(c, false)
	if !ok {
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errCmdPaletteServiceUnavailable)
		return
	}
	summary, err := h.CmdPalette.PersonalizationSummary(c.Request.Context(), profileLens, workspaceID)
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPalettePersonalizationFromDomain(summary))
}

func (h *BaseHandlers) ResetCmdPalettePersonalization(c *gin.Context) {
	profileLens, ok := h.resolveCmdPaletteProfileLens(c, true)
	if !ok {
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errCmdPaletteServiceUnavailable)
		return
	}
	if err := h.CmdPalette.ResetPersonalization(c.Request.Context(), profileLens, workspaceID); err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPalettePersonalizationResetResponse{Status: "reset"})
}
