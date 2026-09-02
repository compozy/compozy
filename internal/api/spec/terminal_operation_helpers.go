package spec

import "github.com/compozy/compozy/internal/api/contract"

func terminalOperation(
	method, path, operationID, summary string,
	transports []Transport,
	parameters []ParameterSpec,
	request any,
	responses []ResponseSpec,
) OperationSpec {
	return OperationSpec{
		Method: method, Path: path, OperationID: operationID, Summary: summary,
		Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: parameters,
		RequestBody: request, Responses: terminalResponseMatrix(responses),
	}
}

func terminalResponseMatrix(responses []ResponseSpec) []ResponseSpec {
	common := []ResponseSpec{
		terminalErrorResponse(400, "Malformed terminal request"),
		terminalErrorResponse(401, "Terminal authentication required"),
		terminalErrorResponse(403, "Terminal operation forbidden"),
		terminalErrorResponse(409, "Terminal state or profile conflict"),
		terminalErrorResponse(500, "Terminal transport failed"),
		terminalErrorResponse(503, "Terminal service unavailable"),
	}
	seen := make(map[int]struct{}, len(responses))
	for _, response := range responses {
		seen[response.Status] = struct{}{}
	}
	for _, response := range common {
		if _, exists := seen[response.Status]; !exists {
			responses = append(responses, response)
		}
	}
	return responses
}

func terminalErrorResponse(status int, description string) ResponseSpec {
	return ResponseSpec{Status: status, Description: description, Body: contract.TerminalErrorResponse{}}
}
