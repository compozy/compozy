package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	cmdPaletteInvalidWorkspaceDescription = "Invalid workspace"
	cmdPaletteUnavailableDescription      = "Command palette unavailable"
	cmdPaletteCommandNotFoundDescription  = "Command not found"
	cmdPaletteViewNotFoundDescription     = "View not found"
)

var (
	cmdPaletteTransports     = []Transport{TransportHTTP, TransportUDS}
	cmdPaletteOperationSpecs = []OperationSpec{
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/commands",
			OperationID: "listCmdPaletteCommands", Summary: "List command palette commands",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				queryParam("workspace", "Workspace id, name, or path", true),
				queryParam("client", "Attached client whose context resolves availability", false),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.CmdPaletteCommandsResponse{}},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/clients",
			OperationID: "listCmdPaletteClients", Summary: "List attached command palette clients",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: []contract.CmdPaletteClient{}},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/rank-signals",
			OperationID: "getCmdPaletteRankSignals", Summary: "Get command palette rank signals",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.CmdPaletteRankSignalsResponse{}},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPost, Path: "/api/cmd-palette/usage",
			OperationID: "recordCmdPaletteUsage", Summary: "Record command palette usage",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			RequestBody: contract.CmdPaletteUsageRequest{},
			Responses: []ResponseSpec{
				{Status: 204, Description: "Recorded"},
				{Status: 400, Description: "Invalid request", Body: contract.CmdPaletteError{}},
				{Status: 404, Description: cmdPaletteCommandNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPut, Path: "/api/cmd-palette/pins/{id}",
			OperationID: "pinCmdPaletteCommand", Summary: "Pin one command palette command",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("id", "Canonical command id"),
				queryParam("workspace", "Workspace id, name, or path", true),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Pinned", Body: contract.CmdPalettePinResponse{}},
				{Status: 404, Description: cmdPaletteCommandNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodDelete, Path: "/api/cmd-palette/pins/{id}",
			OperationID: "unpinCmdPaletteCommand", Summary: "Unpin one command palette command",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("id", "Canonical command id"),
				queryParam("workspace", "Workspace id, name, or path", true),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Unpinned", Body: contract.CmdPalettePinResponse{}},
				{Status: 404, Description: cmdPaletteCommandNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/personalization",
			OperationID: "getCmdPalettePersonalization", Summary: "Get command palette personalization summary",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.CmdPalettePersonalizationResponse{}},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodDelete, Path: "/api/cmd-palette/personalization",
			OperationID: "resetCmdPalettePersonalization", Summary: "Reset command palette personalization",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Reset", Body: contract.CmdPalettePersonalizationResetResponse{}},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPost, Path: "/api/cmd-palette/commands/{id}/invoke",
			OperationID: "invokeCmdPaletteCommand", Summary: "Invoke one command palette command",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters:  []ParameterSpec{pathParam("id", "Canonical command id")},
			RequestBody: contract.CmdPaletteInvokeRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Completed", Body: contract.CmdPaletteInvokeResult{}},
				{Status: 202, Description: "Approval pending", Body: contract.CmdPaletteInvokeResult{}},
				{Status: 401, Description: "Invalid client attachment", Body: contract.CmdPaletteError{}},
				{Status: 404, Description: cmdPaletteCommandNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 409, Description: "Invocation conflict", Body: contract.CmdPaletteError{}},
				{Status: 412, Description: "Command unavailable", Body: contract.CmdPaletteError{}},
				{Status: 422, Description: "Invalid command arguments", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/views/{id}",
			OperationID: "getCmdPaletteView", Summary: "Get one declarative command palette view",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("id", "Canonical view id"),
				queryParam("workspace", "Workspace id, name, or path", true),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.CmdPaletteViewEnvelope{}},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 404, Description: cmdPaletteViewNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 422, Description: "Invalid view payload", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/views/{id}/stream",
			OperationID: "streamCmdPaletteView", Summary: "Stream declarative command palette view patches",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("id", "Canonical view id"),
				queryParam("workspace", "Workspace id, name, or path", true),
				queryParam("after", "Last applied patch sequence", false),
				queryParam("stream_epoch", "Stream epoch required when after is greater than zero", false),
			},
			Responses: []ResponseSpec{
				{
					Status: 200, Description: "Revision-fenced view patch stream",
					Body: contract.CmdPaletteViewPatch{}, ContentType: specContentTypeEventStream,
				},
				{Status: 400, Description: "Invalid stream cursor", Body: contract.CmdPaletteError{}},
				{Status: 404, Description: cmdPaletteViewNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPost, Path: "/api/cmd-palette/views/{id}/open",
			OperationID: "openCmdPaletteViewSession", Summary: "Open one programmable command palette view",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("id", "Canonical view id"),
				headerParam("X-Compozy-Client-Token", "Attached client identity token"),
			},
			RequestBody: contract.CmdPaletteViewSessionOpenRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Opened", Body: contract.CmdPaletteViewSessionOpenResponse{}},
				{Status: 401, Description: "Invalid client attachment", Body: contract.CmdPaletteError{}},
				{Status: 404, Description: cmdPaletteViewNotFoundDescription, Body: contract.CmdPaletteError{}},
				{Status: 422, Description: "View is not programmable", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/view-sessions/{session}/stream",
			OperationID: "streamCmdPaletteViewSession", Summary: "Stream one programmable command palette view",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("session", "Opaque view session id"),
				queryParam("token", "Opaque session stream token", true),
			},
			Responses: []ResponseSpec{
				{
					Status: 200, Description: "Session-scoped view frames",
					Body: contract.ViewFrame{}, ContentType: specContentTypeEventStream,
				},
				{Status: 403, Description: "Invalid stream token", Body: contract.CmdPaletteError{}},
				{Status: 410, Description: "View session is gone", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodPost, Path: "/api/cmd-palette/view-sessions/{session}/events",
			OperationID: "admitCmdPaletteViewSessionEvent", Summary: "Send one programmable view event",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("session", "Opaque view session id"),
				headerParam("X-Compozy-Client-Token", "Attached client identity token"),
			},
			RequestBody: contract.CmdPaletteViewSessionEventRequest{},
			Responses: []ResponseSpec{
				{Status: 202, Description: "Accepted", Body: contract.CmdPaletteViewSessionAccepted{}},
				{Status: 403, Description: "Session ownership mismatch", Body: contract.CmdPaletteError{}},
				{Status: 409, Description: "View event cap reached", Body: contract.CmdPaletteError{}},
				{Status: 410, Description: "View session is gone", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodDelete, Path: "/api/cmd-palette/view-sessions/{session}",
			OperationID: "closeCmdPaletteViewSession", Summary: "Close one programmable command palette view",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{
				pathParam("session", "Opaque view session id"),
				headerParam("X-Compozy-Client-Token", "Attached client identity token"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Closed", Body: contract.CmdPaletteViewSessionClosed{}},
				{Status: 403, Description: "Session ownership mismatch", Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/cmd-palette/stream",
			OperationID: "streamCmdPalette", Summary: "Stream command palette catalog invalidations",
			Tags: []string{specCmdPaletteKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{queryParam("workspace", "Workspace id, name, or path", true)},
			Responses: []ResponseSpec{
				{
					Status: 200, Description: "Catalog invalidation stream",
					Body: contract.CmdPaletteCatalogChangedEvent{}, ContentType: specContentTypeEventStream,
				},
				{Status: 400, Description: cmdPaletteInvalidWorkspaceDescription, Body: contract.CmdPaletteError{}},
				{Status: 503, Description: cmdPaletteUnavailableDescription, Body: contract.CmdPaletteError{}},
			},
		},
		{
			Method: httpMethodGet, Path: "/api/tools/approvals/{id}",
			OperationID: "getPendingToolApproval", Summary: "Get one pending tool approval lifecycle",
			Tags: []string{specToolsKey}, Transports: cmdPaletteTransports,
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
			Tags: []string{specToolsKey}, Transports: cmdPaletteTransports,
			Parameters: []ParameterSpec{pathParam("id", "Stable approval id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Canceled", Body: contract.ToolApprovalStatusResponse{}},
				{Status: 404, Description: "Approval not found", Body: contract.CmdPaletteError{}},
				{Status: 409, Description: "Approval already terminal", Body: contract.CmdPaletteError{}},
				{Status: 503, Description: "Tool approval unavailable", Body: contract.CmdPaletteError{}},
			},
		},
	}
)

func cmdPaletteOperations() []OperationSpec {
	return append([]OperationSpec(nil), cmdPaletteOperationSpecs...)
}
