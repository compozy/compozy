package core

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// LoopService exposes the daemon-owned Loop public API facade to transports.
type LoopService interface {
	ListLoops(
		ctx context.Context,
		workspaceID string,
		query looppkg.CatalogQuery,
	) (contract.LoopsResponse, error)
	CreateLoop(ctx context.Context, workspaceID string, req contract.CreateLoopRequest) (contract.LoopResponse, error)
	GetLoop(ctx context.Context, workspaceID string, name string) (contract.LoopResponse, error)
	PatchLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		req contract.PatchLoopRequest,
	) (contract.LoopResponse, error)
	ValidateLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		req contract.ValidateLoopRequest,
	) (contract.LoopValidationResponse, error)
	DeleteLoop(ctx context.Context, workspaceID string, name string) error
	RunLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		req contract.RunLoopRequest,
		startKind dsl.StartKind,
		actor taskpkg.ActorContext,
		dry bool,
	) (contract.RunLoopResponse, error)
	GetLoopConfig(ctx context.Context, workspaceID string, name string) (contract.LoopConfigResponse, error)
	PutLoopConfig(
		ctx context.Context,
		workspaceID string,
		name string,
		req contract.PutLoopConfigRequest,
	) (contract.LoopConfigResponse, error)
	GetLoopInputDefaults(
		ctx context.Context,
		workspaceID string,
		name string,
		scope contract.LoopInputDefaultsScope,
	) (contract.LoopInputDefaultsResponse, error)
	GetLoopInputDefault(
		ctx context.Context,
		workspaceID string,
		name string,
		key string,
		scope contract.LoopInputDefaultsScope,
	) (contract.LoopInputDefaultResponse, error)
	PutLoopInputDefaults(
		ctx context.Context,
		workspaceID string,
		name string,
		req contract.PutLoopInputDefaultsRequest,
	) (contract.LoopInputDefaultsResponse, error)
	PutLoopInputDefault(
		ctx context.Context,
		workspaceID string,
		name string,
		key string,
		req contract.PutLoopInputDefaultRequest,
	) (contract.LoopInputDefaultResponse, error)
	DeleteLoopInputDefault(
		ctx context.Context,
		workspaceID string,
		name string,
		key string,
		scope contract.LoopInputDefaultsScope,
	) (contract.DeleteLoopInputDefaultResponse, error)
	GetLoopAnnotations(ctx context.Context, workspaceID string, name string) (contract.LoopAnnotationsResponse, error)
	PutLoopAnnotations(
		ctx context.Context,
		workspaceID string,
		name string,
		req contract.PutLoopAnnotationsRequest,
	) (contract.LoopAnnotationsResponse, error)
	ListLoopRuns(ctx context.Context, workspaceID string, query LoopRunListQuery) (contract.LoopRunsResponse, error)
	GetLoopRun(ctx context.Context, workspaceID string, runID string) (contract.LoopRunResponse, error)
	DiffLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		query looppkg.DiffQuery,
	) (contract.LoopDiffResponse, error)
	RerunLoopRun(
		context.Context,
		string,
		string,
		contract.RerunLoopRequest,
		taskpkg.ActorContext,
	) (contract.RerunLoopResponse, error)
	ForkLoopRun(
		context.Context,
		string,
		string,
		contract.ForkLoopRequest,
		taskpkg.ActorContext,
	) (contract.ForkLoopResponse, error)
	GetSessionGoal(ctx context.Context, workspaceID string, sessionID string) (*session.GoalSnapshot, error)
	ListGoalTurns(
		ctx context.Context,
		workspaceID string,
		runID string,
		query GoalTurnListQuery,
	) (session.GoalTurnPage, error)
	CancelLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	KillLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	PauseLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		req contract.LoopNodePauseRequest,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	ResumeLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		req contract.LoopNodeResumeRequest,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	CancelLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		req contract.LoopNodeMutationRequest,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	KillLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		req contract.LoopNodeMutationRequest,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	RequeueLoopNode(
		ctx context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		req contract.LoopNodeMutationRequest,
		actor taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error)
	ListLoopNodes(
		ctx context.Context,
		workspaceID string,
		query LoopNodeListQuery,
	) (contract.LoopNodeInventoryResponse, error)
	PauseLoopRun(ctx context.Context, workspaceID string, runID string, actor taskpkg.ActorContext) error
	ResumeLoopRun(ctx context.Context, workspaceID string, runID string, actor taskpkg.ActorContext) error
	ApproveLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		req contract.ApproveLoopRunRequest,
		actor taskpkg.ActorContext,
	) error
	ListLoopRequests(context.Context, string, LoopRequestListQuery) (contract.LoopRequestsResponse, error)
	GetLoopRequest(context.Context, string, string, int, string, int) (contract.LoopRequestPayload, error)
	RespondLoopRequest(
		context.Context,
		string,
		string,
		string,
		contract.RespondLoopRequest,
		taskpkg.ActorContext,
	) (contract.RespondLoopRequestResponse, error)
	AmendLoopNode(
		context.Context,
		string,
		string,
		string,
		contract.LoopNodeAmendRequest,
		taskpkg.ActorContext,
	) (contract.LoopNodeAmendResponse, error)
	ListLoopRunEvents(
		ctx context.Context,
		workspaceID string,
		runID string,
		afterSeq int64,
	) ([]contract.LoopRunEventPayload, error)
}

// LoopRunListQuery contains HTTP/UDS list filters for loop runs.
type LoopRunListQuery struct {
	ReadScope     store.ReadScope
	LoopName      string
	Status        string
	Origin        string
	OriginSession string
	Live          *bool
	Limit         int
}

// LoopRequestListQuery contains request inventory filters shared by HTTP and UDS.
type LoopRequestListQuery struct {
	RunID  string
	State  string
	Cursor string
	Limit  int
}

// LoopNodeListQuery contains workspace node-inventory filters.
type LoopNodeListQuery struct {
	State    string
	LoopName string
	RunID    string
	Cursor   string
	Limit    int
}

// GoalTurnListQuery contains validated public turn-audit filters.
type GoalTurnListQuery struct {
	NodeID    string
	ItemIndex *int
	AfterSeq  int64
	Limit     int
}
