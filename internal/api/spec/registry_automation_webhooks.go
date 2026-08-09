package spec

import "github.com/compozy/compozy/internal/api/contract"

func deliverGlobalWebhookOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/webhooks/global/{endpoint}",
		OperationID: "deliverGlobalWebhook",
		Summary:     "Deliver one global automation webhook",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP},
		Parameters: []ParameterSpec{
			pathParam("endpoint", "Webhook endpoint slug and id"),
			headerParam("X-Compozy-Webhook-Delivery-ID", "Unique webhook delivery id"),
			headerParam("X-Compozy-Webhook-Timestamp", "Signed webhook timestamp"),
			headerParam("X-Compozy-Webhook-Signature", "Signed webhook HMAC signature"),
		},
		RequestBody: map[string]any{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WebhookDeliveryResponse{}},
			{Status: 400, Description: "Invalid webhook request", Body: contract.ErrorPayload{}},
			{
				Status:      401,
				Description: "Webhook authentication failed",
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Webhook trigger not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Webhook delivery is disabled or already claimed",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      413,
				Description: specPayloadTooLargeDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specAutomationManagerIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}

func deliverWorkspaceWebhookOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/webhooks/workspaces/{workspace_id}/{endpoint}",
		OperationID: "deliverWorkspaceWebhook",
		Summary:     "Deliver one workspace-scoped automation webhook",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("endpoint", "Webhook endpoint slug and id"),
			headerParam("X-Compozy-Webhook-Delivery-ID", "Unique webhook delivery id"),
			headerParam("X-Compozy-Webhook-Timestamp", "Signed webhook timestamp"),
			headerParam("X-Compozy-Webhook-Signature", "Signed webhook HMAC signature"),
		},
		RequestBody: map[string]any{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WebhookDeliveryResponse{}},
			{Status: 400, Description: "Invalid webhook request", Body: contract.ErrorPayload{}},
			{
				Status:      401,
				Description: "Webhook authentication failed",
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Webhook trigger not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Webhook delivery is disabled or already claimed",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      413,
				Description: specPayloadTooLargeDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specAutomationManagerIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}
