package main

import (
	"context"
	"os"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *teamsProvider) handleBridgesDeliver(
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
		return p.failTeamsDelivery(ctx, session, resolvedInstanceConfig{}, marker, err)
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
	if err := p.flushTeamsDeliveryProgress(ctx, session, cfg.instanceID, state); err != nil {
		return p.failTeamsDelivery(ctx, session, cfg, marker, err)
	}
	ack, state, err := p.executeTeamsDelivery(ctx, cfg, request, state)
	if err != nil {
		p.handleTeamsDeliveryExecutionError(cfg.instanceID, request.Event.DeliveryID, state, err)
		return p.failTeamsDelivery(ctx, session, cfg, marker, err)
	}
	return p.completeTeamsDelivery(ctx, session, cfg.instanceID, request, marker, ack, state)
}

func (p *teamsProvider) executeTeamsDelivery(
	ctx context.Context,
	cfg resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	return executeTeamsDelivery(
		ctx,
		p.apiFactory(cfg),
		cfg,
		request,
		state,
		func(deliveryID string) (deliveryState, bool) {
			referencedState := p.deliveryState(cfg.instanceID, deliveryID)
			return referencedState, strings.TrimSpace(referencedState.RemoteMessageID) != ""
		},
		p.userContext,
	)
}

func (p *teamsProvider) flushTeamsDeliveryProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	state deliveryState,
) error {
	if state.Progress == nil {
		return nil
	}
	if err := state.Progress.Flush(ctx); err != nil {
		p.reportTeamsProgressFailure(ctx, session, instanceID, err)
		if !bridgesdk.ShouldContinueTextDeliveryAfterProgress(err) {
			return err
		}
	}
	return nil
}

func (p *teamsProvider) handleTeamsDeliveryExecutionError(
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

func (p *teamsProvider) completeTeamsDelivery(
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
		if isTerminalTeamsDeliveryEvent(request.Event) {
			state.Progress = nil
		}
	}
	p.storeDeliveryState(instanceID, request.Event.DeliveryID, state)
	if dispatcher != nil && state.Progress == nil {
		dispatcher.Close()
	}

	readyErr := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, instanceID)
	switch {
	case readyErr != nil:
		p.setLastError(readyErr)
	case cleanupErr != nil:
		p.recordTeamsProgressCleanupError(cleanupErr)
	default:
		p.clearLastError()
	}
	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	return ack, nil
}

func (p *teamsProvider) failTeamsDelivery(
	ctx context.Context,
	session *bridgesdk.Session,
	cfg resolvedInstanceConfig,
	marker bridgesdk.DeliveryMarker,
	err error,
) (bridgepkg.DeliveryAck, error) {
	marker.Error = err.Error()
	p.markers.RecordDelivery(marker)
	if strings.TrimSpace(cfg.instanceID) == "" {
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}
	classified := bridgesdk.ClassifyError(err)
	_, _, reportErr := session.ReportClassifiedError(ctx, cfg.instanceID, classified)
	if reportErr != nil {
		p.setLastError(reportErr)
	} else {
		p.setLastError(err)
	}
	return bridgepkg.DeliveryAck{}, err
}
