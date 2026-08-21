package spec

import "github.com/compozy/compozy/internal/api/contract"

func loopInputDefaultsOperations() []OperationSpec {
	return []OperationSpec{
		loopOperation(
			httpMethodGet,
			loopInputDefaultsPath(),
			"getLoopInputDefaults",
			"Get one scoped Loop input-default table",
			nil,
			loopInputDefaultsParameters(false),
			loopInputDefaultsResponses(contract.LoopInputDefaultsResponse{}),
		),
		loopOperation(
			httpMethodPut,
			loopInputDefaultsPath(),
			"putLoopInputDefaults",
			"Replace one scoped Loop input-default table",
			contract.PutLoopInputDefaultsRequest{},
			[]ParameterSpec{workspaceIDParam(), loopNameParam()},
			loopInputDefaultsWriteResponses(contract.LoopInputDefaultsResponse{}),
		),
		loopOperation(
			httpMethodGet,
			loopInputDefaultPath(),
			"getLoopInputDefault",
			"Get one scoped Loop input default",
			nil,
			loopInputDefaultsParameters(true),
			loopInputDefaultsResponses(contract.LoopInputDefaultResponse{}),
		),
		loopOperation(
			httpMethodPut,
			loopInputDefaultPath(),
			"putLoopInputDefault",
			"Set one scoped Loop input default",
			contract.PutLoopInputDefaultRequest{},
			[]ParameterSpec{workspaceIDParam(), loopNameParam(), loopInputDefaultKeyParam()},
			loopInputDefaultsWriteResponses(contract.LoopInputDefaultResponse{}),
		),
		loopOperation(
			httpMethodDelete,
			loopInputDefaultPath(),
			"deleteLoopInputDefault",
			"Delete one scoped Loop input default",
			nil,
			loopInputDefaultsParameters(true),
			loopInputDefaultsResponses(contract.DeleteLoopInputDefaultResponse{}),
		),
	}
}

func loopInputDefaultsPath() string {
	return loopPath() + "/input-defaults"
}

func loopInputDefaultPath() string {
	return loopInputDefaultsPath() + "/{key}"
}

func loopInputDefaultsParameters(includeKey bool) []ParameterSpec {
	parameters := []ParameterSpec{workspaceIDParam(), loopNameParam()}
	if includeKey {
		parameters = append(parameters, loopInputDefaultKeyParam())
	}
	scope := requiredEnumQueryParam(
		"scope",
		"Config scope: user or workspace",
		[]string{
			string(contract.LoopInputDefaultsScopeUser),
			string(contract.LoopInputDefaultsScopeWorkspace),
		},
	)
	return append(parameters, scope)
}

func loopInputDefaultKeyParam() ParameterSpec {
	return pathParam("key", "Declared Loop input name")
}

func loopInputDefaultsResponses(body any) []ResponseSpec {
	return []ResponseSpec{
		ok(body),
		badRequest(),
		notFound(specWorkspaceNotFoundDescription),
		{Status: 410, Description: workspaceRootMissingDescription, Body: contract.ErrorPayload{}},
		loopUnavailable(),
		internalError(),
	}
}

func loopInputDefaultsWriteResponses(body any) []ResponseSpec {
	responses := loopInputDefaultsResponses(body)
	return append(responses[:2], append([]ResponseSpec{loopInputUnprocessable()}, responses[2:]...)...)
}
