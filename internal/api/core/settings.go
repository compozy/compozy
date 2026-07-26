package core

import (
	"bufio"
	"errors"
	"net/http"
	"strings"
	"time"

	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

const (
	settingsErrorKey = "error"
)

const (
	settingsRestartStatusPathPrefix  = "/api/settings/actions/restart/"
	settingsObservabilityLogTailPath = "/api/settings/observability/log-tail"
)

var (
	errSettingsServiceUnavailable = errors.New("settings service is not configured")
	errSettingsRestartUnavailable = errors.New("settings restart controller is not configured")
	errSettingsUpdateUnavailable  = errors.New("settings update controller is not configured")
)

// SettingsLogTailEventPayload is the shared SSE payload for daemon log tailing.
type SettingsLogTailEventPayload struct {
	Line string `json:"line"`
}

// GetSettingsGeneral returns the general settings section.
func (h *BaseHandlers) GetSettingsGeneral(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionGeneral)
}

// UpdateSettingsGeneral persists the general settings section.
func (h *BaseHandlers) UpdateSettingsGeneral(c *gin.Context) {
	req, err := parseUpdateSettingsGeneralRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsMemory returns the memory settings section.
func (h *BaseHandlers) GetSettingsMemory(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionMemory)
}

// UpdateSettingsMemory persists the memory settings section.
func (h *BaseHandlers) UpdateSettingsMemory(c *gin.Context) {
	req, err := parseUpdateSettingsMemoryRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	if err := h.validateSettingsMemoryProvider(c.Request.Context(), req); err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsRoles returns the background-role routing settings section.
func (h *BaseHandlers) GetSettingsRoles(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionRoles)
}

// UpdateSettingsRoles persists the background-role routing settings section.
func (h *BaseHandlers) UpdateSettingsRoles(c *gin.Context) {
	req, err := parseUpdateSettingsRolesRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsSkills returns the skills settings section.
func (h *BaseHandlers) GetSettingsSkills(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionSkills)
}

// UpdateSettingsSkills persists the skills settings section.
func (h *BaseHandlers) UpdateSettingsSkills(c *gin.Context) {
	req, err := parseUpdateSettingsSkillsRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// ReloadSettings reconciles desired config.toml with the daemon active generation.
func (h *BaseHandlers) ReloadSettings(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	result, err := h.Settings.Reload(
		settingspkg.WithMutationSource(c.Request.Context(), h.TransportName),
	)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, SettingsApplyResponseFromResult(result))
}

// ListSettingsApplyRecords returns persisted config apply history.
func (h *BaseHandlers) ListSettingsApplyRecords(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	filter, err := parseConfigApplyRecordFilter(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	records, err := h.Settings.ListApplyRecords(c.Request.Context(), filter)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, ConfigApplyRecordsResponseFromRecords(records))
}

// GetSettingsAutomation returns the automation settings section.
func (h *BaseHandlers) GetSettingsAutomation(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionAutomation)
}

// UpdateSettingsAutomation persists the automation settings section.
func (h *BaseHandlers) UpdateSettingsAutomation(c *gin.Context) {
	req, err := parseUpdateSettingsAutomationRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsNetwork returns the network settings section.
func (h *BaseHandlers) GetSettingsNetwork(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionNetwork)
}

// UpdateSettingsNetwork persists the network settings section.
func (h *BaseHandlers) UpdateSettingsNetwork(c *gin.Context) {
	req, err := parseUpdateSettingsNetworkRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsWindowManager returns the window-manager settings section.
func (h *BaseHandlers) GetSettingsWindowManager(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionWindowManager)
}

// UpdateSettingsWindowManager persists the window-manager settings section.
func (h *BaseHandlers) UpdateSettingsWindowManager(c *gin.Context) {
	req, err := parseUpdateSettingsWindowManagerRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsObservability returns the observability settings section.
func (h *BaseHandlers) GetSettingsObservability(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionObservability)
}

// UpdateSettingsObservability persists the observability settings section.
func (h *BaseHandlers) UpdateSettingsObservability(c *gin.Context) {
	req, err := parseUpdateSettingsObservabilityRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// GetSettingsHooksExtensions returns the hooks and extensions settings section.
func (h *BaseHandlers) GetSettingsHooksExtensions(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionHooksExtensions)
}

// GetSettingsUpdate returns the current software update status snapshot.
func (h *BaseHandlers) GetSettingsUpdate(c *gin.Context) {
	if h.SettingsUpdate == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsUpdateUnavailable)
		return
	}

	status, err := h.SettingsUpdate.GetUpdate(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusOK, SettingsUpdateResponseFromStatus(status))
}

// UpdateSettingsHooksExtensions persists the hooks and extensions settings section.
func (h *BaseHandlers) UpdateSettingsHooksExtensions(c *gin.Context) {
	req, err := parseUpdateSettingsHooksExtensionsRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}

// ListSettingsProviders returns the provider settings collection.
func (h *BaseHandlers) ListSettingsProviders(c *gin.Context) {
	h.listSettingsCollection(c, settingspkg.CollectionProviders)
}

// GetSettingsProvider returns one provider settings item.
func (h *BaseHandlers) GetSettingsProvider(c *gin.Context) {
	h.getSettingsCollectionItem(c, settingspkg.CollectionProviders)
}

