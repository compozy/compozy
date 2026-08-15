package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specSessionAttachmentsPath     = specWorkspaceSessionPath + "/attachments"
	specSessionAttachmentBytesPath = specSessionAttachmentsPath + "/{attachment_id}/bytes"
	specSessionAttachmentPath      = specSessionAttachmentsPath + "/{attachment_id}"
)

func sessionAttachmentParameters(includeAttachmentID bool) []ParameterSpec {
	params := []ParameterSpec{
		pathParam("workspace_id", "Workspace id"),
		pathParam("session_id", "Session id"),
	}
	if includeAttachmentID {
		params = append(params, pathParam("attachment_id", "Attachment id"))
	}
	return params
}

func uploadSessionAttachmentOperationSpec() OperationSpec {
	return OperationSpec{
		Method:             httpMethodPost,
		Path:               specSessionAttachmentsPath,
		OperationID:        "uploadSessionAttachment",
		Summary:            "Upload a session attachment",
		Tags:               []string{specSessionsKey},
		Transports:         []Transport{TransportHTTP, TransportUDS},
		Parameters:         sessionAttachmentParameters(false),
		RequestContentType: RequestUploadForm,
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.SessionAttachmentUploadResponse{}},
			{Status: 400, Description: "Invalid attachment upload", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 413, Description: "Attachment exceeds configured size limit", Body: contract.ErrorPayload{}},
			{Status: 415, Description: "Attachment MIME type is not allowed", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func readSessionAttachmentBytesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specSessionAttachmentBytesPath,
		OperationID: "readSessionAttachmentBytes",
		Summary:     "Read session attachment bytes",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  sessionAttachmentParameters(true),
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Attachment bytes",
				Body:        binaryResponse{},
				ContentType: "*/*",
			},
			{Status: 400, Description: "Invalid attachment id", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func deleteSessionAttachmentOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specSessionAttachmentPath,
		OperationID: "deleteSessionAttachment",
		Summary:     "Delete a session attachment",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  sessionAttachmentParameters(true),
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 400, Description: "Invalid attachment id", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
