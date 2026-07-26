package bridgesdk

import (
	"context"
	"errors"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

// CachedInstances returns the current provider-owned instance snapshot without a Host API call.
func (s *Session) CachedInstances() []subprocess.InitializeBridgeManagedInstance {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.List()
}

// GetBridgeInstance reads one provider-owned bridge instance through the Host API.
func (s *Session) GetBridgeInstance(
	ctx context.Context,
	bridgeInstanceID string,
) (*bridgepkg.BridgeInstance, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("bridgesdk: runtime session host api is required")
	}
	return s.host.GetBridgeInstance(ctx, bridgeInstanceID)
}

// ReportBridgeInstanceState reports one provider-owned bridge state through the Host API.
func (s *Session) ReportBridgeInstanceState(
	ctx context.Context,
	params bridgepkg.BridgesInstancesReportStateParams,
) (*bridgepkg.BridgeInstance, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("bridgesdk: runtime session host api is required")
	}
	return s.host.ReportBridgeInstanceState(ctx, params)
}

// IngestBridgeMessage submits one normalized provider envelope through the Host API.
func (s *Session) IngestBridgeMessage(
	ctx context.Context,
	envelope bridgepkg.InboundMessageEnvelope,
) (*bridgepkg.BridgesMessagesIngestResult, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("bridgesdk: runtime session host api is required")
	}
	return s.host.IngestBridgeMessage(ctx, envelope)
}
