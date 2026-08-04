package cli

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
)

// sessionClientAPI groups the daemon's session-scoped CLI transport surface.
type sessionClientAPI interface {
	ListSessions(context.Context, SessionListQuery) (SessionListPage, error)
	CreateSession(context.Context, CreateSessionRequest) (SessionRecord, error)
	GetSession(context.Context, string) (SessionRecord, error)
	GetSessionHealth(context.Context, string) (SessionHealthRecord, error)
	GetSessionStatus(context.Context, string) (SessionStatusRecord, error)
	GetSessionUsage(context.Context, string) (SessionUsageRecord, error)
	InspectSession(context.Context, string, SessionInspectQuery) (SessionInspectRecord, error)
	RefreshSessionSoul(context.Context, string, SessionSoulRefreshRequest) (AgentSoulRecord, error)
	StopSession(context.Context, string) error
	DeleteSession(context.Context, string) error
	ResumeSession(context.Context, string) (SessionRecord, error)
	SessionRecap(context.Context, string, int) (SessionRecapRecord, error)
	RepairSession(context.Context, string, SessionRepairQuery) (SessionRepairRecord, error)
	ApproveSession(context.Context, string, SessionApprovalRequest) (SessionApprovalRecord, error)
	ListSessionClarifications(context.Context, string) (ClarificationsRecord, error)
	AnswerSessionClarification(
		context.Context,
		string,
		string,
		ClarificationAnswerRequest,
	) (ClarificationAnswerRecord, error)
	PromptSession(context.Context, string, string) ([]AgentEventRecord, error)
	SendSessionPrompt(context.Context, string, SessionPromptRequest) (SessionPromptRecord, error)
	SteerSessionPrompt(context.Context, string, contract.SteerPromptRequest) (SessionPromptRecord, error)
	ListSessionInputs(context.Context, string) (SessionInputListRecord, error)
	ReplaceSessionInput(context.Context, string, string, ReplaceSessionInputRequest) (SessionInputRecord, error)
	PromoteSessionInput(context.Context, string, string, PromoteSessionInputRequest) (SessionPromptRecord, error)
	CancelSessionInput(context.Context, string, string) (SessionPromptRecord, error)
	StreamPromptSession(context.Context, string, SessionPromptRequest, SSEHandler) error
	SessionEvents(context.Context, string, SessionEventQuery) ([]SessionEventRecord, error)
	StreamSessionEvents(context.Context, string, SessionEventQuery, string, SSEHandler) error
	SessionHistory(context.Context, string, SessionEventQuery) ([]TurnHistoryRecord, error)
}
