package spec

import "github.com/compozy/compozy/internal/api/contract"

func attentionNotificationOperations() []OperationSpec {
	parameters := withProfileScope(
		queryParam("receipt_profile", "Profile owning acknowledgement receipts, including aggregate reads", false),
		queryParam("workspace", "Home workspace scope; empty selects global tasks", false),
	)
	return []OperationSpec{
		{
			Method: httpMethodGet, Path: "/api/notifications/attention", OperationID: "listAttentionNotifications",
			Summary: "List unread operator notifications across all workspaces and source profiles",
			Tags:    []string{specNotificationsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: parameters,
			Responses: []ResponseSpec{
				{Status: 200, Description: "Exact counts and first 100 rows; snapshot includes every occurrence",
					Body: contract.AttentionNotificationsResponse{}},
				{Status: 400, Description: "Invalid scope", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Operator surface", Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Notification source unavailable", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/notifications/attention/acknowledge",
			OperationID: "acknowledgeAttentionNotifications",
			Summary:     "Acknowledge an exact notification snapshot",
			Tags:        []string{specNotificationsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: append(
				parameters,
				enumQueryParam("surface", "Snapshot owner (default bell)", []string{"bell", "home"}),
			),
			RequestBody: contract.AcknowledgeAttentionRequest{},
			Responses: []ResponseSpec{
				{Status: 204, Description: "Receipts committed atomically; source state unchanged"},
				{Status: 400, Description: "Invalid snapshot request", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Operator surface", Body: contract.ErrorPayload{}},
				{
					Status:      409,
					Description: "Snapshot expired or does not belong to this scope; refresh",
					Body:        contract.ErrorPayload{},
				},
				{Status: 503, Description: "Notification store unavailable", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
