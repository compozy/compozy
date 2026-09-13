package spec

import "github.com/compozy/compozy/internal/api/contract"

func updateExtensionsOperationSpec() OperationSpec {
	operation := updateExtensionOperationSpec()
	operation.Method = httpMethodPost
	operation.Path = "/api/extensions/update"
	operation.OperationID = "updateExtensions"
	operation.Summary = "Update installed extensions and report each outcome"
	operation.Parameters = nil
	operation.RequestBody = contract.UpdateExtensionsRequest{}
	operation.Responses[0] = ResponseSpec{
		Status:      200,
		Description: "Per-extension outcomes, including partial failures",
		Body:        contract.ExtensionUpdateBatchResponse{},
	}
	return operation
}
