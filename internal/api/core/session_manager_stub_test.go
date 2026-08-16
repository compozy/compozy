package core

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type sessionManagerStub struct {
	create              func(context.Context, session.CreateOpts) (*session.Session, error)
	list                func() []*session.Info
	listAll             func(context.Context) ([]*session.Info, error)
	listPage            func(context.Context, session.ListQuery) (session.ListPage, error)
	status              func(context.Context, string) (*session.Info, error)
	events              func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error)
	latestEvent         func(context.Context, string, string) (*store.SessionEvent, error)
	history             func(context.Context, string, store.EventQuery) ([]store.TurnHistory, error)
	transcriptPage      func(context.Context, string, transcript.PageQuery) (transcript.Page, error)
	transcriptChanges   func(context.Context, string, transcript.ChangeQuery) (transcript.ChangePage, error)
	inputQueueSummary   func(context.Context, string) (session.InputQueueSummary, error)
	repairSession       func(context.Context, session.RepairOpts) (*session.RepairResult, error)
	delete              func(context.Context, string) error
	stop                func(context.Context, string) error
	archive             func(context.Context, string, string) (*session.Info, error)
	unarchive           func(context.Context, string, string) (*session.Info, error)
	stopWithCause       func(context.Context, string, session.StopCause, string) error
	resume              func(context.Context, string) (*session.Session, error)
	clearConversation   func(context.Context, string) (*session.Session, error)
	rewindConversation  func(context.Context, string, session.ConversationRewindOptions) (session.ConversationRewindResult, error)
	prompt              func(context.Context, string, string) (<-chan acp.AgentEvent, error)
	promptSynthetic     func(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	sendPrompt          func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error)
	steerPrompt         func(context.Context, string, session.SteerPromptOpts) (session.SendPromptResult, error)
	listPendingInputs   func(context.Context, string) ([]session.PendingInput, error)
	replacePendingInput func(
		context.Context, string, string, session.ReplacePendingInputOpts,
	) (session.PendingInput, error)
	promotePendingInput func(
		context.Context, string, string, session.PromotePendingInputOpts,
	) (session.SendPromptResult, error)
	cancelQueued      func(context.Context, string, string) (session.SendPromptResult, error)
	cancelPrompt      func(context.Context, string) (session.PromptCancelResult, error)
	approvePermission func(context.Context, string, acp.ApproveRequest) (session.ApprovalResult, error)
}

