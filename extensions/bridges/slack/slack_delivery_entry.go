package main

import (
	"context"

	"os"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *slackProvider) handleBridgesDeliver(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker := bridgesdk.DeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	cfg, err := p.waitForInstanceConfig(strings.TrimSpace(request.Event.BridgeInstanceID), 500*time.Millisecond)
	if err != nil {
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}

	if p.markers.ShouldCrashOnce() {
		p.markers.RecordDelivery(marker)
		p.markers.RecordCrash(map[string]any{
			"crashed":                   true,
			"pid":                       os.Getpid(),
			providerDeliveryIDKey:       strings.TrimSpace(request.Event.DeliveryID),
			providerBridgeInstanceIDKey: cfg.instanceID,
		})
		os.Exit(23)
	}

	ack, state, err := p.executeTextDeliveryWithProgress(ctx, &cfg, request)
	if err != nil {
		p.recordDeliveryFailure(ctx, session, cfg.instanceID, request, state, marker, err)
		return bridgepkg.DeliveryAck{}, err
	}

	progressCleanupErr := p.completeTextDeliveryProgress(ctx, cfg.instanceID, request, state)
	if progressCleanupErr == nil {
		p.clearLastError()
	}
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	}

	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	if progressCleanupErr != nil {
		p.recordProgressCleanupError("clear progress after text delivery", progressCleanupErr)
	}
	return ack, nil
}
