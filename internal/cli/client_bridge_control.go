package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// BridgeVerifyRecord is the structured result of one provider verification pass.
type BridgeVerifyRecord = contract.BridgeVerifyResponse

// BridgeSendTestRequest asks the daemon to make one real provider delivery.
type BridgeSendTestRequest = contract.BridgeSendTestRequest

// BridgeSendTestRecord is the provider acknowledgment for a real test delivery.
type BridgeSendTestRecord = contract.BridgeSendTestResponse

func (c *unixSocketClient) VerifyBridge(ctx context.Context, id string) (BridgeVerifyRecord, error) {
	trimmedID, err := requiredBridgeControlID(id)
	if err != nil {
		return BridgeVerifyRecord{}, err
	}

	var response contract.BridgeVerifyResponse
	path := bridgeControlPath(trimmedID, "verify")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return BridgeVerifyRecord{}, err
	}
	if strings.TrimSpace(response.BridgeInstanceID) != trimmedID {
		return BridgeVerifyRecord{}, bridgeControlResponseIDError("verify", response.BridgeInstanceID, trimmedID)
	}
	if err := (bridgepkg.BridgeCheckResponse{Checks: response.Checks}).Validate(); err != nil {
		return BridgeVerifyRecord{}, fmt.Errorf("cli: invalid bridge verify response: %w", err)
	}
	return response, nil
}

func (c *unixSocketClient) SendBridgeTest(
	ctx context.Context,
	id string,
	request BridgeSendTestRequest,
) (BridgeSendTestRecord, error) {
	trimmedID, err := requiredBridgeControlID(id)
	if err != nil {
		return BridgeSendTestRecord{}, err
	}

	var response contract.BridgeSendTestResponse
	path := bridgeControlPath(trimmedID, "send-test")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return BridgeSendTestRecord{}, err
	}
	if strings.TrimSpace(response.BridgeInstanceID) != trimmedID {
		return BridgeSendTestRecord{}, bridgeControlResponseIDError("send-test", response.BridgeInstanceID, trimmedID)
	}
	if err := response.Status.Validate(); err != nil {
		return BridgeSendTestRecord{}, fmt.Errorf("cli: invalid bridge send-test status: %w", err)
	}
	if strings.TrimSpace(response.DeliveryID) == "" {
		return BridgeSendTestRecord{}, errors.New("cli: invalid bridge send-test response")
	}
	if err := response.DeliveryTarget.Validate(); err != nil {
		return BridgeSendTestRecord{}, fmt.Errorf("cli: invalid bridge send-test target: %w", err)
	}
	return response, nil
}

func requiredBridgeControlID(id string) (string, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return "", errors.New("cli: bridge instance id is required")
	}
	return trimmedID, nil
}

func bridgeControlPath(id string, action string) string {
	return "/api/bridges/" + url.PathEscape(id) + "/" + action
}

func bridgeControlResponseIDError(action string, got string, want string) error {
	return fmt.Errorf(
		"cli: bridge %s returned bridge %q, want %q",
		action,
		strings.TrimSpace(got),
		want,
	)
}
