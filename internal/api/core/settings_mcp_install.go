package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

// MCPSettingsInstaller exposes feed-aware MCP installation to API transports.
type MCPSettingsInstaller interface {
	InstallMCPCatalog(
		ctx context.Context,
		req settingspkg.MCPCatalogInstallRequest,
	) (settingspkg.MCPCatalogInstallResult, error)
}

// InstallSettingsMCPServer installs one curated MCP template through the settings lifecycle.
func (h *BaseHandlers) InstallSettingsMCPServer(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	req, err := parseInstallSettingsMCPServerRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	result, err := h.Settings.InstallMCPCatalog(
		settingspkg.WithMutationSource(c.Request.Context(), h.TransportName),
		req,
	)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	payloads := settingsMCPServerItemPayloads([]settingspkg.MCPServerItem{result.Item})
	if len(payloads) != 1 {
		h.respondError(c, http.StatusInternalServerError, errors.New("settings: map installed MCP server response"))
		return
	}
	c.JSON(http.StatusOK, contract.InstallSettingsMCPServerResponse{
		MCPServer: payloads[0],
		Apply:     SettingsApplyResponseFromResult(result.Apply),
		NextStep:  contract.SettingsMCPInstallNextStep(result.NextStep),
		Warnings:  append([]contract.DiagnosticItem(nil), result.Warnings...),
	})
}

func parseInstallSettingsMCPServerRequest(c *gin.Context) (settingspkg.MCPCatalogInstallRequest, error) {
	var body struct {
		EntryID     string                              `json:"entry_id"`
		Name        string                              `json:"name,omitempty"`
		Scope       contract.SettingsWorkspaceScopeKind `json:"scope"`
		WorkspaceID string                              `json:"workspace_id,omitempty"`
		Values      json.RawMessage                     `json:"values"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		return settingspkg.MCPCatalogInstallRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode MCP catalog install request: %w", err),
		)
	}
	if strings.TrimSpace(body.EntryID) == "" {
		return settingspkg.MCPCatalogInstallRequest{}, NewSettingsValidationError(
			errors.New("mcp-servers.install.entry_id is required"),
		)
	}
	if strings.TrimSpace(string(body.Scope)) == "" {
		return settingspkg.MCPCatalogInstallRequest{}, NewSettingsValidationError(
			errors.New("mcp-servers.install.scope is required"),
		)
	}
	if body.Values == nil {
		return settingspkg.MCPCatalogInstallRequest{}, NewSettingsValidationError(
			errors.New("mcp-servers.install.values is required"),
		)
	}
	var valuesPayload *contract.SettingsMCPCatalogInstallValuesPayload
	if err := decodeStrictJSONValue(body.Values, &valuesPayload); err != nil {
		return settingspkg.MCPCatalogInstallRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode mcp-servers.install.values: %w", err),
		)
	}
	values := settingspkg.MCPCatalogInstallValues{}
	if valuesPayload != nil {
		values.Env = make(map[string]settingspkg.MCPSecretInput, len(valuesPayload.Env))
		for key, input := range valuesPayload.Env {
			values.Env[key] = settingspkg.MCPSecretInput{Value: input.Value, VaultRef: input.VaultRef}
		}
		if len(values.Env) == 0 {
			values.Env = nil
		}
		if valuesPayload.OAuthClientSecret != nil {
			values.OAuthClientSecret = &settingspkg.MCPSecretInput{
				Value:    valuesPayload.OAuthClientSecret.Value,
				VaultRef: valuesPayload.OAuthClientSecret.VaultRef,
			}
		}
	}
	return settingspkg.MCPCatalogInstallRequest{
		EntryID:     strings.TrimSpace(body.EntryID),
		Name:        strings.TrimSpace(body.Name),
		Scope:       settingspkg.ScopeKind(strings.TrimSpace(string(body.Scope))),
		WorkspaceID: strings.TrimSpace(body.WorkspaceID),
		Values:      values,
	}, nil
}

func decodeStrictJSONValue(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return err
	} else if err == nil {
		return errors.New("JSON value must contain a single value")
	}
	return nil
}
