package spec

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func sessionTranscriptOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript",
			OperationID: "getSessionTranscript",
			Summary:     "Get a bounded materialized transcript page",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
				intQueryParam(
					"limit",
					"Maximum page size; defaults to the newest 200 entries and is capped at 1000",
				),
				beforeSequenceQueryParam(
					"Return entries whose stable start sequence is before this cursor",
				),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SessionTranscriptResponse{}},
				{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Transcript projection is incompatible", Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/stream",
			OperationID: "streamSession",
			Summary:     "Stream raw session events or fenced transcript changes",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
				optionalHeaderParam(
					"Last-Event-ID",
					"Resume after the last applied SSE cursor; transcript reconnects also require epoch and generation",
				),
				afterSequenceQueryParam("Initial cursor when Last-Event-ID is not supplied"),
				enumQueryParam(
					"frames",
					"Frame mode; transcript is the default and raw preserves persisted event frames",
					[]string{contract.SessionStreamFrameRaw, contract.SessionStreamFrameTranscript},
				),
				transcriptFenceQueryParam("epoch", "Expected transcript epoch for a fenced reconnect"),
				transcriptFenceQueryParam(
					"generation",
					"Expected materialized projection generation for a fenced reconnect",
				),
				intQueryParam("limit", "Maximum entries per transcript snapshot or change batch"),
			},
			Responses: []ResponseSpec{
				{
					Status:      200,
					Description: "Session event stream",
					Body:        contract.SessionStreamPayload{},
					ContentType: specContentTypeEventStream,
				},
				{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Transcript projection is incompatible", Body: contract.ErrorPayload{}},
			},
		},
	}
}

func transcriptFenceQueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        specIntegerKey,
		Format:      specFormatInt64,
	}
}
