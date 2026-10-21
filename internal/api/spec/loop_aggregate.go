package spec

import "github.com/compozy/compozy/internal/api/contract"

func loopAggregateOperations() []OperationSpec {
	definitions := listLoopsOperation()
	definitions.Path, definitions.OperationID = "/api/loops", "listLoopsAggregate"
	definitions.Responses = []ResponseSpec{ok(contract.LoopCatalogBatchResponse{}), badRequest(), forbidden(), loopUnavailable(), internalError()}
	runs := loopRuntimeOperations()[0]
	runs.Path, runs.OperationID = "/api/loop-runs", "listLoopRunsAggregate"
	runs.Parameters = append(runs.Parameters, enumQueryParam("order", "Aggregate runs support recency order only", []string{"recency"}))
	requests := loopRequestOperations()[0]
	requests.Path, requests.OperationID = "/api/loop-requests", "listLoopRequestsAggregate"
	operations := []OperationSpec{definitions, runs, requests}
	for index := range operations {
		op := &operations[index]
		op.AppAccess = AppAccessDenied
		params := []ParameterSpec{boolQueryParam("all_workspaces", "Read all registered projects and User rows"), {Name: "workspace_ids", In: "query", Kind: "array", Description: "Repeatable project IDs or @user; requires all_workspaces=true"}}
		for _, param := range op.Parameters {
			if param.Name != "workspace_id" {
				params = append(params, param)
			}
		}
		op.Parameters = params
	}
	return operations
}
