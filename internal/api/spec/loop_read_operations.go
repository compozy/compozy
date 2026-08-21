package spec

import "github.com/compozy/compozy/internal/api/contract"

func loopRunNodesOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		loopRunPath()+"/nodes",
		"getLoopRunNodes",
		"Get the computed Loop run node roster",
		nil,
		[]ParameterSpec{
			workspaceIDParam(),
			loopRunIDParam(),
			queryParam("state", "Filter by roster state", false),
			intQueryParam("generation", "Filter by generation"),
			queryParam("cursor", "Opaque roster cursor", false),
			intQueryParam("limit", "Maximum rows to return"),
		},
		[]ResponseSpec{
			ok(contract.LoopRunNodesResponse{}),
			badRequest(),
			notFound(specLoopRunNotFound),
			loopUnavailable(),
			internalError(),
		},
	)
}

func loopRunBriefingOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		loopRunPath()+"/briefing",
		"getLoopRunBriefing",
		"Explain the current Loop run state",
		nil,
		[]ParameterSpec{workspaceIDParam(), loopRunIDParam()},
		[]ResponseSpec{
			ok(contract.LoopBriefingResponse{}),
			notFound(specLoopRunNotFound),
			loopUnavailable(),
			internalError(),
		},
	)
}

func loopRunTimelineOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		loopRunPath()+"/timeline",
		"getLoopRunTimeline",
		"Read the durable Loop run timeline",
		nil,
		[]ParameterSpec{
			workspaceIDParam(),
			loopRunIDParam(),
			queryParam("view", "Timeline view: notable or all", false),
			queryParam("cursor", "Opaque snapshot-fenced cursor", false),
			intQueryParam("limit", "Maximum entries to return"),
			intQueryParam("after_sequence", "Return entries after this per-run sequence"),
		},
		[]ResponseSpec{
			ok(contract.LoopTimelineResponse{}),
			badRequest(),
			notFound(specLoopRunNotFound),
			conflict(),
			loopUnavailable(),
			internalError(),
		},
	)
}
