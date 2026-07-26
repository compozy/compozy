package spec

import "github.com/compozy/agh/internal/api/contract"

func registryLogOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/logs",
		OperationID: "listLogs",
		Summary:     "List runtime logs",
		Tags:        []string{specLogsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  logFilterQueryParams(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.LogsListResponse{}},
			{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/logs/stream",
			OperationID: "streamLogs",
			Summary:     "Stream runtime logs",
			Tags:        []string{specLogsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  logStreamQueryParams(),
			Responses: []ResponseSpec{
				{
					Status:      200,
					Description: "Log event stream",
					Body:        contract.LogEventPayload{},
					ContentType: specContentTypeEventStream,
				},
				{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
