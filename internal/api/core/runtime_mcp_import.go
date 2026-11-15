package core

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

type RuntimeMCPImporter interface {
	ImportRuntimeMCP(context.Context, settingspkg.RuntimeMCPImportRequest) (settingspkg.RuntimeMCPImportResult, error)
}

func (h *BaseHandlers) ImportRuntimeMCPServer(c *gin.Context) {
	var body contract.RuntimeMCPImportRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	service, ok := h.Settings.(RuntimeMCPImporter)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	result, err := ImportRuntimeMCP(c.Request.Context(), service, c.Param("runtime"), c.Param("name"), body)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func ImportRuntimeMCP(
	ctx context.Context,
	service RuntimeMCPImporter,
	runtime, name string,
	body contract.RuntimeMCPImportRequest,
) (contract.RuntimeMCPImportResponse, error) {
	result, err := service.ImportRuntimeMCP(ctx, settingspkg.RuntimeMCPImportRequest{
		Scope: settingspkg.ScopeKind(body.Scope), WorkspaceID: body.WorkspaceID, ProfileName: body.Profile,
		Runtime: runtime, ServerName: name, Name: body.Name,
	})
	if err != nil {
		return contract.RuntimeMCPImportResponse{}, err
	}
	payloads := settingsMCPServerItemPayloads([]settingspkg.MCPServerItem{*result.Mutation.MCPServer})
	return contract.RuntimeMCPImportResponse{MCPServer: payloads[0], Source: result.Source, Notes: result.Notes,
		Partial: result.Partial, Warnings: result.Mutation.Warnings}, nil
}
