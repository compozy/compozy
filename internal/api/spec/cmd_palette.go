package spec

import "github.com/compozy/compozy/internal/api/contract"

func cmdPaletteOperations() []OperationSpec {
	transports := []Transport{TransportHTTP, TransportUDS}
	return []OperationSpec{
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/commands",
			OperationID: "listCmdPaletteCommands", Summary: "List command palette commands",
			Tags: []string{specCmdPaletteKey}, Transports: transports,
			Parameters: []ParameterSpec{
				queryParam("workspace", "Workspace id, name, or path", true),
				queryParam("client", "Attached client whose context resolves availability", false),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.CmdPaletteCommandsResponse{}},
				{Status: 400, Description: "Invalid workspace", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Command palette unavailable", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/clients",
			OperationID: "listCmdPaletteClients", Summary: "List attached command palette clients",
			Tags: []string{specCmdPaletteKey}, Transports: transports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: []contract.CmdPaletteClient{}},
				{Status: 400, Description: "Invalid workspace", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Command palette unavailable", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPost, Path: "/api/cmd-palette/commands/{id}/invoke",
			OperationID: "invokeCmdPaletteCommand", Summary: "Invoke one command palette command",
			Tags: []string{specCmdPaletteKey}, Transports: transports,
			Parameters:  []ParameterSpec{pathParam("id", "Canonical command id")},
			RequestBody: contract.CmdPaletteInvokeRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Completed", Body: contract.CmdPaletteInvokeResult{}},
				{Status: 202, Description: "Approval pending", Body: contract.CmdPaletteInvokeResult{}},
				{Status: 401, Description: "Invalid client attachment", Body: contract.CmdPaletteError{}},
				{Status: 404, Description: "Command not found", Body: contract.CmdPaletteError{}},
				{Status: 409, Description: "Invocation conflict", Body: contract.CmdPaletteError{}},
				{Status: 412, Description: "Command unavailable", Body: contract.CmdPaletteError{}},
				{Status: 422, Description: "Invalid command arguments", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Command palette unavailable", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/stream",
			OperationID: "streamCmdPalette", Summary: "Stream command palette catalog invalidations",
			Tags: []string{specCmdPaletteKey}, Transports: transports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{
					Status: 200, Description: "Catalog invalidation stream",
					Body: contract.CmdPaletteCatalogChangedEvent{}, ContentType: specContentTypeEventStream,
				},
				{Status: 400, Description: "Invalid workspace", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Command palette unavailable", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/tools/approvals/{id}",
			OperationID: "getPendingToolApproval", Summary: "Get one pending tool approval lifecycle",
			Tags: []string{specToolsKey}, Transports: transports,
			Parameters: []ParameterSpec{pathParam("id", "Stable approval id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ToolApprovalStatusResponse{}},
				{Status: 404, Description: "Approval not found", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Tool approval unavailable", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPost, Path: "/api/tools/approvals/{id}/cancel",
			OperationID: "cancelPendingToolApproval", Summary: "Cancel one pending tool approval",
			Tags: []string{specToolsKey}, Transports: transports,
			Parameters: []ParameterSpec{pathParam("id", "Stable approval id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Canceled", Body: contract.ToolApprovalStatusResponse{}},
				{Status: 404, Description: "Approval not found", Body: contract.CmdPaletteError{}},
				{Status: 409, Description: "Approval already terminal", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Tool approval unavailable", Body: contract.CmdPaletteError{}},
			},
		},
	}
}
