package spec

import "github.com/compozy/compozy/internal/api/contract"

func getPermissionRollupOperationSpec() OperationSpec {
	return OperationSpec{Method: httpMethodGet, Path: "/api/permissions/rollup", AppAccess: AppAccessDenied, OperationID: "getPermissionRollup", Summary: "Read daily permission totals by agent and decision", Tags: []string{specObserveKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileScope(queryParam("days", "UTC calendar days including today, 1 through 90 (default 30)", false), queryParam("agent", "Filter by agent name", false), boolQueryParam("unattended", "Include only automation, daemon and extension initiators")),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.PermissionRollupResponse{}},
			{Status: 400, Description: "Invalid query", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specAggregateDeniedText, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specObserveNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}
