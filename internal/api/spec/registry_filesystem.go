package spec

import "github.com/compozy/agh/internal/api/contract"

func registryFilesystemOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/fs/browse",
		OperationID: "browseDirectory",
		Summary:     "List directory entries for the workspace folder picker",
		Tags:        []string{specFilesystemKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("path", "Absolute directory path to list (defaults to the operator home)", false),
			boolQueryParam("show_hidden", "Include dotfiles in the listing"),
			boolQueryParam("dirs_only", "Return only directory entries"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.FSBrowseResponse{}},
			{Status: 400, Description: "Invalid directory path", Body: contract.ErrorPayload{}},
			{Status: 403, Description: "Directory is not accessible", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Directory not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}}
}
