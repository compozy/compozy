package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

type bridgeClientAPI interface {
	ListBridges(ctx context.Context, query BridgeListQuery) (BridgeListRecord, error)
	CreateBridge(ctx context.Context, request CreateBridgeRequest) (BridgeRecord, error)
	GetBridge(ctx context.Context, id string) (BridgeRecord, error)
	UpdateBridge(ctx context.Context, id string, request UpdateBridgeRequest) (BridgeRecord, error)
	EnableBridge(ctx context.Context, id string) (BridgeRecord, error)
	DisableBridge(ctx context.Context, id string) (BridgeRecord, error)
	RestartBridge(ctx context.Context, id string) (BridgeRecord, error)
	BridgeRoutes(ctx context.Context, id string) ([]BridgeRouteRecord, error)
	BridgeTargets(ctx context.Context, id string, query string, limit int) (BridgeTargetsRecord, error)
	ResolveBridgeTarget(ctx context.Context, id string, name string) (BridgeResolveTargetRecord, error)
	ListBridgeSecretBindings(ctx context.Context, id string) ([]BridgeSecretBindingRecord, error)
	PutBridgeSecretBinding(
		ctx context.Context,
		id string,
		bindingName string,
		request BridgeSecretBindingRequest,
	) (BridgeSecretBindingRecord, error)
	DeleteBridgeSecretBinding(ctx context.Context, id string, bindingName string) error
	TestBridgeDelivery(
		ctx context.Context,
		id string,
		request BridgeTestDeliveryRequest,
	) (BridgeTestDeliveryRecord, error)
	SlackBridgeManifest(ctx context.Context, instanceID string) (SlackManifestRecord, error)
	VerifyBridge(ctx context.Context, id string) (BridgeVerifyRecord, error)
	SendBridgeTest(
		ctx context.Context,
		id string,
		request BridgeSendTestRequest,
	) (BridgeSendTestRecord, error)
	RegisterBridgeWebhook(ctx context.Context, id string) (BridgeWebhookRegistrationRecord, error)
}

// SlackManifestRecord is the generated Slack application artifact.
type SlackManifestRecord = bridgepkg.SlackAppManifest

func (c *unixSocketClient) SlackBridgeManifest(
	ctx context.Context,
	instanceID string,
) (SlackManifestRecord, error) {
	trimmedID := strings.TrimSpace(instanceID)
	if trimmedID == "" {
		return SlackManifestRecord{}, errors.New("cli: bridge instance id is required")
	}
	query := url.Values{}
	query.Set("instance", trimmedID)
	var response contract.SlackAppManifestResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/bridges/providers/slack/manifest",
		query,
		nil,
		&response,
	); err != nil {
		return SlackManifestRecord{}, err
	}
	return response.Manifest, nil
}
