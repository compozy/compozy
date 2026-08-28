package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

const terminalPath = "/api/workspaces/{workspace_id}/terminals"

func terminalClientIdentityHeaderParam() ParameterSpec {
	return optionalHeaderParam("X-Compozy-Client-Token", "Registered browser client attachment token")
}

func terminalOperations() []OperationSpec {
	transports := []Transport{TransportHTTP, TransportUDS}
	workspace := pathParam("workspace_id", "Workspace id")
	id := pathParam("id", "Terminal or retained artifact id")
	requestID := pathParam("request_id", "Terminal input request id")
	return []OperationSpec{
		terminalCatalogOperation(transports, workspace),
		terminalListOperation(transports, workspace),
		terminalCreateOperation(transports, workspace),
		terminalGetOperation(transports, workspace, id),
		terminalDeleteOperation(transports, workspace, id),
		terminalTicketOperation(transports, workspace, id),
		terminalStreamOperation(transports, workspace, id),
		terminalExecOperation(transports, workspace),
		terminalReadOperation(transports, workspace, id),
		terminalSignalOperation(transports, workspace, id),
		terminalWaitOperation(transports, workspace, id),
		terminalInputRequestsOperation(transports, workspace),
		terminalAnswerOperation(transports, workspace, id, requestID),
		terminalRejectOperation(transports, workspace, id, requestID),
		terminalRecordingOperation(transports, workspace, id),
		terminalJournalOperation(transports, workspace),
		terminalDownloadOperation(transports, workspace, id, terminalDownloadRecording),
		terminalDownloadOperation(transports, workspace, id, terminalDownloadArtifact),
	}
}

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

func terminalExecOperation(transports []Transport, workspace ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/exec",
		"execTerminal",
		"Execute one supervised workspace command",
		transports,
		withProfileSelector(workspace),
		contract.TerminalExecRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Command finished", Body: contract.TerminalExecResponse{}},
			{Status: 202, Description: "Command continues in a terminal", Body: contract.TerminalExecResponse{}},
			terminalErrorResponse(403, "Approval required or rejected"),
			terminalErrorResponse(422, "Invalid execution request"),
		},
	)
}

func terminalReadOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	params := withProfileSelector(
		workspace, id,
		enumQueryParam("view", "Terminal read view", []string{"screen", "tail", "lines"}),
		intQueryParam("max_bytes", "Maximum returned bytes"), queryParam("grep", "Optional regular expression", false),
		decimalUint64QueryParam("since_seq", "Decimal uint64 after the last consumed sequence", false),
		intQueryParam("from", "First scrollback line"),
		intQueryParam("to", "Last scrollback line"),
	)
	return terminalOperation(
		httpMethodGet,
		terminalPath+"/{id}/read",
		"readTerminal",
		"Read bounded untrusted terminal output",
		transports,
		params,
		nil,
		[]ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TerminalReadResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(410, "Terminal expired"),
			terminalErrorResponse(422, "Unsupported terminal read"),
		},
	)
}

func terminalSignalOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/{id}/signal",
		"signalTerminal",
		"Signal one running terminal",
		transports,
		withProfileSelector(workspace, id),
		contract.TerminalSignalRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Delivered", Body: contract.TerminalDeliveredResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(409, "Terminal state or controller conflict"),
			terminalErrorResponse(422, "Invalid signal"),
		},
	)
}

func terminalWaitOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/{id}/wait",
		"waitTerminal",
		"Wait for one terminal condition",
		transports,
		withProfileSelector(workspace, id),
		contract.TerminalWaitRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Wait result", Body: contract.TerminalWaitResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(410, "Terminal expired"),
			terminalErrorResponse(422, "Invalid wait request"),
		},
	)
}

func terminalInputRequestsOperation(transports []Transport, workspace ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodGet,
		terminalPath+"/input-requests",
		"listTerminalInputRequests",
		"List pending terminal input requests",
		transports,
		withProfileScope(workspace, queryParam("terminal_id", "Filter by terminal id", false)),
		nil,
		[]ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TerminalInputRequestsResponse{}},
			terminalErrorResponse(422, "Invalid profile selection"),
			terminalErrorResponse(503, "Terminal service unavailable"),
		},
	)
}

