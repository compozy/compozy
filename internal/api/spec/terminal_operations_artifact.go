package spec

import "github.com/compozy/compozy/internal/api/contract"

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
