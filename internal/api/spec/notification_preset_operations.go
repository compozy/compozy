package spec

import "github.com/compozy/agh/internal/api/contract"

// Operations returns the canonical REST operation registry in deterministic order.
func notificationPresetOperations() []OperationSpec {
	return []OperationSpec{
		listNotificationPresetsOperation(),
		createNotificationPresetOperation(),
		getNotificationPresetOperation(),
		updateNotificationPresetOperation(),
		deleteNotificationPresetOperation(),
	}
}

func listNotificationPresetsOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPINotificationsPresetsPath,
		OperationID: "listNotificationPresets",
		Summary:     "List notification presets",
		Tags:        []string{specNotificationsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			boolQueryParam("enabled", "Filter by enabled state"),
			boolQueryParam("built_in", "Filter by built-in state"),
			queryParam("name", "Filter by exact preset name", false),
			intQueryParam("limit", "Maximum number of presets to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NotificationPresetListResponse{}},
			{Status: 400, Description: "Invalid notification preset filter", Body: contract.ErrorPayload{}},
			notificationPresetServiceUnavailableResponse(),
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func createNotificationPresetOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPINotificationsPresetsPath,
		OperationID: "createNotificationPreset",
		Summary:     "Create a notification preset",
		Tags:        []string{specNotificationsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateNotificationPresetRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.NotificationPresetResponse{}},
			{Status: 400, Description: "Invalid notification preset", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Notification preset already exists", Body: contract.ErrorPayload{}},
			notificationPresetServiceUnavailableResponse(),
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func getNotificationPresetOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPINotificationsPresetsNamePath,
		OperationID: "getNotificationPreset",
		Summary:     "Get one notification preset",
		Tags:        []string{specNotificationsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("name", "Notification preset name")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NotificationPresetResponse{}},
			{Status: 404, Description: specNotificationPresetNotFoundDescription, Body: contract.ErrorPayload{}},
			notificationPresetServiceUnavailableResponse(),
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateNotificationPresetOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPINotificationsPresetsNamePath,
		OperationID: "updateNotificationPreset",
		Summary:     "Update one notification preset",
		Tags:        []string{specNotificationsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("name", "Notification preset name")},
		RequestBody: contract.UpdateNotificationPresetRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NotificationPresetResponse{}},
			{Status: 400, Description: "Invalid notification preset update", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specNotificationPresetNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Bridge target lookup is ambiguous", Body: contract.ErrorPayload{}},
			notificationPresetServiceUnavailableResponse(),
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func deleteNotificationPresetOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPINotificationsPresetsNamePath,
		OperationID: "deleteNotificationPreset",
		Summary:     "Delete one custom notification preset",
		Tags:        []string{specNotificationsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("name", "Notification preset name")},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specNotificationPresetNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Built-in notification preset cannot be deleted", Body: contract.ErrorPayload{}},
			notificationPresetServiceUnavailableResponse(),
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func notificationPresetServiceUnavailableResponse() ResponseSpec {
	return ResponseSpec{
		Status:      503,
		Description: specNotificationPresetServiceIsNotConfiguredDescription,
		Body:        contract.ErrorPayload{},
	}
}