func terminalAnswerOperation(transports []Transport, workspace, id, requestID ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/{id}/input-requests/{request_id}/answer",
		"answerTerminalInputRequest",
		"Answer one terminal input request",
		transports,
		withProfileSelector(workspace, id, requestID),
		contract.TerminalAnswerInputRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Delivered", Body: contract.TerminalInputAnswerResponse{}},
			terminalErrorResponse(403, "Terminal write lease required"),
			terminalErrorResponse(404, "Input request not found"),
			terminalErrorResponse(409, "Input request already answered"),
		},
	)
}

func terminalRejectOperation(transports []Transport, workspace, id, requestID ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/{id}/input-requests/{request_id}/reject",
		"rejectTerminalInputRequest",
		"Reject one terminal input request",
		transports,
		withProfileSelector(workspace, id, requestID),
		contract.TerminalRejectInputRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Rejected", Body: contract.TerminalInputRejectResponse{}},
			terminalErrorResponse(403, "Terminal write lease required"),
			terminalErrorResponse(404, "Input request not found"),
			terminalErrorResponse(409, "Input request already answered"),
		},
	)
}

func terminalRecordingOperation(transports []Transport, workspace, id ParameterSpec) OperationSpec {
	return terminalOperation(
		httpMethodPost,
		terminalPath+"/{id}/recording",
		"controlTerminalRecording",
		"Start or stop terminal recording",
		transports,
		withProfileSelector(workspace, id),
		contract.TerminalRecordingRequest{},
		[]ResponseSpec{
			{Status: 200, Description: "Recording state", Body: contract.TerminalRecordingResponse{}},
			terminalErrorResponse(404, "Terminal not found"),
			terminalErrorResponse(409, "Recording state conflict"),
			terminalErrorResponse(422, "Invalid recording action"),
		},
	)
}

func terminalJournalOperation(transports []Transport, workspace ParameterSpec) OperationSpec {
	params := withProfileScope(
		workspace, enumQueryParam("actor", "Filter by actor kind", []string{"human", "agent"}),
		queryParam("since", "Filter by relative duration", false), boolQueryParam("failed", "Only failed commands"),
		queryParam("terminal_id", "Filter by terminal id", false), intQueryParam("limit", "Page size"),
		queryParam("cursor", "Opaque page cursor", false),
	)
	return terminalOperation(
		httpMethodGet,
		terminalPath+"/journal",
		"queryTerminalJournal",
		"Query terminal command history",
		transports,
		params,
		nil,
		[]ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TerminalJournalResponse{}},
			terminalErrorResponse(422, "Invalid journal query"),
			terminalErrorResponse(503, "Terminal journal unavailable"),
		},
	)
}

type terminalDownloadKind uint8

const (
	terminalDownloadArtifact terminalDownloadKind = iota
	terminalDownloadRecording
)

func terminalDownloadOperation(
	transports []Transport,
	workspace, id ParameterSpec,
	kind terminalDownloadKind,
) OperationSpec {
	segment := "/artifacts/{id}"
	operationID := "downloadTerminalArtifact"
	summary := "Download one terminal spill artifact"
	contentType := specBinaryContentType
	if kind == terminalDownloadRecording {
		segment = "/recordings/{id}"
		operationID = "downloadTerminalRecording"
		summary = "Download one terminal recording"
		contentType = "application/x-asciicast"
	}
	return terminalOperation(
		httpMethodGet,
		terminalPath+segment,
		operationID,
		summary,
		transports,
		withProfileSelector(workspace, id),
		nil,
		[]ResponseSpec{
			{Status: 200, Description: "Artifact bytes", Body: binaryResponse{}, ContentType: contentType},
			terminalErrorResponse(404, "Terminal artifact not found"),
			terminalErrorResponse(503, "Terminal journal unavailable"),
		},
	)
}

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
