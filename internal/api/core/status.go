package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/doctor"

	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

const (
	statusApplyStateCurrent = "current"
	statusStateAvailable    = "available"
	statusStateConfigured   = "configured"
	statusStateDegraded     = "degraded"
	statusStateOK           = "ok"
	statusStateRunning      = "running"
	statusStateWarn         = "warn"
	statusStateError        = "error"
)

// GetDoctor returns the hard-cut diagnostic probe payload shared by HTTP and UDS.
func (h *BaseHandlers) GetDoctor(c *gin.Context) {
	opts := doctor.RunOptions{
		Only:    splitStatusFilter(c.QueryArray("only"), c.Query("only")),
		Exclude: splitStatusFilter(c.QueryArray("exclude"), c.Query("exclude")),
		Quiet:   strings.EqualFold(strings.TrimSpace(c.Query("quiet")), "true"),
		Env: doctor.ProbeEnv{
			Now: h.nowUTC,
		},
	}
	payload, err := h.doctorPayload(c.Request.Context(), opts)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) statusPayload(
	ctx context.Context,
	workspaceID string,
) (contract.StatusPayload, error) {
	if ctx == nil {
		return contract.StatusPayload{}, errors.New("api: status context is required")
	}
	if h.Observer == nil {
		return contract.StatusPayload{}, errors.New("api: observer is required for status")
	}
	if h.Sessions == nil {
		return contract.StatusPayload{}, errors.New("api: session manager is required for status")
	}

	health, err := h.Observer.Health(ctx)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect observer health: %w", err)
	}
	sessionSummary, err := h.sessionAggregate(ctx)
	if err != nil {
		return contract.StatusPayload{}, err
	}
	memoryHealth, err := h.memoryHealthSnapshot(ctx, workspaceID)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect memory health: %w", err)
	}
	automationHealth, err := h.automationHealth(ctx)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect automation health: %w", err)
	}
	networkStatus, err := h.runtimeNetworkStatusPayload(ctx)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect network status: %w", err)
	}
	schemaStreams, err := h.schemaStreamStatusPayloads(ctx)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect schema stream status: %w", err)
	}
	providers, err := h.providerStatusPayloads(ctx)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect provider status: %w", err)
	}
	mcpServers, err := h.mcpServerStatusPayloads(ctx, workspaceID)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect MCP server status: %w", err)
	}
	skillStatus, err := h.skillRuntimeStatusPayload(ctx, workspaceID)
	if err != nil {
		return contract.StatusPayload{}, fmt.Errorf("api: collect skill runtime status: %w", err)
	}

	return contract.StatusPayload{
		SchemaVersion:    contract.StatusSchemaVersion,
		GeneratedAt:      h.nowUTC(),
		Daemon:           h.daemonStatusPayload(&health, sessionSummary.Total, networkStatus, schemaStreams),
		Sessions:         sessionSummary,
		SubprocessHealth: h.subprocessHealthAggregate(workspaceID),
		Health:           ObserveHealthPayloadFromHealth(&health),
		Memory:           memoryHealth,
		Automation:       automationHealth,
		Tasks:            TaskHealthPayloadFromObserve(health.Tasks),
		Bridges:          BridgeAggregateHealthPayloadFromObserve(health.Bridges),
		Providers:        providers,
		MCPServers:       mcpServers,
		Skills:           skillStatus,
		Config:           h.configRuntimeStatusPayload(),
		LogTail:          h.logTailStatusPayload(ctx),
	}, nil
}

func (h *BaseHandlers) runtimeNetworkStatusPayload(ctx context.Context) (*contract.NetworkStatusPayload, error) {
	if !h.Config.Network.Enabled {
		return h.networkStatusPayload(ctx)
	}
	if h.Network == nil {
		return &contract.NetworkStatusPayload{
			Enabled: true,
			Status:  memoryHealthStatusUnavailable,
		}, nil
	}
	payload, err := h.networkStatusPayload(ctx)
	if err != nil {
		return &contract.NetworkStatusPayload{
			Enabled: true,
			Status:  memoryHealthStatusUnavailable,
		}, nil
	}
	return payload, nil
}

