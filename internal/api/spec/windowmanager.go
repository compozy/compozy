package spec

import "github.com/compozy/agh/internal/api/contract"

const (
	windowManagerPath                   = "/api/workspaces/{workspace_id}/window-manager"
	windowManagerUnavailableDescription = "Window manager unavailable"
)

func windowManagerOperations() []OperationSpec {
	transports := []Transport{TransportHTTP, TransportUDS}
	workspaceParam := pathParam("workspace_id", "Workspace id")
	errorBody := contract.WindowManagerErrorPayload{}
	readErrors := []ResponseSpec{
		{Status: 404, Description: specWorkspaceNotFoundDescription, Body: errorBody},
		{Status: 503, Description: windowManagerUnavailableDescription, Body: errorBody},
	}
	mutationErrors := []ResponseSpec{
		{Status: 404, Description: "Workspace or referenced entity not found", Body: errorBody},
		{Status: 409, Description: "Revision or state conflict", Body: errorBody},
		{Status: 422, Description: "Invalid window-manager request", Body: errorBody},
		{Status: 503, Description: windowManagerUnavailableDescription, Body: errorBody},
	}

	operations := windowManagerStateOperations(transports, workspaceParam, readErrors, mutationErrors)
	operations = append(
		operations,
		windowManagerClientOperations(transports, workspaceParam, readErrors, mutationErrors)...,
	)
	operations = append(
		operations,
		windowManagerLayoutOperations(transports, workspaceParam, readErrors, mutationErrors)...,
	)
	return append(operations, windowManagerStreamOperation(transports, workspaceParam, errorBody))
}

func windowManagerStateOperations(
	transports []Transport,
	workspaceParam ParameterSpec,
	readErrors []ResponseSpec,
	mutationErrors []ResponseSpec,
) []OperationSpec {
	return []OperationSpec{
		{
			Method: httpMethodGet, Path: windowManagerPath,
			OperationID: "getWindowManagerSnapshot", Summary: "Read one workspace window-manager snapshot",
			Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
			Responses: append(
				[]ResponseSpec{{Status: 200, Description: "OK", Body: contract.WindowManagerSnapshot{}}},
				readErrors...,
			),
		},
		windowManagerMutationOperation(
			httpMethodPost,
			windowManagerPath+"/preview",
			"previewWindowManagerCommand",
			"Preview one semantic window-manager command",
			contract.WindowManagerPreview{},
			transports,
			workspaceParam,
			mutationErrors,
		),
		windowManagerMutationOperation(
			httpMethodPost,
			windowManagerPath+"/commands",
			"executeWindowManagerCommand",
			"Execute one semantic window-manager command",
			contract.WindowManagerResult{},
			transports,
			workspaceParam,
			mutationErrors,
		),
	}
}

func windowManagerClientOperations(
	transports []Transport,
	workspaceParam ParameterSpec,
	readErrors []ResponseSpec,
	mutationErrors []ResponseSpec,
) []OperationSpec {
	clientParam := pathParam("client_id", "Connected window-manager client id")
	return []OperationSpec{
		{
			Method: httpMethodGet, Path: windowManagerPath + "/clients",
			OperationID: "listWindowManagerClients", Summary: "List connected window-manager clients",
			Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
			Responses: append(
				[]ResponseSpec{{Status: 200, Description: "OK", Body: contract.WindowManagerClientsResponse{}}},
				readErrors...,
			),
		},
		{
			Method: httpMethodPost, Path: windowManagerPath + "/clients",
			OperationID: "registerWindowManagerClient", Summary: "Register a window-manager client view",
			Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
			RequestBody: contract.WindowManagerClientRegistration{},
			Responses: append(
				[]ResponseSpec{{Status: 201, Description: "Registered", Body: contract.WindowManagerClientView{}}},
				mutationErrors...,
			),
		},
		{
			Method:      httpMethodDelete,
			Path:        windowManagerPath + "/clients/{client_id}",
			OperationID: "unregisterWindowManagerClient",
			Summary:     "Unregister a window-manager client view",
			Tags:        []string{specWorkspacesKey},
			Transports:  transports,
			Parameters:  []ParameterSpec{workspaceParam, clientParam},
			Responses: append(
				[]ResponseSpec{{Status: 204, Description: specNoContentDescription}},
				mutationErrors...,
			),
		},
	}
}