// PutSettingsProvider upserts one provider settings item.
func (h *BaseHandlers) PutSettingsProvider(c *gin.Context) {
	req, err := parsePutSettingsProviderRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.putSettingsCollectionItem(c, req)
}

// DeleteSettingsProvider deletes one provider settings item.
func (h *BaseHandlers) DeleteSettingsProvider(c *gin.Context) {
	req, err := parseDeleteSettingsCollectionRequest(c, settingspkg.CollectionProviders)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.deleteSettingsCollectionItem(c, req)
}

// ListSettingsMCPServers returns the MCP server settings collection.
func (h *BaseHandlers) ListSettingsMCPServers(c *gin.Context) {
	h.listSettingsCollection(c, settingspkg.CollectionMCPServers)
}

// PutSettingsMCPServer upserts one MCP server settings item.
func (h *BaseHandlers) PutSettingsMCPServer(c *gin.Context) {
	req, err := parsePutSettingsMCPServerRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.putSettingsCollectionItem(c, req)
}

// DeleteSettingsMCPServer deletes one MCP server settings item.
func (h *BaseHandlers) DeleteSettingsMCPServer(c *gin.Context) {
	req, err := parseDeleteSettingsCollectionRequest(c, settingspkg.CollectionMCPServers)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.deleteSettingsCollectionItem(c, req)
}

// ListSettingsSandboxes returns the sandbox settings collection.
func (h *BaseHandlers) ListSettingsSandboxes(c *gin.Context) {
	h.listSettingsCollection(c, settingspkg.CollectionSandboxes)
}

// GetSettingsSandbox returns one sandbox settings item.
func (h *BaseHandlers) GetSettingsSandbox(c *gin.Context) {
	h.getSettingsCollectionItem(c, settingspkg.CollectionSandboxes)
}

// PutSettingsSandbox upserts one sandbox settings item.
func (h *BaseHandlers) PutSettingsSandbox(c *gin.Context) {
	req, err := parsePutSettingsSandboxRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.putSettingsCollectionItem(c, req)
}

// DeleteSettingsSandbox deletes one sandbox settings item.
func (h *BaseHandlers) DeleteSettingsSandbox(c *gin.Context) {
	req, err := parseDeleteSettingsCollectionRequest(c, settingspkg.CollectionSandboxes)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.deleteSettingsCollectionItem(c, req)
}

// ListSettingsHooks returns the hook settings collection.
func (h *BaseHandlers) ListSettingsHooks(c *gin.Context) {
	h.listSettingsCollection(c, settingspkg.CollectionHooks)
}

// PutSettingsHook upserts one hook settings item.
func (h *BaseHandlers) PutSettingsHook(c *gin.Context) {
	req, err := parsePutSettingsHookRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.putSettingsCollectionItem(c, req)
}

// DeleteSettingsHook deletes one hook settings item.
func (h *BaseHandlers) DeleteSettingsHook(c *gin.Context) {
	req, err := parseDeleteSettingsCollectionRequest(c, settingspkg.CollectionHooks)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.deleteSettingsCollectionItem(c, req)
}

// TriggerSettingsRestart starts the asynchronous daemon restart flow.
func (h *BaseHandlers) TriggerSettingsRestart(c *gin.Context) {
	if h.SettingsRestart == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsRestartUnavailable)
		return
	}

	operation, err := h.SettingsRestart.RequestRestart(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusAccepted, SettingsRestartActionResponseFromOperation(operation))
}

// GetSettingsRestartStatus returns the persisted restart operation payload.
func (h *BaseHandlers) GetSettingsRestartStatus(c *gin.Context) {
	if h.SettingsRestart == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsRestartUnavailable)
		return
	}

	operationID, err := requiredSettingsPathValue(c.Param("operation_id"), "operation_id")
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	operation, err := h.SettingsRestart.GetRestartOperation(c.Request.Context(), operationID)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusOK, SettingsRestartActionStatusFromOperation(operation))
}

// StreamSettingsObservabilityLogTail streams daemon log lines over SSE.
func (h *BaseHandlers) StreamSettingsObservabilityLogTail(c *gin.Context) {
	logPath := strings.TrimSpace(h.HomePaths.LogFile)
	if logPath == "" {
		h.respondError(c, http.StatusInternalServerError, errors.New("settings log tail file is not configured"))
		return
	}

	file, info, err := openSettingsLogTailFile(logPath)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	reader := bufio.NewReader(file)
	var partial string

	ticker := time.NewTicker(settingsLogTailPollInterval(h.PollInterval))
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
			rotated, rotationErr := settingsLogTailRotated(logPath, info, file)
			if rotationErr != nil {
				h.writeSSEBestEffort(writer, SSEMessage{
					Name: settingsErrorKey,
					Data: ErrorPayloadForError(rotationErr),
				})
				return
			}
			if rotated {
				return
			}
			if drainErr := h.drainSettingsLogTail(writer, reader, &partial); drainErr != nil {
				h.writeSSEBestEffort(writer, SSEMessage{
					Name: settingsErrorKey,
					Data: ErrorPayloadForError(drainErr),
				})
				return
			}
		}
	}
}
