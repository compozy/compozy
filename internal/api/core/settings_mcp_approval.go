package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type MCPSettingsApproval interface {
	SetMCPApproval(context.Context, settingspkg.MCPAuthTargetRequest, string, bool) (store.MCPApproval, error)
}

func (h *BaseHandlers) PutSettingsMCPApproval(c *gin.Context) {
	var body contract.SettingsMCPApprovalRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	h.setSettingsMCPApproval(c, body, false)
}

func (h *BaseHandlers) DeleteSettingsMCPApproval(c *gin.Context) {
	h.setSettingsMCPApproval(
		c,
		contract.SettingsMCPApprovalRequest{WorkspaceID: c.Query("workspace_id"), Profile: c.Query("profile")},
		true,
	)
}

func (h *BaseHandlers) setSettingsMCPApproval(c *gin.Context, body contract.SettingsMCPApprovalRequest, revoke bool) {
	service, ok := h.Settings.(MCPSettingsApproval)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	if body.WorkspaceID == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("workspace_id is required"))
		return
	}
	scope := settingspkg.ScopeWorkspace
	if body.Profile != "" {
		scope = settingspkg.ScopeProfile
	}
	approval, err := service.SetMCPApproval(
		c.Request.Context(),
		settingspkg.MCPAuthTargetRequest{
			Scope:       scope,
			WorkspaceID: body.WorkspaceID,
			ProfileName: body.Profile,
			Name:        c.Param("name"),
		},
		body.DefinitionDigest,
		revoke,
	)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	if revoke {
		c.Status(http.StatusNoContent)
		return
	}
	profile := body.Profile
	if profile == "" {
		profile = "default"
	}
	c.JSON(
		http.StatusOK,
		contract.SettingsMCPApprovalResponse{
			Approval: contract.SettingsMCPApprovalPayload{
				ServerName:  approval.ServerName,
				WorkspaceID: approval.WorkspaceID,
				Profile:     profile,
				State:       "allowed",
				ApprovedAt:  approval.ApprovedAt,
			},
		},
	)
}
