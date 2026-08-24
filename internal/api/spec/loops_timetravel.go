package spec

import "github.com/compozy/compozy/internal/api/contract"

func loopRequestAndTimeTravelOperations() []OperationSpec {
	return append(loopRequestOperations(), loopTimeTravelOperations()...)
}

func loopRequestOperations() []OperationSpec {
	return []OperationSpec{
		loopOperation(
			httpMethodGet,
			"/api/workspaces/{workspace_id}/loop-requests",
			"listLoopRequests",
			"List Loop human requests",
			nil,
			withProfileScope(
				workspaceIDParam(),
				queryParam("run_id", "Filter by Loop run id", false),
				queryParam("state", "Filter by pending or resolved state", false),
				queryParam("cursor", "Opaque continuation cursor", false),
				intQueryParam("limit", "Maximum number of records to return"),
			),
			[]ResponseSpec{ok(contract.LoopRequestsResponse{}), badRequest(), loopUnavailable(), internalError()},
		),
		loopOperation(
			httpMethodGet,
			loopRunPath()+"/nodes/{node_id}/request",
			"getLoopRequest",
			"Get one Loop human request",
			nil,
			withProfileScope(
				workspaceIDParam(), loopRunIDParam(), pathParam("node_id", "Loop node id"),
				requiredIntQueryParam("generation", "Loop generation"),
				intQueryParam("item_index", "Fan-out lane index"),
			),
			[]ResponseSpec{ok(contract.LoopRequestPayload{}), badRequest(), notFound("Loop request not found"),
				loopUnavailable(), internalError()},
		),
		loopOperation(
			httpMethodPost,
			loopRunPath()+"/nodes/{node_id}/respond",
			"respondLoopRequest",
			"Answer one Loop human request",
			contract.RespondLoopRequest{},
			withProfileSelector(workspaceIDParam(), loopRunIDParam(), pathParam("node_id", "Loop node id")),
			[]ResponseSpec{ok(contract.RespondLoopRequestResponse{}), badRequest(), forbidden(),
				notFound("Loop request not found"), conflict(),
				{Status: 410, Description: "Loop request is closed", Body: contract.ErrorPayload{}},
				loopInputUnprocessable(), loopUnavailable(), internalError()},
		),
		loopOperation(
			httpMethodPost,
			loopRunPath()+"/nodes/{node_id}/amend",
			"amendLoopNode",
			"Amend one parked Loop node output",
			contract.LoopNodeAmendRequest{},
			withProfileSelector(workspaceIDParam(), loopRunIDParam(), pathParam("node_id", "Loop node id")),
			[]ResponseSpec{
				ok(contract.LoopNodeAmendResponse{}), badRequest(), forbidden(),
				notFound(specLoopRunNotFound), conflict(), loopInputUnprocessable(),
				loopUnavailable(), internalError(),
			},
		),
	}
}

func loopTimeTravelOperations() []OperationSpec {
	return []OperationSpec{
		loopOperation(
			httpMethodGet,
			loopRunPath()+"/diff",
			"diffLoopRun",
			"Compare Loop generations or runs",
			nil,
			withProfileScope(
				workspaceIDParam(), loopRunIDParam(),
				intQueryParam("generation", "Base generation"),
				intQueryParam("against_generation", "Generation to compare"),
				queryParam("against_run", "Run to compare", false),
			),
			[]ResponseSpec{ok(contract.LoopDiffResponse{}), badRequest(), notFound(specLoopRunNotFound),
				loopUnprocessable(), loopUnavailable(), internalError()},
		),
		loopOperation(
			httpMethodPost,
			loopRunPath()+"/rerun",
			"rerunLoopRun",
			"Rerun a Loop from one settled node",
			contract.RerunLoopRequest{},
			withProfileSelector(workspaceIDParam(), loopRunIDParam()),
			[]ResponseSpec{ok(contract.RerunLoopResponse{}), badRequest(), forbidden(),
				notFound(specLoopRunNotFound), conflict(), loopUnprocessable(), loopUnavailable(), internalError()},
		),
		loopOperation(
			httpMethodPost,
			loopRunPath()+"/fork",
			"forkLoopRun",
			"Fork a linked Loop run from history",
			contract.ForkLoopRequest{},
			withProfileSelector(workspaceIDParam(), loopRunIDParam()),
			[]ResponseSpec{
				created(contract.ForkLoopResponse{}),
				badRequest(),
				forbidden(),
				notFound(
					"Loop generation not found",
				),
				conflict(),
				loopInputUnprocessable(),
				loopUnavailable(),
				internalError(),
			},
		),
	}
}
