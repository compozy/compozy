package spec

import "github.com/compozy/agh/internal/api/contract"

const (
	specAPIToolApprovalGrantsPath    = "/api/tool-approval-grants"
	specAPIToolApprovalGrantsIDPath  = specAPIToolApprovalGrantsPath + "/{id}"
	specToolApprovalGrantUnavailable = "Tool approval grant service unavailable"
)

func toolApprovalGrantOperationSpecs() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodPut,
			Path:        specAPIToolApprovalGrantsPath,
			OperationID: "setToolApprovalGrant",
			Summary:     "Set an explicit agent-wide or tool-wide native-tool approval decision",
			Tags:        []string{specToolsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("workspace_id", "Workspace id or reference", true),
			},
			RequestBody: contract.ToolApprovalGrantSetRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Stored", Body: contract.ToolApprovalGrantResponse{}},
				{Status: 400, Description: "Invalid wider approval decision", Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Workspace not found", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specToolApprovalGrantUnavailable, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        specAPIToolApprovalGrantsPath,
			OperationID: "listToolApprovalGrants",
			Summary:     "List remembered native-tool approval decisions for one workspace",
			Tags:        []string{specToolsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("workspace_id", "Workspace id or reference", true),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ToolApprovalGrantListResponse{}},
				{Status: 400, Description: "Workspace is required", Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Workspace not found", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specToolApprovalGrantUnavailable, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodDelete,
			Path:        specAPIToolApprovalGrantsIDPath,
			OperationID: "revokeToolApprovalGrant",
			Summary:     "Revoke one remembered native-tool approval decision in one workspace",
			Tags:        []string{specToolsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Remembered approval grant id"),
				queryParam("workspace_id", "Workspace id or reference", true),
			},
			Responses: []ResponseSpec{
				{Status: 204, Description: "Revoked"},
				{Status: 400, Description: "Workspace and grant id are required", Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Workspace or approval grant not found", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specToolApprovalGrantUnavailable, Body: contract.ErrorPayload{}},
			},
		},
	}
}
