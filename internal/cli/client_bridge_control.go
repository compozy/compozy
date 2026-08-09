package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/contract"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
)

// BridgeVerifyRecord is the structured result of one provider verification pass.
type BridgeVerifyRecord = contract.BridgeVerifyResponse

// BridgeSendTestRequest asks the daemon to make one real provider delivery.
type BridgeSendTestRequest = contract.BridgeSendTestRequest

// BridgeSendTestRecord is the provider acknowledgment for a real test delivery.
type BridgeSendTestRecord = contract.BridgeSendTestResponse

func (c *daemonClient) VerifyBridge(ctx context.Context, id string) (BridgeVerifyRecord, error) {
	bridgeID, err := requiredBridgeControlID(id)
	if err != nil {
		return BridgeVerifyRecord{}, err
	}

	var response contract.BridgeVerifyResponse
	path := bridgeControlPath(bridgeID, "verify")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return BridgeVerifyRecord{}, err
	}
	if response.BridgeInstanceID != bridgeID {
		return BridgeVerifyRecord{}, bridgeControlResponseIDError("verify", response.BridgeInstanceID, bridgeID)
	}
	if err := (bridgepkg.BridgeCheckResponse{Checks: response.Checks}).Validate(); err != nil {
		return BridgeVerifyRecord{}, fmt.Errorf("cli: invalid bridge verify response: %w", err)
	}
	return response, nil
}

func (c *daemonClient) SendBridgeTest(
	ctx context.Context,
	id string,
	request BridgeSendTestRequest,
) (BridgeSendTestRecord, error) {
	bridgeID, err := requiredBridgeControlID(id)
	if err != nil {
		return BridgeSendTestRecord{}, err
	}
	if _, err := request.ToResolveDeliveryTargetRequest(bridgeID); err != nil {
		return BridgeSendTestRecord{}, fmt.Errorf("cli: invalid bridge send-test request: %w", err)
	}

	var response contract.BridgeSendTestResponse
	path := bridgeControlPath(bridgeID, "send-test")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return BridgeSendTestRecord{}, err
	}
	if response.BridgeInstanceID != bridgeID {
		return BridgeSendTestRecord{}, bridgeControlResponseIDError("send-test", response.BridgeInstanceID, bridgeID)
	}
	if err := response.Status.Validate(); err != nil {
		return BridgeSendTestRecord{}, fmt.Errorf("cli: invalid bridge send-test status: %w", err)
	}
	if response.DeliveryID == "" || !utf8.ValidString(response.DeliveryID) {
		return BridgeSendTestRecord{}, errors.New("cli: invalid bridge send-test response")
	}
	if err := response.DeliveryTarget.Validate(); err != nil {
		return BridgeSendTestRecord{}, fmt.Errorf("cli: invalid bridge send-test target: %w", err)
	}
	return response, nil
}

func requiredBridgeControlID(id string) (string, error) {
	if id == "" {
		return "", errors.New("cli: bridge instance id is required")
	}
	if !utf8.ValidString(id) {
		return "", errors.New("cli: bridge instance id must be valid UTF-8")
	}
	return id, nil
}

func bridgeControlPath(id string, action string) string {
	return "/api/bridges/" + url.PathEscape(id) + "/" + action
}

func bridgeControlResponseIDError(action string, got string, want string) error {
	return fmt.Errorf(
		"cli: bridge %s returned bridge %q, want %q",
		action,
		got,
		want,
	)
}
