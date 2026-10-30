package spec

import "github.com/compozy/compozy/internal/api/contract"

// sessionMonitorOperations describes the operator reads and cancellation of a
// session's daemon-owned monitors.
func sessionMonitorOperations() []OperationSpec {
	const monitorsPath = "/api/workspaces/{workspace_id}/sessions/{session_id}/monitors"
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        monitorsPath,
			AppAccess:   AppAccessAllowed,
			OperationID: "listSessionMonitors",
			Summary:     "List a session's monitors",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			// The owner selector reaches sessions outside the default profile.
			Parameters: withProfileScope(
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			),
			Responses: sessionMonitorResponses(contract.SessionMonitorsResponse{}),
		},
		{
			Method:      httpMethodPost,
			Path:        monitorsPath + "/{monitor_id}/cancel",
			AppAccess:   AppAccessAllowed,
			OperationID: "cancelSessionMonitor",
			Summary:     "Cancel a session monitor without waking the session",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
				pathParam("monitor_id", "Monitor id"),
			),
			Responses: sessionMonitorResponses(contract.SessionMonitorResponse{}),
		},
	}
}

func sessionMonitorResponses(body any) []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: specSuccessDescription, Body: body},
		{Status: 400, Description: "Invalid profile selection", Body: contract.ProfileErrorPayload{}},
		{Status: 404, Description: "Session or monitor not found", Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Session monitors unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}
