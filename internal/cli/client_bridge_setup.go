package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

var _ bridgeSetupClient = (*unixSocketClient)(nil)

func (c *unixSocketClient) RegisterBridgeWebhook(
	ctx context.Context,
	id string,
) (BridgeWebhookRegistrationRecord, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return BridgeWebhookRegistrationRecord{}, errors.New("cli: bridge instance id is required")
	}
	var response contract.BridgeWebhookRegistrationResponse
	path := "/api/bridges/" + url.PathEscape(trimmedID) + "/webhook/register"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return BridgeWebhookRegistrationRecord{}, err
	}
	if strings.TrimSpace(response.BridgeInstanceID) != trimmedID {
		return BridgeWebhookRegistrationRecord{}, fmt.Errorf(
			"cli: webhook registration returned bridge %q, want %q",
			response.BridgeInstanceID,
			trimmedID,
		)
	}
	record := BridgeWebhookRegistrationRecord{
		Status:      response.Status,
		Remediation: response.Remediation,
	}
	if err := record.Validate(); err != nil {
		return BridgeWebhookRegistrationRecord{}, fmt.Errorf(
			"cli: invalid bridge webhook registration response: %w",
			err,
		)
	}
	return record, nil
}
