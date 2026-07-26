package testutil

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/acp"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

type StubSessionManager struct {
	CreateFn            func(context.Context, session.CreateOpts) (*session.Session, error)
	CreateAcceptedFn    func(context.Context, session.CreateAcceptedOpts) (*session.Info, error)
	ListFn              func() []*session.Info
	ListAllFn           func(context.Context) ([]*session.Info, error)
	ListPageFn          func(context.Context, session.ListQuery) (session.ListPage, error)
	SubscribeCatalogFn  func(context.Context) (<-chan session.CatalogEvent, func(), error)
	MetricsByAgentFn    func(context.Context, string) (map[string]session.AgentSessionMetrics, error)
	ListSessionsFn      func(context.Context, store.SessionListQuery) ([]store.SessionInfo, error)
	StatusFn            func(context.Context, string) (*session.Info, error)
	EventsFn            func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error)
	LatestEventFn       func(context.Context, string, string) (*store.SessionEvent, error)
	HistoryFn           func(context.Context, string, store.EventQuery) ([]store.TurnHistory, error)
	TranscriptPageFn    func(context.Context, string, transcript.PageQuery) (transcript.Page, error)
	TranscriptChangesFn func(context.Context, string, transcript.ChangeQuery) (transcript.ChangePage, error)
	RepairFn            func(context.Context, session.RepairOpts) (*session.RepairResult, error)
	DeleteFn            func(context.Context, string) error
	StopFn              func(context.Context, string) error
	StopWithCauseFn     func(context.Context, string, session.StopCause, string) error
	ResumeFn            func(context.Context, string) (*session.Session, error)
	AttachSessionFn     func(context.Context, store.SessionAttachRequest) (store.SessionAttach, error)
	ClearFn             func(context.Context, string) (*session.Session, error)
	PromptFn            func(context.Context, string, string) (<-chan acp.AgentEvent, error)
	PromptWithOptsFn    func(context.Context, string, session.PromptOpts) (<-chan acp.AgentEvent, error)
	PromptSyntheticFn   func(
		context.Context,
		string,
		session.SyntheticPromptOpts,
	) (<-chan acp.AgentEvent, error)
	SendPromptFn   func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error)
	InterruptFn    func(context.Context, string) (session.SendPromptResult, error)
	SteerFn        func(context.Context, string, string) (session.SendPromptResult, error)
	CancelQueuedFn func(context.Context, string, string) (session.SendPromptResult, error)
	CancelPromptFn func(context.Context, string) error
	ApproveFn      func(context.Context, string, acp.ApproveRequest) error
	InputQueueFn   func(context.Context, string) (session.InputQueueSummary, error)
}

