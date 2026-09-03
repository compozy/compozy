package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

func terminalCatalogOperation(transports []Transport, workspace ParameterSpec) OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: terminalPath + "/stream", OperationID: "streamTerminalCatalog",
		Summary: "Stream one profile-scoped terminal catalog", Tags: []string{specWorkspacesKey},
		Transports: transports,
		Parameters: withProfileSelector(
			workspace,
			optionalLastEventIDHeaderParam("Resume after the last terminal catalog event"),
		),
		Responses: terminalResponseMatrix([]ResponseSpec{
			{
				Status:      200,
				Description: "Terminal catalog event stream",
				Body:        contract.TerminalCatalogSnapshot{},
				ContentType: specContentTypeEventStream,
			},
			terminalErrorResponse(422, "Invalid terminal catalog cursor"),
			terminalErrorResponse(503, "Terminal catalog unavailable"),
		}),
	}
}

func terminalListOperation(transports []Transport, workspace ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodGet,
		terminalPath,
		"listTerminals",
		"List workspace terminals",
		transports,
		withProfileScope(workspace),
		nil,
		[]ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TerminalListResponse{}},
			terminalErrorResponse(422, "Invalid profile selection"),
			terminalErrorResponse(503, "Terminal service unavailable"),
		},
	)
}

func terminalCreateOperation(transports []Transport, workspace ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath,
		"createTerminal",
		"Create an interactive terminal",
		transports,
		withProfileSelector(
			workspace,
			terminalClientIdentityHeaderParam(),
		),
		contract.TerminalCreateRequest{},
		[]ResponseSpec{
			{Status: 201, Description: "Created", Body: contract.TerminalResponse{}},
			terminalErrorResponse(409, "Terminal limit reached"),
			terminalErrorResponse(422, "Invalid terminal request"),
			terminalErrorResponse(503, "Terminal service unavailable"),
		},
	)
}

func terminalGetOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodGet,
		terminalPath+"/{id}",
		"getTerminal",
		"Read one terminal",
		transports,
		withProfileSelector(workspace, id),
		nil,
		[]ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TerminalResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(410, "Terminal expired"),
			terminalErrorResponse(503, "Terminal service unavailable"),
		},
	)
}

func terminalDeleteOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	operation := terminalOperation(
		httpMethodDelete,
		terminalPath+"/{id}",
		"deleteTerminal",
		"Terminate one terminal",
		transports,
		withProfileSelector(workspace, id),
		contract.TerminalCloseRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Terminated", Body: contract.TerminalExitResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(409, "Terminal state or controller conflict"),
			terminalErrorResponse(410, "Terminal expired"),
		},
	)
	operation.RequestBodyOptional = true
	return operation
}

func terminalTicketOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/{id}/attach-ticket",
		"mintTerminalAttachTicket",
		"Mint a single-use terminal attach ticket",
		transports,
		withProfileSelector(
			workspace,
			id,
			terminalClientIdentityHeaderParam(),
		),
		contract.TerminalAttachTicketRequest{},
		[]ResponseSpec{
			{Status: 201, Description: "Created", Body: contract.TerminalAttachTicketResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(409, "Subscriber limit reached"),
			terminalErrorResponse(422, "Invalid attach mode"),
		},
	)
}

func terminalStreamOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	after := decimalUint64QueryParam("after_seq", "Decimal uint64 after the last parsed terminal sequence", false)
	return terminalOperation(
		httpMethodGet,
		terminalPath+"/{id}/stream",
		"streamTerminal",
		"Upgrade to one live terminal byte stream",
		transports,
		[]ParameterSpec{
			workspace, id,
			queryParam("ticket", "Single-use attach ticket", true),
			enumQueryParam("mode", "Attach mode", []string{"read", "write"}),
			intQueryParam("cols", "Proposed terminal columns"), intQueryParam("rows", "Proposed terminal rows"),
			after, enumQueryParam("flow", "Flow-control mode", []string{"ack", "drop"}),
		},
		nil,
		[]ResponseSpec{
			{
				Status:      101,
				Description: terminalwire.ProtocolDescription,
			},
			terminalErrorResponse(403, "Terminal ticket invalid or expired"),
			terminalErrorResponse(409, "Subscriber limit reached"),
		},
	)
}
