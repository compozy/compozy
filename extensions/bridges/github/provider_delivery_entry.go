package main

import (
	"context"

	"os"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *githubProvider) handleBridgesDeliver(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker := bridgesdk.DeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	cfg, err := p.waitForInstanceConfig(ctx, strings.TrimSpace(request.Event.BridgeInstanceID))
	if err != nil {
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
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

	installationID, err := p.resolveDeliveryInstallationID(&cfg, request)
	if err != nil {
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}

	api := p.apiFactory(cfg)
	ack, state, err := executeGitHubDelivery(
		ctx,
		api,
		&cfg,
		request,
		p.deliveryState(cfg.instanceID, request.Event.DeliveryID),
		installationID,
	)
	if err != nil {
		if bridgesdk.IsCommittedMutation(err) {
			p.deliveries.Delete(deliveryStateKey(cfg.instanceID, request.Event.DeliveryID))
		}
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

	p.storeDeliveryState(cfg.instanceID, request.Event.DeliveryID, request.Event, state)
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	}

	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	p.clearLastError()
	return ack, nil
}
