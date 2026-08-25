package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specSessionGoalPath = "/api/workspaces/{workspace_id}/sessions/{session_id}/goal"
	specGoalTurnsPath   = "/api/workspaces/{workspace_id}/loop-runs/{run_id}/turns"
)

func goalOperations() []OperationSpec {
	return []OperationSpec{
		loopOperation(
			httpMethodPost,
			specSessionGoalPath,
			"mutateSessionGoal",
			"Set or control the authenticated session Goal",
			contract.SessionGoalCommandRequest{},
			[]ParameterSpec{workspaceIDParam(), pathParam("session_id", "Session id")},
			[]ResponseSpec{
				{Status: 200, Description: "Goal operation completed", Body: contract.GoalCommandResult{}},
				{Status: 202, Description: "Goal operation accepted", Body: contract.GoalCommandResult{}},
				{Status: 400, Description: "Invalid Goal command", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Goal caller is not authorized for the target session", Body: contract.GoalCommandResult{}},
				{Status: 404, Description: "Session or Goal not found", Body: contract.GoalCommandResult{}},
				{Status: 409, Description: "Goal state is stale or requires replacement", Body: contract.GoalCommandResult{}},
				{Status: 422, Description: "Goal operation is invalid", Body: contract.GoalCommandResult{}},
				loopUnavailable(),
				internalError(),
			},
		),
		loopOperation(
			httpMethodGet,
			specSessionGoalPath,
			"getSessionGoal",
			"Get the newest visible session Goal",
			nil,
			[]ParameterSpec{workspaceIDParam(), pathParam("session_id", "Session id")},
			[]ResponseSpec{
				ok(contract.SessionGoalResponse{}),
				badRequest(),
				notFound("Session not found in workspace"),
				loopUnavailable(),
				internalError(),
			},
		),
		loopOperation(
			httpMethodGet,
			specGoalTurnsPath,
			"listGoalTurns",
			"List Goal turns in run-wide sequence order",
			nil,
			[]ParameterSpec{
				workspaceIDParam(),
				loopRunIDParam(),
				queryParam("node", "Filter by Goal node id", false),
				intQueryParam("item", "Filter by nonnegative item index"),
				intQueryParam("after_seq", "Resume after this run-wide Goal sequence"),
				intQueryParam("limit", "Page size from 1 to 200; defaults to 50"),
			},
			[]ResponseSpec{
				ok(contract.GoalTurnPage{}),
				badRequest(),
				notFound(specLoopRunNotFound),
				goalTurnFilterInvalid(),
				loopUnavailable(),
				internalError(),
			},
		),
	}
}

func goalTurnFilterInvalid() ResponseSpec {
	reason := contract.GoalReasonTurnFilterInvalid
	return ResponseSpec{
		Status:      422,
		Description: "Goal turn filter is invalid",
		Body: contract.GoalCommandResult{
			Outcome:    contract.GoalOutcomeError,
			ReasonCode: &reason,
		},
	}
}

func promptSuccessResponse(status int, description string) ResponseSpec {
	return ResponseSpec{
		Status:      status,
		Description: description,
		Body:        contract.SendPromptResultResponse{},
	}
}

func promptGoalErrorResponse(status int, description string) ResponseSpec {
	return ResponseSpec{
		Status:      status,
		Description: description,
		Bodies: responseBodiesOf(
			responseBodyOf[contract.ErrorPayload](),
			responseBodyOf[contract.SendPromptResultResponse](),
		),
	}
}
