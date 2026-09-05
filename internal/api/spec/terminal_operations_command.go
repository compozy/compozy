package spec

import "github.com/compozy/compozy/internal/api/contract"

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
			terminalErrorResponse(409, "Terminal state conflict"),
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
