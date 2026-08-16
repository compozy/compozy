package cli

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

type loopCommandClient interface {
	GetWorkspace(ctx context.Context, ref string) (WorkspaceDetailRecord, error)
	ListLoops(ctx context.Context, workspaceID string, query LoopListQuery) (contract.LoopsResponse, error)
	CreateLoop(
		ctx context.Context,
		workspaceID string,
		request contract.CreateLoopRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopResponse, error)
	GetLoop(ctx context.Context, workspaceID string, name string) (contract.LoopResponse, error)
	PatchLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.PatchLoopRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopResponse, error)
	ValidateLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.ValidateLoopRequest,
	) (contract.LoopValidationResponse, error)
	DeleteLoop(ctx context.Context, workspaceID string, name string, credentials agentidentity.Credentials) error
	RunLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.RunLoopRequest,
		dry bool,
		credentials agentidentity.Credentials,
	) (contract.RunLoopResponse, error)
	GetLoopConfig(ctx context.Context, workspaceID string, name string) (contract.LoopConfigResponse, error)
	PutLoopConfig(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.PutLoopConfigRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopConfigResponse, error)
	ListLoopRuns(ctx context.Context, workspaceID string, query LoopRunListQuery) (contract.LoopRunsResponse, error)
	ListLoopRequests(context.Context, string, LoopRequestListQuery) (contract.LoopRequestsResponse, error)
	GetLoopRequest(context.Context, string, string, string, int) (contract.LoopRequestPayload, error)
	RespondLoopRequest(
		context.Context,
		string,
		string,
		string,
		contract.RespondLoopRequest,
		agentidentity.Credentials,
	) (contract.RespondLoopRequestResponse, error)
	AmendLoopNode(
		context.Context,
		string,
		string,
		string,
		contract.LoopNodeAmendRequest,
		agentidentity.Credentials,
	) (contract.LoopNodeAmendResponse, error)
	ListGoalTurns(
		ctx context.Context,
		workspaceID string,
		runID string,
		query GoalTurnListQuery,
	) (contract.GoalTurnPage, error)
	GetLoopRun(ctx context.Context, workspaceID string, runID string) (contract.LoopRunResponse, error)
	DiffLoopRun(context.Context, string, string, int64, int64, string) (contract.LoopDiffResponse, error)
	RerunLoopRun(
		context.Context, string, string, contract.RerunLoopRequest, agentidentity.Credentials,
	) (contract.RerunLoopResponse, error)
	ForkLoopRun(
		context.Context, string, string, contract.ForkLoopRequest, agentidentity.Credentials,
	) (contract.ForkLoopResponse, error)
	CancelLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	KillLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	ListLoopNodes(
		ctx context.Context,
		workspaceID string,
		query LoopNodeListQuery,
	) (contract.LoopNodeInventoryResponse, error)
	PauseLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		request contract.LoopNodePauseRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	ResumeLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		request contract.LoopNodeResumeRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	CancelLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		request contract.LoopNodeMutationRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	KillLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		request contract.LoopNodeMutationRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	RequeueLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		request contract.LoopNodeMutationRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopMutationResponse, error)
	PauseLoopRun(ctx context.Context, workspaceID string, runID string, credentials agentidentity.Credentials) error
	ResumeLoopRun(ctx context.Context, workspaceID string, runID string, credentials agentidentity.Credentials) error
	ApproveLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		request contract.ApproveLoopRunRequest,
		credentials agentidentity.Credentials,
	) error
}

func loopClientFromDeps(deps commandDeps) (loopCommandClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	loopClient, ok := client.(loopCommandClient)
	if !ok {
		return nil, errors.New("cli: daemon client does not support loop commands")
	}
	return loopClient, nil
}