func windowManagerLayoutOperations(
	transports []Transport,
	workspaceParam ParameterSpec,
	readErrors []ResponseSpec,
	mutationErrors []ResponseSpec,
) []OperationSpec {
	operations := []OperationSpec{
		{
			Method: httpMethodGet, Path: windowManagerPath + "/layout",
			OperationID: "exportWindowManagerLayout", Summary: "Export one declarative window layout",
			Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
			Responses: append(
				[]ResponseSpec{{Status: 200, Description: "OK", Body: contract.WindowManagerLayoutDocument{}}},
				readErrors...,
			),
		},
		{
			Method: httpMethodPost, Path: windowManagerPath + "/layout/validate",
			OperationID: "validateWindowManagerLayout", Summary: "Validate one declarative window layout",
			Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
			RequestBody: contract.WindowManagerLayoutValidationRequest{},
			Responses: append(
				[]ResponseSpec{
					{
						Status:      200,
						Description: "Validation result",
						Body:        contract.WindowManagerLayoutValidationResponse{},
					},
				},
				mutationErrors...,
			),
		},
		{
			Method: httpMethodPut, Path: windowManagerPath + "/layout",
			OperationID: "replaceWindowManagerLayout", Summary: "Replace one declarative window layout",
			Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
			RequestBody: contract.WindowManagerLayoutReplaceRequest{},
			Responses: append(
				[]ResponseSpec{{Status: 200, Description: "Applied", Body: contract.WindowManagerResult{}}},
				mutationErrors...,
			),
		},
	}
	return append(
		operations,
		windowManagerLayoutProfileOperations(transports, workspaceParam)...,
	)
}

func windowManagerLayoutProfileOperations(
	transports []Transport,
	workspaceParam ParameterSpec,
) []OperationSpec {
	profileParam := pathParam("profile_id", "Layout profile id")
	return []OperationSpec{
		listWindowManagerLayoutProfilesOperation(transports, workspaceParam),
		putWindowManagerLayoutProfileOperation(transports, workspaceParam, profileParam),
		deleteWindowManagerLayoutProfileOperation(transports, workspaceParam, profileParam),
	}
}

func listWindowManagerLayoutProfilesOperation(
	transports []Transport,
	workspaceParam ParameterSpec,
) OperationSpec {
	resourceError := contract.ErrorPayload{}
	resourceOrWindowManagerError := []any{resourceError, contract.WindowManagerErrorPayload{}}
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        windowManagerPath + "/layout-profiles",
		OperationID: "listWindowManagerLayoutProfiles",
		Summary:     "List layout profiles visible to one workspace",
		Tags:        []string{specWorkspacesKey},
		Transports:  transports,
		Parameters:  []ParameterSpec{workspaceParam},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ResourcesResponse{}},
			{
				Status: 404, Description: specWorkspaceNotFoundDescription,
				Bodies: resourceOrWindowManagerError,
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: resourceError},
			{
				Status: 503, Description: windowManagerUnavailableDescription,
				Bodies: resourceOrWindowManagerError,
			},
		},
	}
}

func putWindowManagerLayoutProfileOperation(
	transports []Transport,
	workspaceParam ParameterSpec,
	profileParam ParameterSpec,
) OperationSpec {
	resourceError := contract.ErrorPayload{}
	resourceOrWindowManagerError := []any{resourceError, contract.WindowManagerErrorPayload{}}
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        windowManagerPath + "/layout-profiles/{profile_id}",
		OperationID: "putWindowManagerLayoutProfile",
		Summary:     "Create or replace one workspace-visible layout profile",
		Tags:        []string{specWorkspacesKey},
		Transports:  transports,
		Parameters:  []ParameterSpec{workspaceParam, profileParam},
		RequestBody: contract.PutResourceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Updated", Body: contract.ResourceResponse{}},
			{Status: 201, Description: specCreatedDescription, Body: contract.ResourceResponse{}},
			{Status: 400, Description: "Invalid JSON body", Body: resourceError},
			{Status: 403, Description: specForbiddenDescription, Body: resourceError},
			{
				Status: 404, Description: "Workspace or profile not found",
				Bodies: resourceOrWindowManagerError,
			},
			{Status: 409, Description: "Version conflict", Body: resourceError},
			{Status: 413, Description: specPayloadTooLargeDescription, Body: resourceError},
			{Status: 422, Description: "Invalid layout profile", Body: resourceError},
			{Status: 429, Description: specRateLimitedDescription, Body: resourceError},
			{Status: 500, Description: specInternalServerErrorDescription, Body: resourceError},
			{
				Status: 503, Description: windowManagerUnavailableDescription,
				Bodies: resourceOrWindowManagerError,
			},
		},
	}
}

