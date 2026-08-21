package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	profilesPath          = "/api/profiles"
	profileNamePath       = profilesPath + "/{name}"
	profileSelectionPath  = profilesPath + "/selection"
	profileOperationsPath = profilesPath + "/ops"
)

func registryProfileOperations() []OperationSpec {
	return []OperationSpec{
		profileListOperation(), profileCreateOperation(), profileGetOperation(), profileUpdateOperation(),
		profileRenamePlanOperation(), profileRenameOperation(), profileArchivePlanOperation(), profileArchiveOperation(),
		profileUnarchiveOperation(), profileDeletePlanOperation(), profileDeleteOperation(),
		profileSelectionListOperation(), profileSelectionPutOperation(), profileOperationsListOperation(), profileOperationRetryOperation(),
	}
}

func profileListOperation() OperationSpec {
	return profileReadOperation(httpMethodGet, profilesPath, "listProfiles", "List profiles", []contract.Profile{})
}

func profileCreateOperation() OperationSpec {
	return profileWriteOperation(httpMethodPost, profilesPath, "createProfile", "Create a profile", contract.CreateProfileRequest{}, 201, contract.Profile{})
}

func profileGetOperation() OperationSpec {
	op := profileReadOperation(httpMethodGet, profileNamePath, "getProfile", "Get a profile", contract.Profile{})
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileUpdateOperation() OperationSpec {
	op := profileWriteOperation(httpMethodPatch, profileNamePath, "updateProfile", "Update profile identity", contract.UpdateProfileRequest{}, 200, contract.Profile{})
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileRenamePlanOperation() OperationSpec {
	op := profileReadOperation(httpMethodGet, profileNamePath+"/rename-plan", "prepareProfileRename", "Preview a profile rename", contract.RenameProfilePlan{})
	op.Parameters = []ParameterSpec{profileNameParameter(), queryParam("new_name", "New profile name", true)}
	return op
}

func profileRenameOperation() OperationSpec {
	op := profileWriteOperation(httpMethodPost, profileNamePath+"/rename", "renameProfile", "Rename a profile", contract.RenameProfileRequest{}, 200, contract.RenameProfileResponse{})
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileArchivePlanOperation() OperationSpec {
	op := profileReadOperation(httpMethodGet, profileNamePath+"/archive-plan", "prepareProfileArchive", "Preview profile archival", contract.ArchiveProfilePlan{})
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileArchiveOperation() OperationSpec {
	op := profileWriteOperation(httpMethodPost, profileNamePath+"/archive", "archiveProfile", "Archive a profile", contract.ProfilePlanRequest{}, 200, contract.ArchiveProfileResponse{})
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileUnarchiveOperation() OperationSpec {
	op := profileWriteOperation(httpMethodPost, profileNamePath+"/unarchive", "unarchiveProfile", "Unarchive a profile", struct{}{}, 200, contract.UnarchiveProfileResponse{})
	op.RequestBodyOptional = true
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileDeletePlanOperation() OperationSpec {
	op := profileReadOperation(httpMethodGet, profileNamePath+"/delete-plan", "prepareProfileDelete", "Preview profile deletion", contract.DeleteProfilePlan{})
	op.Parameters = []ParameterSpec{profileNameParameter()}
	return op
}

func profileDeleteOperation() OperationSpec {
	op := profileWriteOperation(httpMethodDelete, profileNamePath, "deleteProfile", "Delete a profile", nil, 200, contract.DeleteProfileResponse{})
	op.Parameters = []ParameterSpec{profileNameParameter(), queryParam("plan_revision", "Revision returned by the delete plan", true)}
	return op
}

func profileSelectionListOperation() OperationSpec {
	op := profileReadOperation(httpMethodGet, profileSelectionPath, "getProfileSelections", "Read profile selections", []contract.ProfileSelection{})
	op.Parameters = []ParameterSpec{
		enumQueryParam("scope", "Selection scope", []string{"global", "workspace"}),
		queryParam("workspace_id", "Workspace selection lens", false),
	}
	return op
}

func profileSelectionPutOperation() OperationSpec {
	return profileWriteOperation(httpMethodPut, profileSelectionPath, "putProfileSelection", "Remember a profile selection", contract.ProfileSelection{}, 200, contract.ProfileSelection{})
}

func profileOperationsListOperation() OperationSpec {
	return profileReadOperation(httpMethodGet, profileOperationsPath, "listProfileOperations", "List profile lifecycle operations", []contract.ProfileOperation{})
}

func profileOperationRetryOperation() OperationSpec {
	op := profileWriteOperation(httpMethodPost, profileOperationsPath+"/{op_id}/retry", "retryProfileOperation", "Retry a profile lifecycle operation", nil, 200, contract.ProfileOperation{})
	op.Parameters = []ParameterSpec{pathParam("op_id", "Lifecycle operation id")}
	return op
}

func profileReadOperation(method, path, id, summary string, body any) OperationSpec {
	return OperationSpec{
		Method: method, Path: path, OperationID: id, Summary: summary,
		Tags: []string{specProfilesKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: body},
			{Status: 400, Description: "Invalid profile request", Body: contract.ProfileErrorPayload{}},
			{Status: 404, Description: "Profile not found", Body: contract.ProfileErrorPayload{}},
			{Status: 409, Description: "Profile conflict", Body: contract.ProfileErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ProfileErrorPayload{}},
		},
	}
}

func profileWriteOperation(method, path, id, summary string, request any, status int, body any) OperationSpec {
	op := profileReadOperation(method, path, id, summary, body)
	op.RequestBody = request
	op.Responses[0].Status = status
	op.Responses = append(op.Responses, ResponseSpec{
		Status: 403, Description: "Remote profile management is forbidden", Body: contract.ProfileErrorPayload{},
	})
	return op
}

func profileNameParameter() ParameterSpec { return pathParam("name", "Profile name") }