func (s StubSessionManager) Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error) {
	if s.CreateFn != nil {
		return s.CreateFn(ctx, opts)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) CreateAccepted(
	ctx context.Context,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	if s.CreateAcceptedFn != nil {
		return s.CreateAcceptedFn(ctx, opts)
	}
	if strings.TrimSpace(opts.InitialPrompt) != "" {
		return nil, errors.New("testutil: CreateAcceptedFn is required for an initial prompt")
	}
	created, err := s.Create(ctx, opts.Session)
	if created == nil || err != nil {
		return nil, err
	}
	return created.Info(), nil
}

func (s StubSessionManager) List() []*session.Info {
	if s.ListFn != nil {
		return s.ListFn()
	}
	if s.ListAllFn != nil {
		infos, err := s.ListAllFn(context.Background())
		if err != nil {
			return []*session.Info{}
		}
		return infos
	}
	return nil
}

func (s StubSessionManager) ListAll(ctx context.Context) ([]*session.Info, error) {
	if s.ListAllFn != nil {
		return s.ListAllFn(ctx)
	}
	return nil, nil
}

func (s StubSessionManager) ListPage(ctx context.Context, query session.ListQuery) (session.ListPage, error) {
	if s.ListPageFn != nil {
		return s.ListPageFn(ctx, query)
	}
	infos, err := s.ListAll(ctx)
	if err != nil {
		return session.ListPage{}, err
	}
	return stubSessionListPage(infos, query), nil
}

func (s StubSessionManager) SubscribeSessionCatalogEvents(
	ctx context.Context,
) (<-chan session.CatalogEvent, func(), error) {
	if s.SubscribeCatalogFn != nil {
		return s.SubscribeCatalogFn(ctx)
	}
	events := make(chan session.CatalogEvent)
	close(events)
	return events, func() {}, nil
}

func (s StubSessionManager) AggregateSessionsByAgent(
	ctx context.Context,
	workspaceID string,
) (map[string]session.AgentSessionMetrics, error) {
	if s.MetricsByAgentFn != nil {
		return s.MetricsByAgentFn(ctx, workspaceID)
	}
	infos, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	metrics := make(map[string]session.AgentSessionMetrics)
	for _, info := range infos {
		if info == nil || info.WorkspaceID != workspaceID || info.Type == session.SessionTypeDream {
			continue
		}
		if info.Lineage != nil && session.IsInternalSpawnRole(info.Lineage.SpawnRole) {
			continue
		}
		agentName := aghconfig.NormalizeAgentName(info.AgentName)
		if agentName == "" {
			continue
		}
		metric := metrics[agentName]
		metric.Total++
		if info.State == session.StateActive {
			metric.Active++
		}
		if info.State == session.StateStopped &&
			(info.Failure != nil || info.StopReason == store.StopAgentCrashed ||
				info.StopReason == store.StopError) {
			metric.Failed++
		}
		if !info.CreatedAt.IsZero() && info.UpdatedAt.After(info.CreatedAt) {
			metric.RuntimeSeconds += int64(info.UpdatedAt.Sub(info.CreatedAt).Seconds())
		}
		if info.UpdatedAt.After(metric.LastActivityAt) {
			metric.LastActivityAt = info.UpdatedAt
		}
		metrics[agentName] = metric
	}
	return metrics, nil
}

func (s StubSessionManager) ListSessions(
	ctx context.Context,
	query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	if s.ListSessionsFn != nil {
		return s.ListSessionsFn(ctx, query)
	}
	if s.ListAllFn == nil {
		return nil, nil
	}
	infos, err := s.ListAllFn(ctx)
	if err != nil {
		return nil, err
	}
	storeInfos := make([]store.SessionInfo, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		if query.WorkspaceID != "" && info.WorkspaceID != query.WorkspaceID {
			continue
		}
		if query.State != "" && string(info.State) != query.State {
			continue
		}
		storeInfos = append(storeInfos, storeSessionInfoFromRuntime(info))
	}
	return storeInfos, nil
}

func (s StubSessionManager) Status(ctx context.Context, id string) (*session.Info, error) {
	if s.StatusFn != nil {
		return s.StatusFn(ctx, id)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) Events(
	ctx context.Context,
	id string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	if s.EventsFn != nil {
		return s.EventsFn(ctx, id, query)
	}
	return nil, nil
}

func (s StubSessionManager) LatestSessionEventByType(
	ctx context.Context,
	id string,
	eventType string,
) (*store.SessionEvent, error) {
	if s.LatestEventFn != nil {
		return s.LatestEventFn(ctx, id, eventType)
	}
	return nil, nil
}

func (s StubSessionManager) History(
	ctx context.Context,
	id string,
	query store.EventQuery,
) ([]store.TurnHistory, error) {
	if s.HistoryFn != nil {
		return s.HistoryFn(ctx, id, query)
	}
	return nil, nil
}

func (s StubSessionManager) TranscriptPage(
	ctx context.Context,
	id string,
	query transcript.PageQuery,
) (transcript.Page, error) {
	if s.TranscriptPageFn != nil {
		return s.TranscriptPageFn(ctx, id, query)
	}
	return transcript.Page{}, nil
}

func (s StubSessionManager) TranscriptChanges(
	ctx context.Context,
	id string,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	if s.TranscriptChangesFn != nil {
		return s.TranscriptChangesFn(ctx, id, query)
	}
	return transcript.ChangePage{}, nil
}

func (s StubSessionManager) RepairSession(
	ctx context.Context,
	opts session.RepairOpts,
) (*session.RepairResult, error) {
	if s.RepairFn != nil {
		return s.RepairFn(ctx, opts)
	}
	return &session.RepairResult{SessionID: opts.SessionID}, nil
}

func (s StubSessionManager) Delete(ctx context.Context, id string) error {
	if s.DeleteFn != nil {
		return s.DeleteFn(ctx, id)
	}
	return nil
}

func (s StubSessionManager) Stop(ctx context.Context, id string) error {
	if s.StopFn != nil {
		return s.StopFn(ctx, id)
	}
	return nil
}

func (s StubSessionManager) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	if s.StopWithCauseFn != nil {
		return s.StopWithCauseFn(ctx, id, cause, detail)
	}
	if s.StopFn != nil {
		return s.StopFn(ctx, id)
	}
	return nil
}

func (s StubSessionManager) Resume(ctx context.Context, id string) (*session.Session, error) {
	if s.ResumeFn != nil {
		return s.ResumeFn(ctx, id)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) AttachSession(
	ctx context.Context,
	req store.SessionAttachRequest,
) (store.SessionAttach, error) {
	if s.AttachSessionFn != nil {
		return s.AttachSessionFn(ctx, req)
	}
	return store.SessionAttach{}, store.ErrSessionNotFound
}

func (s StubSessionManager) ClearConversation(
	ctx context.Context,
	id string,
) (*session.Session, error) {
	if s.ClearFn != nil {
		return s.ClearFn(ctx, id)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	if s.PromptFn != nil {
		return s.PromptFn(ctx, id, msg)
	}
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (s StubSessionManager) PromptWithOpts(
	ctx context.Context,
	id string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	if s.PromptWithOptsFn != nil {
		return s.PromptWithOptsFn(ctx, id, opts)
	}
	return s.Prompt(ctx, id, opts.Message)
}

func (s StubSessionManager) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	if s.PromptSyntheticFn != nil {
		return s.PromptSyntheticFn(ctx, id, opts)
	}
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (s StubSessionManager) SendPrompt(
	ctx context.Context,
	id string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	if s.SendPromptFn != nil {
		return s.SendPromptFn(ctx, id, opts)
	}
	events, err := s.Prompt(ctx, id, opts.Message)
	if err != nil {
		return session.SendPromptResult{}, err
	}
	return session.SendPromptResult{Status: "accepted", Events: events}, nil
}

func (s StubSessionManager) InterruptPrompt(ctx context.Context, id string) (session.SendPromptResult, error) {
	if s.InterruptFn != nil {
		return s.InterruptFn(ctx, id)
	}
	if err := s.CancelPrompt(ctx, id); err != nil {
		return session.SendPromptResult{}, err
	}
	return session.SendPromptResult{Status: "interrupted", Interrupted: true}, nil
}

func (s StubSessionManager) SteerPrompt(
	ctx context.Context,
	id string,
	msg string,
) (session.SendPromptResult, error) {
	if s.SteerFn != nil {
		return s.SteerFn(ctx, id, msg)
	}
	return session.SendPromptResult{Status: "staged", Staged: true}, nil
}

func (s StubSessionManager) CancelQueuedPrompt(
	ctx context.Context,
	id string,
	queueEntryID string,
) (session.SendPromptResult, error) {
	if s.CancelQueuedFn != nil {
		return s.CancelQueuedFn(ctx, id, queueEntryID)
	}
	return session.SendPromptResult{Status: "canceled", QueueEntryID: queueEntryID}, nil
}

func (s StubSessionManager) CancelPrompt(ctx context.Context, id string) error {
	if s.CancelPromptFn != nil {
		return s.CancelPromptFn(ctx, id)
	}
	return nil
}

func (s StubSessionManager) ApprovePermission(ctx context.Context, id string, req acp.ApproveRequest) error {
	if s.ApproveFn != nil {
		return s.ApproveFn(ctx, id, req)
	}
	return nil
}

func (s StubSessionManager) InputQueueSummary(ctx context.Context, id string) (session.InputQueueSummary, error) {
	if s.InputQueueFn != nil {
		return s.InputQueueFn(ctx, id)
	}
	return session.InputQueueSummary{}, nil
}

var _ core.SessionManager = (*StubSessionManager)(nil)
var _ core.SessionCatalog = (*StubSessionManager)(nil)
var _ core.SessionCatalogEventSubscriber = (*StubSessionManager)(nil)
var _ core.AgentSessionMetricsReader = (*StubSessionManager)(nil)

func storeSessionInfoFromRuntime(info *session.Info) store.SessionInfo {
	storeInfo := store.SessionInfo{
		ID:          info.ID,
		Name:        info.Name,
		AgentName:   info.AgentName,
		Provider:    info.Provider,
		WorkspaceID: info.WorkspaceID,
		SessionNetworkState: &store.SessionNetworkState{
			NetworkSpec: info.NetworkParticipation,
		},
		SessionType:      string(info.Type),
		Lineage:          info.Lineage,
		State:            string(info.State),
		StopReason:       info.StopReason,
		StopDetail:       info.StopDetail,
		Failure:          info.Failure,
		Liveness:         info.Liveness,
		Sandbox:          info.Sandbox,
		SoulSnapshotID:   info.SoulSnapshotID,
		SoulDigest:       info.SoulDigest,
		ParentSoulDigest: info.ParentSoulDigest,
		AttachedTo:       info.AttachedTo,
		AttachExpiresAt:  info.AttachExpiresAt,
		TranscriptEpoch:  info.TranscriptEpoch,
		CreatedAt:        info.CreatedAt,
		UpdatedAt:        info.UpdatedAt,
	}
	if info.ACPSessionID != "" {
		acpSessionID := info.ACPSessionID
		storeInfo.ACPSessionID = &acpSessionID
	}
	return storeInfo
}
