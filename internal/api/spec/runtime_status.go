package spec

import "github.com/compozy/agh/internal/api/contract"

func runtimeStatusOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/doctor",
			OperationID: "getDoctor",
			Summary:     "Run runtime diagnostics",
			Tags:        []string{specDiagnosticsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("only", "Comma-separated probe ids or categories to include", false),
				queryParam("exclude", "Comma-separated probe ids or categories to exclude", false),
				boolQueryParam("quiet", "Omit OK diagnostics"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.DoctorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/status",
			OperationID: "getStatus",
			Summary:     "Get the runtime status snapshot",
			Tags:        []string{specDiagnosticsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.StatusPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		drainStateOperation(httpMethodPost, "/api/drain", "drainDaemon", "Stop admitting new work"),
		drainStateOperation(httpMethodPost, "/api/undrain", "undrainDaemon", "Resume admission of new work"),
	}
}

func drainStateOperation(method string, path string, operationID string, summary string) OperationSpec {
	return OperationSpec{
		Method:      method,
		Path:        path,
		OperationID: operationID,
		Summary:     summary,
		Tags:        []string{specDiagnosticsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.DrainStatusResponse{}},
			{Status: 503, Description: "Daemon drain controller is unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
