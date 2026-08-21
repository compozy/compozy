package spec

import "github.com/compozy/compozy/internal/api/contract"

const specAPILoopNodesPath = "/api/workspaces/{workspace_id}/loop-nodes"

func loopNodeLifecycleOperations() []OperationSpec {
	return []OperationSpec{
		loopRunLifecycleMutation("cancelLoopRun", "Cancel one Loop run", "/cancel"),
		loopRunLifecycleMutation("killLoopRun", "Kill one Loop run", "/kill"),
		loopNodeMutationOperation(
			"pauseLoopNode",
			"Pause one Loop node",
			"/pause",
			contract.LoopNodePauseRequest{},
		),
		loopNodeMutationOperation(
			"resumeLoopNode",
			"Resume one Loop node",
			"/resume",
			contract.LoopNodeResumeRequest{},
		),
		loopNodeMutationOperation(
			"cancelLoopNode",
			"Cancel one Loop node",
			"/cancel",
			contract.LoopNodeMutationRequest{},
		),
		loopNodeMutationOperation(
			"killLoopNode",
			"Kill one Loop node",
			"/kill",
			contract.LoopNodeMutationRequest{},
		),
		loopNodeMutationOperation(
			"requeueLoopNode",
			"Requeue one Loop node",
			"/requeue",
			contract.LoopNodeMutationRequest{},
		),
		listLoopNodesOperation(),
	}
}

func loopRunLifecycleMutation(operationID string, summary string, suffix string) OperationSpec {
	return loopOperation(
		httpMethodPost,
		loopRunPath()+suffix,
		operationID,
		summary,
		map[string]any{},
		withProfileSelector(workspaceIDParam(), loopRunIDParam()),
		loopLifecycleMutationResponses(),
	)
}

func loopNodeMutationOperation(
	operationID string,
	summary string,
	suffix string,
	request any,
) OperationSpec {
	return loopOperation(
		httpMethodPost,
		loopRunPath()+"/nodes/{node_id}"+suffix,
		operationID,
		summary,
		request,
		withProfileSelector(workspaceIDParam(), loopRunIDParam(), pathParam("node_id", "Loop node id")),
		loopLifecycleMutationResponses(),
	)
}

func listLoopNodesOperation() OperationSpec {
	return loopOperation(
		httpMethodGet,
		specAPILoopNodesPath,
		"listLoopNodes",
		"List workspace Loop node state",
		nil,
		withProfileScope(
			workspaceIDParam(),
			queryParam("state", "Node inventory state", true),
			queryParam("loop", "Filter by Loop name", false),
			queryParam("run_id", "Filter by Loop run id", false),
			queryParam("cursor", "Opaque continuation cursor", false),
			intQueryParam("limit", "Maximum number of records to return"),
		),
		[]ResponseSpec{
			ok(contract.LoopNodeInventoryResponse{}),
			badRequest(),
			loopUnavailable(),
			internalError(),
		},
	)
}

func loopLifecycleMutationResponses() []ResponseSpec {
	return []ResponseSpec{
		ok(contract.LoopMutationResponse{}),
		badRequest(),
		notFound(specLoopRunNotFound),
		conflict(),
		loopUnprocessable(),
		loopUnavailable(),
		internalError(),
	}
}
