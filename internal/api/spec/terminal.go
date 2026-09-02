package spec

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