func (s sessionManagerStub) Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error) {
	if s.create != nil {
		return s.create(ctx, opts)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) List() []*session.Info {
	if s.list != nil {
		return s.list()
	}
	return nil
}

func (s sessionManagerStub) ListAll(ctx context.Context) ([]*session.Info, error) {
	if s.listAll != nil {
		return s.listAll(ctx)
	}
	return nil, nil
}

func (s sessionManagerStub) ListPage(ctx context.Context, query session.ListQuery) (session.ListPage, error) {
	if s.listPage != nil {
		return s.listPage(ctx, query)
	}
	return session.ListPage{}, nil
}

func (s sessionManagerStub) Status(ctx context.Context, id string) (*session.Info, error) {
	if s.status != nil {
		return s.status(ctx, id)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) Events(
	ctx context.Context,
	id string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	if s.events != nil {
		return s.events(ctx, id, query)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) LatestSessionEventByType(
	ctx context.Context,
	id string,
	eventType string,
) (*store.SessionEvent, error) {
	if s.latestEvent != nil {
		return s.latestEvent(ctx, id, eventType)
	}
	return nil, nil
}

func (s sessionManagerStub) History(
	ctx context.Context,
	id string,
	query store.EventQuery,
) ([]store.TurnHistory, error) {
	if s.history != nil {
		return s.history(ctx, id, query)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) TranscriptPage(
	ctx context.Context,
	id string,
	query transcript.PageQuery,
) (transcript.Page, error) {
	if s.transcriptPage != nil {
		return s.transcriptPage(ctx, id, query)
	}
	return transcript.Page{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) TranscriptChanges(
	ctx context.Context,
	id string,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	if s.transcriptChanges != nil {
		return s.transcriptChanges(ctx, id, query)
	}
	return transcript.ChangePage{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) InputQueueSummary(
	ctx context.Context,
	id string,
) (session.InputQueueSummary, error) {
	if s.inputQueueSummary != nil {
		return s.inputQueueSummary(ctx, id)
	}
	return session.InputQueueSummary{}, nil
}

func (s sessionManagerStub) RepairSession(ctx context.Context, opts session.RepairOpts) (*session.RepairResult, error) {
	if s.repairSession != nil {
		return s.repairSession(ctx, opts)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) Delete(ctx context.Context, id string) error {
	if s.delete != nil {
		return s.delete(ctx, id)
	}
	return session.ErrSessionNotFound
}

func (s sessionManagerStub) Stop(ctx context.Context, id string) error {
	if s.stop != nil {
		return s.stop(ctx, id)
	}
	return session.ErrSessionNotFound
}

func (s sessionManagerStub) Archive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if s.archive != nil {
		return s.archive(ctx, workspaceID, sessionID)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) Unarchive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if s.unarchive != nil {
		return s.unarchive(ctx, workspaceID, sessionID)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	if s.stopWithCause != nil {
		return s.stopWithCause(ctx, id, cause, detail)
	}
	return session.ErrSessionNotFound
}

func (s sessionManagerStub) Resume(ctx context.Context, id string) (*session.Session, error) {
	if s.resume != nil {
		return s.resume(ctx, id)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) ClearConversation(ctx context.Context, id string) (*session.Session, error) {
	if s.clearConversation != nil {
		return s.clearConversation(ctx, id)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) RewindConversation(
	ctx context.Context,
	id string,
	opts session.ConversationRewindOptions,
) (session.ConversationRewindResult, error) {
	if s.rewindConversation != nil {
		return s.rewindConversation(ctx, id, opts)
	}
	return session.ConversationRewindResult{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	if s.prompt != nil {
		return s.prompt(ctx, id, msg)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) PromptWithOpts(
	ctx context.Context,
	id string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	return s.Prompt(ctx, id, opts.Message)
}

func (s sessionManagerStub) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	if s.promptSynthetic != nil {
		return s.promptSynthetic(ctx, id, opts)
	}
	return nil, session.ErrSessionNotFound
}

func (s sessionManagerStub) SendPrompt(
	ctx context.Context,
	id string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	if s.sendPrompt != nil {
		return s.sendPrompt(ctx, id, opts)
	}
	return session.SendPromptResult{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) SteerPrompt(
	ctx context.Context,
	id string,
	opts session.SteerPromptOpts,
) (session.SendPromptResult, error) {
	if s.steerPrompt != nil {
		return s.steerPrompt(ctx, id, opts)
	}
	return session.SendPromptResult{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) CancelQueuedPrompt(
	ctx context.Context,
	id string,
	queueEntryID string,
) (session.SendPromptResult, error) {
	if s.cancelQueued != nil {
		return s.cancelQueued(ctx, id, queueEntryID)
	}
	return session.SendPromptResult{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) ListPendingInputs(
	ctx context.Context,
	id string,
) ([]session.PendingInput, error) {
	if s.listPendingInputs != nil {
		return s.listPendingInputs(ctx, id)
	}
	return []session.PendingInput{}, nil
}

func (s sessionManagerStub) ReplacePendingInput(
	ctx context.Context,
	id string,
	entryID string,
	opts session.ReplacePendingInputOpts,
) (session.PendingInput, error) {
	if s.replacePendingInput != nil {
		return s.replacePendingInput(ctx, id, entryID, opts)
	}
	return session.PendingInput{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) PromotePendingInputToSteer(
	ctx context.Context,
	id string,
	entryID string,
	opts session.PromotePendingInputOpts,
) (session.SendPromptResult, error) {
	if s.promotePendingInput != nil {
		return s.promotePendingInput(ctx, id, entryID, opts)
	}
	return session.SendPromptResult{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) CancelPrompt(ctx context.Context, id string) (session.PromptCancelResult, error) {
	if s.cancelPrompt != nil {
		return s.cancelPrompt(ctx, id)
	}
	return session.PromptCancelResult{}, session.ErrSessionNotFound
}

func (s sessionManagerStub) ApprovePermission(
	ctx context.Context,
	id string,
	req acp.ApproveRequest,
) (session.ApprovalResult, error) {
	if s.approvePermission != nil {
		return s.approvePermission(ctx, id, req)
	}
	return session.ApprovalResult{}, session.ErrSessionNotFound
}
