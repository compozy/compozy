package spec

import "github.com/compozy/agh/internal/api/contract"

func taskRunConversationStreamOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/task-runs/{id}/conversation/stream",
		OperationID: "streamTaskRunConversation",
		Summary:     "Stream one task run coordination conversation",
		Tags:        []string{specTasksKey, specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
			queryParam("after", "Replay messages after this message id", false),
		},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Task run conversation stream",
				Body:        contract.TaskRunConversationStreamPayload{},
				ContentType: specContentTypeEventStream,
			},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Local task run has no coordination conversation",
				Body:        contract.ErrorPayload{},
			},
			{Status: 422, Description: "Invalid task-run id or conversation cursor", Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Task or Network service is not configured", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
