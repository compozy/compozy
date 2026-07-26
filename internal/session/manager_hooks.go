package session

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

const (
	hookInputClassUserMessage    = acp.EventTypeUserMessage
	hookInputClassNetworkMessage = "network_message"
	hookInputClassSynthetic      = acp.EventTypeSyntheticReentry
	hookInputClassStartup        = "startup_prompt"

	hookMessageRoleAssistant = "assistant"

	hookMessageDeltaTypeFull    = "full"
	hookMessageDeltaTypeText    = "text"
	hookMessageDeltaTypeThought = "thought"
)

func newPromptTurnDispatchState(
	session *Session,
	turnID string,
	turnSource TurnSource,
	userMessage string,
) *promptTurnDispatchState {
	normalizedTurnSource := normalizeTurnSource(turnSource)
	if normalizedTurnSource == "" {
		normalizedTurnSource = TurnSourceUser
	}

	return &promptTurnDispatchState{
		session:     session,
		turnID:      strings.TrimSpace(turnID),
		turnSource:  normalizedTurnSource,
		inputClass:  inputClassForTurnSource(normalizedTurnSource),
		userMessage: userMessage,
	}
}

func inputClassForTurnSource(source TurnSource) string {
	switch normalizeTurnSource(source) {
	case TurnSourceNetwork:
		return hookInputClassNetworkMessage
	case TurnSourceSynthetic:
		return hookInputClassSynthetic
	default:
		return hookInputClassUserMessage
	}
}

func (m *Manager) dispatchSessionPreCreate(ctx context.Context, opts CreateOpts) (CreateOpts, error) {
	if m == nil {
		return opts, nil
	}

	payload, err := m.hooks.session().DispatchSessionPreCreate(ctx, hookspkg.SessionPreCreatePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPreCreate,
			Timestamp: m.now(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionName: strings.TrimSpace(opts.Name),
			SessionType: string(normalizeSessionType(opts.Type)),
			AgentName:   strings.TrimSpace(opts.AgentName),
			WorkspaceID: strings.TrimSpace(opts.Workspace),
			Workspace:   strings.TrimSpace(opts.WorkspacePath),
			State:       string(StateStarting),
		},
	})
	if err != nil {
		return CreateOpts{}, fmt.Errorf("session: dispatch session.pre_create: %w", err)
	}

	next := opts
	next.Name = strings.TrimSpace(payload.SessionName)
	next.Type = normalizeSessionType(Type(strings.TrimSpace(payload.SessionType)))
	next.AgentName = strings.TrimSpace(payload.AgentName)

	workspaceID := strings.TrimSpace(payload.WorkspaceID)
	workspacePath := strings.TrimSpace(payload.Workspace)
	switch {
	case workspaceID != "" && workspacePath != "":
		return CreateOpts{}, errors.New("session: session.pre_create produced both workspace id and workspace path")
	case workspaceID != "":
		next.Workspace = workspaceID
		next.WorkspacePath = ""
	case workspacePath != "":
		next.Workspace = ""
		next.WorkspacePath = workspacePath
	default:
		next.Workspace = ""
		next.WorkspacePath = ""
	}

	return next, nil
}

func (m *Manager) dispatchSessionPreResume(ctx context.Context, meta store.SessionMeta) (store.SessionMeta, error) {
	if m == nil {
		return meta, nil
	}

	payload, err := m.hooks.session().DispatchSessionPreResume(ctx, hookspkg.SessionPreResumePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPreResume,
			Timestamp: m.now(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID:    strings.TrimSpace(meta.ID),
			SessionName:  strings.TrimSpace(meta.Name),
			SessionType:  string(normalizeSessionType(Type(meta.SessionType))),
			AgentName:    strings.TrimSpace(meta.AgentName),
			WorkspaceID:  strings.TrimSpace(meta.WorkspaceID),
			ACPSessionID: strings.TrimSpace(derefString(meta.ACPSessionID)),
			State:        strings.TrimSpace(meta.State),
			CreatedAt:    meta.CreatedAt,
			UpdatedAt:    meta.UpdatedAt,
		},
	})
	if err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: dispatch session.pre_resume: %w", err)
	}

	next := meta
	next.Name = strings.TrimSpace(payload.SessionName)
	next.AgentName = strings.TrimSpace(payload.AgentName)
	next.WorkspaceID = strings.TrimSpace(payload.WorkspaceID)
	next.SessionType = string(normalizeSessionType(Type(strings.TrimSpace(payload.SessionType))))
	return next, nil
}

func (m *Manager) dispatchSessionPostCreate(ctx context.Context, session *Session) {
	m.dispatchSessionLifecycleObservation(ctx, session, hookspkg.HookSessionPostCreate)
}

func (m *Manager) dispatchSessionPostResume(ctx context.Context, session *Session) {
	m.dispatchSessionLifecycleObservation(ctx, session, hookspkg.HookSessionPostResume)
}

func (m *Manager) dispatchSessionPreStop(ctx context.Context, session *Session) error {
	if m == nil || session == nil {
		return nil
	}
	ctx = hookDispatchContext(ctx, m, session)

	payload, err := m.hooks.session().
		DispatchSessionPreStop(ctx, hookSessionLifecyclePayload(session, hookspkg.HookSessionPreStop, m.now()))
	if err != nil {
		return fmt.Errorf("session: dispatch session.pre_stop: %w", err)
	}

	session.applyHookSessionContext(payload.SessionContext, m.now())
	return nil
}

func (m *Manager) dispatchSessionPostStop(ctx context.Context, session *Session) {
	m.dispatchSessionLifecycleObservation(ctx, session, hookspkg.HookSessionPostStop)
}

func (m *Manager) dispatchSessionLifecycleObservation(ctx context.Context, session *Session, event hookspkg.HookEvent) {
	if m == nil || session == nil {
		return
	}
	ctx = m.postLifecycleHookContext(ctx, session)

	payload := hookSessionLifecyclePayload(session, event, m.now())
	var err error
	lifecycleHooks := m.hooks.session()
	switch event {
	case hookspkg.HookSessionPostCreate:
		_, err = lifecycleHooks.DispatchSessionPostCreate(ctx, payload)
	case hookspkg.HookSessionPostResume:
		_, err = lifecycleHooks.DispatchSessionPostResume(ctx, payload)
	case hookspkg.HookSessionPostStop:
		_, err = lifecycleHooks.DispatchSessionPostStop(ctx, payload)
	default:
		return
	}
	if err != nil {
		m.warnHookDispatch(ctx, session, event, err)
	}
}

func (m *Manager) hookLifecycleContext(ctx context.Context) context.Context {
	if ctx == nil {
		return m.fallbackLifecycleContext()
	}
	return ctx
}

func (m *Manager) fallbackLifecycleContext() context.Context {
	if m != nil && m.lifecycleCtx != nil {
		return m.lifecycleCtx
	}
	return context.Background()
}

func (m *Manager) postLifecycleHookContext(ctx context.Context, session *Session) context.Context {
	if ctx == nil {
		return hookDispatchContext(m.fallbackLifecycleContext(), m, session)
	}
	ctx = context.WithoutCancel(ctx)
	return hookDispatchContext(ctx, m, session)
}
