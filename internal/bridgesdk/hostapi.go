package bridgesdk

import (
	"context"
	"errors"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

// CallFunc issues one typed Host API request.
type CallFunc func(context.Context, string, any, any) error

// HostAPIClient is the typed provider-side client for bridge Host API calls.
type HostAPIClient struct {
	call CallFunc
}

// NewHostAPIClient constructs a Host API client over the shared runtime peer.
func NewHostAPIClient(peer *Peer) *HostAPIClient {
	if peer == nil {
		return nil
	}
	return &HostAPIClient{
		call: peer.Call,
	}
}

// NewHostAPIClientFromCall constructs a Host API client from an arbitrary call
// function, mainly for tests.
func NewHostAPIClientFromCall(call CallFunc) *HostAPIClient {
	if call == nil {
		return nil
	}
	return &HostAPIClient{call: call}
}

// Call issues one raw Host API request.
func (c *HostAPIClient) Call(ctx context.Context, method string, params any, result any) error {
	if c == nil || c.call == nil {
		return errors.New("bridgesdk: host api client is required")
	}
	if ctx == nil {
		return errors.New("bridgesdk: host api context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.call(ctx, method, params, result)
}

// ListBridgeInstances returns every bridge instance currently assigned to the provider runtime.
func (c *HostAPIClient) ListBridgeInstances(ctx context.Context) ([]bridgepkg.BridgeInstance, error) {
	var result []bridgepkg.BridgeInstance
	if err := c.Call(
		ctx,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		struct{}{},
		&result,
	); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBridgeInstance returns one provider-owned bridge instance.
func (c *HostAPIClient) GetBridgeInstance(
	ctx context.Context,
	bridgeInstanceID string,
) (*bridgepkg.BridgeInstance, error) {
	var result bridgepkg.BridgeInstance
	if err := c.Call(
		ctx,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		bridgepkg.BridgeInstanceTargetParams{BridgeInstanceID: bridgeInstanceID},
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReportBridgeInstanceState reports one provider-observed bridge status change.
func (c *HostAPIClient) ReportBridgeInstanceState(
	ctx context.Context,
	params bridgepkg.BridgesInstancesReportStateParams,
) (*bridgepkg.BridgeInstance, error) {
	var result bridgepkg.BridgeInstance
	if err := c.Call(
		ctx,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		params,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

// IngestBridgeMessage ingests one normalized inbound bridge event.
func (c *HostAPIClient) IngestBridgeMessage(
	ctx context.Context,
	envelope bridgepkg.InboundMessageEnvelope,
) (*bridgepkg.BridgesMessagesIngestResult, error) {
	var result bridgepkg.BridgesMessagesIngestResult
	if err := c.Call(
		ctx,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		envelope,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}
