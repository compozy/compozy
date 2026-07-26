package main

import (
	"context"
	"os"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *gchatProvider) handleBridgesDeliver(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker := bridgesdk.DeliveryMarker{PID: os.Getpid(), Request: request}
	cfg, err := p.waitForInstanceConfig(
		strings.TrimSpace(request.Event.BridgeInstanceID),
		500*time.Millisecond,
	)
	if err != nil {
		return p.failUnconfiguredGChatDelivery(marker, err)
	}
	if cfg.configError != nil {
		return p.failUnconfiguredGChatDelivery(marker, cfg.configError)
	}

	if p.markers.ShouldCrashOnce() {
		p.markers.RecordDelivery(marker)
		p.markers.RecordCrash(map[string]any{
			"crashed":            true,
			"pid":                os.Getpid(),
			"delivery_id":        strings.TrimSpace(request.Event.DeliveryID),
			"bridge_instance_id": cfg.instanceID,
		})
		os.Exit(23)
	}

	state := p.deliveryState(cfg.instanceID, request.Event.DeliveryID)
	if err := p.flushGChatDeliveryProgress(ctx, session, cfg.instanceID, state); err != nil {
		return p.failGChatDelivery(ctx, session, &cfg, marker, err)
	}
	ack, state, err := executeGChatDelivery(ctx, p.apiFactory(&cfg), request, state)
	if err != nil {
		p.handleGChatDeliveryExecutionError(cfg.instanceID, request.Event.DeliveryID, state, err)
		return p.failGChatDelivery(ctx, session, &cfg, marker, err)
	}
	return p.completeGChatDelivery(ctx, session, cfg.instanceID, request, marker, ack, state)
}

func (p *gchatProvider) failUnconfiguredGChatDelivery(
	marker bridgesdk.DeliveryMarker,
	err error,
) (bridgepkg.DeliveryAck, error) {
	marker.Error = err.Error()
	p.markers.RecordDelivery(marker)
	p.setLastError(err)
	return bridgepkg.DeliveryAck{}, err
}

func (p *gchatProvider) flushGChatDeliveryProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	state deliveryState,
) error {
	if state.Progress == nil {
		return nil
	}
	if err := state.Progress.Flush(ctx); err != nil {
		p.reportGChatProgressFailure(ctx, session, instanceID, err)
		if !bridgesdk.ShouldContinueTextDeliveryAfterProgress(err) {
			return err
		}
	}
	return nil
}

func (p *gchatProvider) handleGChatDeliveryExecutionError(
	instanceID string,
	deliveryID string,
	state deliveryState,
	err error,
) {
	if bridgesdk.IsCommittedMutation(err) {
		p.deliveries.Delete(deliveryStateKey(instanceID, deliveryID))
		if state.Progress != nil {
			state.Progress.Close()
		}
		return
	}
	p.storeDeliveryRetryState(instanceID, deliveryID, state)
}

func (p *gchatProvider) completeGChatDelivery(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	request bridgepkg.DeliveryRequest,
	marker bridgesdk.DeliveryMarker,
	ack bridgepkg.DeliveryAck,
	state deliveryState,
) (bridgepkg.DeliveryAck, error) {
	dispatcher := state.Progress
	var cleanupErr error
	if dispatcher != nil {
		cleanupErr = dispatcher.OnContent(ctx)
		if isTerminalGChatDeliveryEvent(request.Event) {
			state.Progress = nil
		}
	}
	p.storeDeliveryState(instanceID, request.Event.DeliveryID, request.Event, state)
	if dispatcher != nil && state.Progress == nil {
		dispatcher.Close()
	}

	readyErr := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, instanceID)
	switch {
	case readyErr != nil:
		p.setLastError(readyErr)
	case cleanupErr != nil:
		p.recordGChatProgressCleanupError(cleanupErr)
	default:
		p.clearLastError()
	}
	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	return ack, nil
}

func (p *gchatProvider) failGChatDelivery(
	ctx context.Context,
	session *bridgesdk.Session,
	cfg *resolvedInstanceConfig,
	marker bridgesdk.DeliveryMarker,
	err error,
) (bridgepkg.DeliveryAck, error) {
	marker.Error = err.Error()
	p.markers.RecordDelivery(marker)
	classified := bridgesdk.ClassifyError(err)
	_, _, reportErr := session.ReportClassifiedError(ctx, cfg.instanceID, classified)
	if reportErr != nil {
		p.setLastError(reportErr)
	} else {
		p.setLastError(err)
	}
	return bridgepkg.DeliveryAck{}, err
}
