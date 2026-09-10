package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
)

func (h *BaseHandlers) availableHeartbeatStatusForHealth(
	ctx context.Context,
	health contract.SessionHealthPayload,
	info *session.Info,
	includeHealth bool,
) (*contract.HeartbeatStatusResponse, error) {
	status, err := h.heartbeatStatusForHealth(ctx, health, info, includeHealth)
	if errors.Is(err, heartbeat.ErrAuthoringAgentNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (h *BaseHandlers) heartbeatStatusForHealth(
	ctx context.Context,
	health contract.SessionHealthPayload,
	info *session.Info,
	includeHealth bool,
) (contract.HeartbeatStatusResponse, error) {
	target, err := h.resolveAuthoredAgentTargetForSession(ctx, info)
	if err != nil {
		return contract.HeartbeatStatusResponse{}, fmt.Errorf("resolve authored agent target: %w", err)
	}
	result, err := h.HeartbeatStatus.Status(ctx, heartbeat.StatusRequest{
		Target:               target.heartbeatAuthoringTarget(),
		SessionID:            health.SessionID,
		IncludeSessionHealth: includeHealth,
	})
	if err != nil {
		return contract.HeartbeatStatusResponse{}, fmt.Errorf("read heartbeat status: %w", err)
	}
	return contract.HeartbeatStatusResponseFromResult(&result)
}
