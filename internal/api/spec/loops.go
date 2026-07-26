package spec

import "github.com/compozy/agh/internal/api/contract"

const (
	specLoopsKey          = "loops"
	specAPILoopsPath      = "/api/workspaces/{workspace_id}/loops"
	specAPILoopRunsPath   = "/api/workspaces/{workspace_id}/loop-runs"
	specLoopRunNotFound   = "Loop run not found"
	specLoopDefinition404 = "Loop definition not found"
)

func loopsOperations() []OperationSpec {
	operations := loopCatalogOperations()
	operations = append(operations, loopRuntimeOperations()...)
	return operations
}

func loopCatalogOperations() []OperationSpec {
	return []OperationSpec{
		listLoopsOperation(),
		createLoopOperation(),
		getLoopOperation(),
		patchLoopOperation(),
		deleteLoopOperation(),
		validateLoopOperation(),
		runLoopOperation(),
		getLoopConfigOperation(),
		putLoopConfigOperation(),
		getLoopAnnotationsOperation(),
		putLoopAnnotationsOperation(),
	}
}

func listLoopsOperation() OperationSpec {
	return loopOperation(httpMethodGet, loopsPath(), "listLoops", "List Loop catalog entries", nil,
		append([]ParameterSpec{workspaceIDParam()}, loopCatalogQueryParams()...),
		[]ResponseSpec{
			ok(contract.LoopsResponse{}),
			badRequest(),
			notFound(specWorkspaceNotFoundDescription),
			{Status: 410, Description: workspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			loopUnavailable(),
			internalError(),
		})
}

func createLoopOperation() OperationSpec {
	return loopOperation(
		httpMethodPost,
		loopsPath(),
		"createLoop",
		"Create or fork a Loop definition",
		contract.CreateLoopRequest{},
		[]ParameterSpec{workspaceIDParam()},
		[]ResponseSpec{
			created(contract.LoopResponse{}),
			badRequest(),
			conflict(),
			notFound(specLoopDefinition404),
			lintFailed(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func getLoopOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		loopPath(),
		"getLoop",
		"Inspect one Loop definition",
		nil,
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{
			ok(contract.LoopResponse{}),
			badRequest(),
			notFound(specLoopDefinition404),
			loopUnavailable(),
			internalError(),
		},
	)
}

func patchLoopOperation() OperationSpec {
	return loopOperation(
		httpMethodPatch,
		loopPath(),
		"patchLoop",
		"Publish one Loop definition with optimistic concurrency",
		contract.PatchLoopRequest{},
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{
			ok(contract.LoopResponse{}),
			badRequest(),
			forbidden(),
			notFound(specLoopDefinition404),
			versionConflict(),
			lintFailed(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func deleteLoopOperation() OperationSpec {
	return loopOperation(
		httpMethodDelete,
		loopPath(),
		"deleteLoop",
		"Delete one writable Loop definition",
		nil,
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			badRequest(),
			forbidden(),
			notFound(specLoopDefinition404),
			loopUnavailable(),
			internalError(),
		},
	)
}

func validateLoopOperation() OperationSpec {
	return loopOperation(
		httpMethodPost,
		loopPath()+"/validate",
		"validateLoop",
		"Validate one Loop definition without saving",
		contract.ValidateLoopRequest{},
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{
			ok(contract.LoopValidationResponse{}),
			badRequest(),
			lintFailed(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func runLoopOperation() OperationSpec {
	return loopOperation(
		httpMethodPost,
		loopPath()+"/run",
		"runLoop",
		"Start or dry-run one Loop",
		contract.RunLoopRequest{},
		[]ParameterSpec{
			workspaceIDParam(),
			loopNameParam(),
			boolQueryParam("dry", "Preview the run without creating durable state"),
		},
		[]ResponseSpec{
			ok(contract.RunLoopResponse{}),
			created(contract.RunLoopResponse{}),
			unauthorized(),
			badRequest(),
			forbidden(),
			conflict(),
			loopUnprocessable(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func getLoopConfigOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		loopPath()+"/config",
		"getLoopConfig",
		"Get Loop config override",
		nil,
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{
			ok(contract.LoopConfigResponse{}),
			badRequest(),
			notFound("Loop config not found"),
			loopUnavailable(),
			internalError(),
		},
	)
}

func putLoopConfigOperation() OperationSpec {
	return loopOperation(
		httpMethodPut,
		loopPath()+"/config",
		"putLoopConfig",
		"Replace Loop config override",
		contract.PutLoopConfigRequest{},
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{
			ok(contract.LoopConfigResponse{}),
			badRequest(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func getLoopAnnotationsOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		loopPath()+"/annotations",
		"getLoopAnnotations",
		"Get Loop editor annotations",
		nil,
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{ok(contract.LoopAnnotationsResponse{}), badRequest(), loopUnavailable(), internalError()},
	)
}

func putLoopAnnotationsOperation() OperationSpec {
	return loopOperation(
		httpMethodPut,
		loopPath()+"/annotations",
		"putLoopAnnotations",
		"Replace Loop editor annotations",
		contract.PutLoopAnnotationsRequest{},
		[]ParameterSpec{workspaceIDParam(), loopNameParam()},
		[]ResponseSpec{ok(contract.LoopAnnotationsResponse{}), badRequest(), loopUnavailable(), internalError()},
	)
}

func loopRuntimeOperations() []OperationSpec {
	return []OperationSpec{
		loopOperation(
			httpMethodGet,
			loopRunsPath(),
			"listLoopRuns",
			"List Loop runs",
			nil,
			[]ParameterSpec{
				workspaceIDParam(),
				queryParam("loop", "Filter by Loop name", false),
				queryParam("status", "Filter by Loop status", false),
				queryParam("origin", "Filter by catalog or session origin", false),
				queryParam("origin_session", "Filter by origin session id", false),
				boolQueryParam("live", "Filter by live or terminal status"),
				intQueryParam("limit", "Maximum number of records to return"),
			},
			[]ResponseSpec{ok(contract.LoopRunsResponse{}), badRequest(), loopUnavailable(), internalError()},
		),
		loopOperation(
			httpMethodGet,
			loopRunPath(),
			"getLoopRun",
			"Get one Loop run",
			nil,
			[]ParameterSpec{workspaceIDParam(), loopRunIDParam()},
			[]ResponseSpec{
				ok(contract.LoopRunResponse{}),
				badRequest(),
				notFound(specLoopRunNotFound),
				loopUnavailable(),
				internalError(),
			},
		),
		loopRunMutationOperation("stopLoopRun", "Stop one Loop run", "/stop"),
		loopRunMutationOperation("pauseLoopRun", "Pause one Loop run", "/pause"),
		loopRunMutationOperation("resumeLoopRun", "Resume one Loop run", "/resume"),
		loopOperation(
			httpMethodPost,
			loopRunPath()+"/approve",
			"approveLoopRun",
			"Approve one Loop gate",
			contract.ApproveLoopRunRequest{},
			[]ParameterSpec{workspaceIDParam(), loopRunIDParam()},
			[]ResponseSpec{
				ok(map[string]bool{"ok": true}),
				badRequest(),
				notFound(specLoopRunNotFound),
				conflict(),
				loopUnprocessable(),
				loopUnavailable(),
				internalError(),
			},
		),
		loopOperation(
			httpMethodGet,
			loopRunPath()+"/events",
			"streamLoopRunEvents",
			"Stream Loop run events",
			nil,
			[]ParameterSpec{
				workspaceIDParam(),
				loopRunIDParam(),
				queryParam("after_sequence", "Resume after this sequence", false),
				optionalHeaderParam("Last-Event-ID", "Last received event sequence"),
			},
			[]ResponseSpec{
				{
					Status:      200,
					Description: "SSE stream",
					Body:        contract.LoopRunEventPayload{},
					ContentType: specContentTypeEventStream,
				},
				badRequest(),
				notFound(specLoopRunNotFound),
				loopUnavailable(),
				internalError(),
			},
		),
	}
}

func applyLoopAutomationContract(ops []OperationSpec) []OperationSpec {
	for idx := range ops {
		switch ops[idx].OperationID {
		case "listAutomationJobs", "listAutomationTriggers":
			ops[idx].Parameters = append(ops[idx].Parameters, queryParam("loop", "Filter by Loop target name", false))
		case "createAutomationJob", "updateAutomationJob", "createAutomationTrigger", "updateAutomationTrigger":
			ops[idx].Responses = append(ops[idx].Responses, ResponseSpec{
				Status:      422,
				Description: "Automation Loop target validation failed",
				Body:        contract.ErrorPayload{},
			})
		}
	}
	return ops
}

func loopOperation(
	method string,
	path string,
	operationID string,
	summary string,
	request any,
	params []ParameterSpec,
	responses []ResponseSpec,
) OperationSpec {
	return OperationSpec{
		Method:      method,
		Path:        path,
		OperationID: operationID,
		Summary:     summary,
		Tags:        []string{specLoopsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  params,
		RequestBody: request,
		Responses:   responses,
	}
}

func loopRunMutationOperation(operationID string, summary string, suffix string) OperationSpec {
	return loopOperation(
		httpMethodPost,
		loopRunPath()+suffix,
		operationID,
		summary,
		map[string]any{},
		[]ParameterSpec{workspaceIDParam(), loopRunIDParam()},
		[]ResponseSpec{
			ok(map[string]bool{"ok": true}),
			badRequest(),
			notFound(specLoopRunNotFound),
			conflict(),
			loopUnprocessable(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func loopsPath() string {
	return specAPILoopsPath
}

func loopPath() string {
	return loopsPath() + "/{name}"
}

func loopRunsPath() string {
	return specAPILoopRunsPath
}

func loopRunPath() string {
	return loopRunsPath() + "/{run_id}"
}

func workspaceIDParam() ParameterSpec {
	return pathParam("workspace_id", "Workspace id")
}

func loopNameParam() ParameterSpec {
	return pathParam("name", "Loop name")
}

func loopRunIDParam() ParameterSpec {
	return pathParam("run_id", "Loop run id")
}

func ok(body any) ResponseSpec {
	return ResponseSpec{Status: 200, Description: "OK", Body: body}
}

func created(body any) ResponseSpec {
	return ResponseSpec{Status: 201, Description: specCreatedDescription, Body: body}
}

func badRequest() ResponseSpec {
	return ResponseSpec{Status: 400, Description: "Invalid Loop request", Body: contract.ErrorPayload{}}
}

func unauthorized() ResponseSpec {
	return ResponseSpec{
		Status:      401,
		Description: specAgentCallerIdentityIsMissingDescription,
		Body:        contract.ErrorPayload{},
	}
}

func forbidden() ResponseSpec {
	return ResponseSpec{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}}
}

func conflict() ResponseSpec {
	return ResponseSpec{Status: 409, Description: "Loop conflict", Body: contract.ErrorPayload{}}
}

func versionConflict() ResponseSpec {
	return ResponseSpec{Status: 409, Description: "Loop version conflict", Body: contract.LoopVersionConflictResponse{}}
}

func lintFailed() ResponseSpec {
	return ResponseSpec{Status: 422, Description: "Loop validation failed", Body: contract.LoopValidationResponse{}}
}

func loopUnprocessable() ResponseSpec {
	return ResponseSpec{Status: 422, Description: "Loop operation rejected", Body: contract.ErrorPayload{}}
}

func notFound(description string) ResponseSpec {
	return ResponseSpec{Status: 404, Description: description, Body: contract.ErrorPayload{}}
}

func loopUnavailable() ResponseSpec {
	return ResponseSpec{Status: 503, Description: "Loop service is not configured", Body: contract.ErrorPayload{}}
}

func internalError() ResponseSpec {
	return ResponseSpec{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}}
}
