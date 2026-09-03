package core

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// SessionManager is the runtime session surface exposed by API transports.
// List returns the current in-memory session snapshot without performing I/O.
// ListAll may perform I/O to return the authoritative session set, so it accepts a context.
type SessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	List() []*session.Info
	ListAll(ctx context.Context) ([]*session.Info, error)
	Status(ctx context.Context, id string) (*session.Info, error)
	ActivePromptRun(ctx context.Context, id string) (session.PromptRunIdentity, error)
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
	LatestSessionEventByType(ctx context.Context, id string, eventType string) (*store.SessionEvent, error)
	History(ctx context.Context, id string, query store.EventQuery) ([]store.TurnHistory, error)
	TranscriptPage(ctx context.Context, id string, query transcript.PageQuery) (transcript.Page, error)
	TranscriptChanges(ctx context.Context, id string, query transcript.ChangeQuery) (transcript.ChangePage, error)
	RepairSession(ctx context.Context, opts session.RepairOpts) (*session.RepairResult, error)
	Delete(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
	Resume(ctx context.Context, id string) (*session.Session, error)
	ClearConversation(ctx context.Context, id string) (*session.Session, error)
	RewindConversation(
		ctx context.Context,
		id string,
		opts session.ConversationRewindOptions,
	) (session.ConversationRewindResult, error)
	Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
	PromptWithOpts(ctx context.Context, id string, opts session.PromptOpts) (<-chan acp.AgentEvent, error)
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	SendPrompt(ctx context.Context, id string, opts session.SendPromptOpts) (session.SendPromptResult, error)
	SteerPrompt(ctx context.Context, id string, opts session.SteerPromptOpts) (session.SendPromptResult, error)
	ListPendingInputs(ctx context.Context, id string) ([]session.PendingInput, error)
	ReplacePendingInput(
		ctx context.Context,
		id string,
		entryID string,
		opts session.ReplacePendingInputOpts,
	) (session.PendingInput, error)
	PromotePendingInputToSteer(
		ctx context.Context,
		id string,
		entryID string,
		opts session.PromotePendingInputOpts,
	) (session.SendPromptResult, error)
	CancelQueuedPrompt(ctx context.Context, id string, queueEntryID string) (session.SendPromptResult, error)
	CancelPrompt(ctx context.Context, id string) (session.PromptCancelResult, error)
	ApprovePermission(ctx context.Context, id string, req acp.ApproveRequest) (session.ApprovalResult, error)
	InputQueueSummary(ctx context.Context, id string) (session.InputQueueSummary, error)
}

// SessionAcceptanceManager durably accepts user-created sessions without
// waiting for provider startup.
type SessionAcceptanceManager interface {
	CreateAccepted(ctx context.Context, opts session.CreateAcceptedOpts) (*session.Info, error)
}

// SessionWorktreeForkAcceptanceManager reserves the origin session while a
// clean worktree-bound child is accepted.
type SessionWorktreeForkAcceptanceManager interface {
	CreateWorktreeForkAccepted(context.Context, string, session.CreateAcceptedOpts) (*session.Info, error)
}

// SessionPageManager is the bounded public catalog capability implemented by
// the runtime manager without widening internal full-snapshot consumers.
type SessionPageManager interface {
	ListPage(ctx context.Context, query session.ListQuery) (session.ListPage, error)
}

// AgentSessionMetricsReader exposes exact workspace-scoped session aggregates.
type AgentSessionMetricsReader interface {
	AggregateSessionsByAgent(
		ctx context.Context,
		readScope store.ReadScope,
		workspaceID string,
	) (map[string]session.AgentSessionMetrics, error)
}

// SessionAttachManager owns durable attach CAS and live-session synchronization.
type SessionAttachManager interface {
	AttachSession(ctx context.Context, req store.SessionAttachRequest) (store.SessionAttach, error)
}

// SessionRuntimeSelectionManager owns durable compare-and-set runtime preferences.
type SessionRuntimeSelectionManager interface {
	SetRuntimeSelection(
		ctx context.Context,
		id string,
		selection session.RuntimeSelection,
		expectedRevision int64,
	) (*session.Info, error)
	ClearRuntimeSelection(ctx context.Context, id string, expectedRevision int64) (*session.Info, error)
}

// SessionCatalog exposes daemon-owned session catalog operations that must not
// create a second live session authority.
type SessionCatalog interface {
	ListSessions(ctx context.Context, query store.SessionListQuery) ([]store.SessionInfo, error)
}