func deleteWindowManagerLayoutProfileOperation(
	transports []Transport,
	workspaceParam ParameterSpec,
	profileParam ParameterSpec,
) OperationSpec {
	resourceError := contract.ErrorPayload{}
	resourceOrWindowManagerError := []any{resourceError, contract.WindowManagerErrorPayload{}}
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        windowManagerPath + "/layout-profiles/{profile_id}",
		OperationID: "deleteWindowManagerLayoutProfile",
		Summary:     "Delete one workspace-visible layout profile",
		Tags:        []string{specWorkspacesKey},
		Transports:  transports,
		Parameters:  []ParameterSpec{workspaceParam, profileParam},
		RequestBody: contract.DeleteResourceRequest{},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 400, Description: "Invalid JSON body", Body: resourceError},
			{Status: 403, Description: specForbiddenDescription, Body: resourceError},
			{
				Status: 404, Description: "Workspace or profile not found",
				Bodies: resourceOrWindowManagerError,
			},
			{Status: 409, Description: "Version conflict", Body: resourceError},
			{Status: 422, Description: "Invalid delete request", Body: resourceError},
			{Status: 429, Description: specRateLimitedDescription, Body: resourceError},
			{Status: 500, Description: specInternalServerErrorDescription, Body: resourceError},
			{
				Status: 503, Description: windowManagerUnavailableDescription,
				Bodies: resourceOrWindowManagerError,
			},
		},
	}
}

func windowManagerMutationOperation(
	method string,
	path string,
	operationID string,
	summary string,
	response any,
	transports []Transport,
	workspaceParam ParameterSpec,
	errors []ResponseSpec,
) OperationSpec {
	return OperationSpec{
		Method: method, Path: path, OperationID: operationID, Summary: summary,
		Tags: []string{specWorkspacesKey}, Transports: transports, Parameters: []ParameterSpec{workspaceParam},
		RequestBody: contract.WindowManagerCommandRequest{},
		Responses:   append([]ResponseSpec{{Status: 200, Description: "OK", Body: response}}, errors...),
	}
}

func windowManagerStreamOperation(
	transports []Transport,
	workspaceParam ParameterSpec,
	errorBody contract.WindowManagerErrorPayload,
) OperationSpec {
	after := intQueryParam("after_revision", "Last applied window-manager revision")
	after.Format = specFormatInt64
	maximum := float64(contract.WindowManagerMaxSafeRevision)
	after.Maximum = &maximum
	clientID := queryParam("client_id", "Optional connected client bound to presentation updates", false)
	return OperationSpec{
		Method: httpMethodGet, Path: windowManagerPath + "/stream",
		OperationID: "streamWindowManager", Summary: "Stream a topology and optional client presentation fence",
		Tags: []string{specWorkspacesKey}, Transports: transports,
		Parameters: []ParameterSpec{workspaceParam, after, clientID},
		Responses: []ResponseSpec{
			{
				Status:      101,
				Description: "WebSocket upgrade and frame contract",
				Bodies: []any{
					contract.WindowManagerSnapshotFrame{},
					contract.WindowManagerEventFrame{},
					contract.WindowManagerClientFrame{},
					contract.WindowManagerErrorFrame{},
				},
			},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: errorBody},
			{Status: 409, Description: "Requested revision is ahead", Body: errorBody},
			{Status: 422, Description: "Invalid after_revision", Body: errorBody},
			{Status: 503, Description: windowManagerUnavailableDescription, Body: errorBody},
		},
	}
}