func (h *BaseHandlers) sessionAggregate(ctx context.Context) (contract.SessionAggregatePayload, error) {
	infos, err := h.Sessions.ListAll(ctx)
	if err != nil {
		return contract.SessionAggregatePayload{}, fmt.Errorf("api: list sessions for status: %w", err)
	}
	byStatus := make(map[string]int)
	byBadge := make(map[string]int)
	active := 0
	for _, info := range infos {
		if info == nil {
			continue
		}
		state := strings.TrimSpace(string(info.State))
		if state == "" {
			state = unknownValue
		}
		byStatus[state]++
		badge := string(session.BadgeForInfo(info))
		if badge == "" {
			badge = string(session.BadgeUnknown)
		}
		byBadge[badge]++
		if info.State == session.StateActive {
			active++
		}
	}
	return contract.SessionAggregatePayload{
		Active:   active,
		Total:    len(infos),
		ByStatus: byStatus,
		ByBadge:  byBadge,
	}, nil
}

func (h *BaseHandlers) providerStatusPayloads(ctx context.Context) ([]contract.ProviderStatusPayload, error) {
	response, err := h.providerListResponse(ctx)
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.ProviderStatusPayload, 0, len(response.Providers))
	for _, provider := range response.Providers {
		status := provider.AuthStatus
		var lastProbeAt *time.Time
		if status.LastProbeAt != nil && !status.LastProbeAt.IsZero() {
			timestamp := status.LastProbeAt.UTC()
			lastProbeAt = &timestamp
		}
		payloads = append(payloads, contract.ProviderStatusPayload{
			Name:          strings.TrimSpace(provider.Name),
			DisplayName:   strings.TrimSpace(provider.DisplayName),
			Default:       provider.Default,
			Mode:          strings.TrimSpace(status.Mode),
			EnvPolicy:     strings.TrimSpace(status.EnvPolicy),
			HomePolicy:    strings.TrimSpace(status.HomePolicy),
			State:         strings.TrimSpace(status.State),
			Code:          strings.TrimSpace(status.Code),
			Message:       diagnostics.RedactAndBound(status.Message, maxDiagnosticPayloadBytes),
			StatusCommand: diagnostics.RedactAndBound(status.StatusCmd, maxDiagnosticPayloadBytes),
			LoginCommand:  diagnostics.RedactAndBound(status.LoginCmd, maxDiagnosticPayloadBytes),
			LastProbeAt:   lastProbeAt,
			SuggestedCommand: diagnostics.RedactAndBound(
				providerSuggestedCommand(provider.Name, status),
				maxDiagnosticPayloadBytes,
			),
		})
	}
	return payloads, nil
}

func (h *BaseHandlers) configRuntimeStatusPayload() contract.ConfigRuntimeStatusPayload {
	cfg := h.Config
	payload := contract.ConfigRuntimeStatusPayload{
		Status:          statusStateOK,
		Validated:       true,
		HomeDir:         strings.TrimSpace(h.HomePaths.HomeDir),
		ConfigFile:      strings.TrimSpace(h.HomePaths.ConfigFile),
		RestartRequired: false,
		ApplyState:      statusApplyStateCurrent,
	}
	if err := cfg.Validate(); err != nil {
		payload.Status = statusStateError
		payload.Validated = false
		payload.ValidationError = diagnostics.RedactAndBound(err.Error(), maxDiagnosticPayloadBytes)
	}
	return payload
}

func (h *BaseHandlers) logTailStatusPayload(ctx context.Context) contract.LogTailStatusPayload {
	if h.Settings == nil {
		return contract.LogTailStatusPayload{
			Available: false,
			Status:    memoryHealthStatusUnavailable,
		}
	}
	envelope, err := h.Settings.GetSection(ctx, settingspkg.SectionRequest{
		Section: settingspkg.SectionObservability,
		Scope:   settingspkg.ScopeGlobal,
	})
	if err != nil || envelope.Observability == nil {
		return contract.LogTailStatusPayload{
			Available: false,
			Status:    memoryHealthStatusUnavailable,
		}
	}
	if envelope.Observability.LogTailSupport.Available {
		return contract.LogTailStatusPayload{
			Available: true,
			Status:    statusStateAvailable,
		}
	}
	return contract.LogTailStatusPayload{
		Available: false,
		Status:    memoryHealthStatusDisabled,
	}
}
