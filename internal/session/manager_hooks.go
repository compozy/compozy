package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
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

func newPromptRequestDispatchState(
	session *Session,
	req promptRequest,
	message string,
) *promptTurnDispatchState {
	state := newPromptTurnDispatchState(session, req.turnID, req.turnSource, message)
	state.runID = strings.TrimSpace(req.runID)
	return state
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

	request := hookspkg.SessionPreCreatePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPreCreate,
			Timestamp: m.now(),
		},
		SessionContext: hookspkg.SessionContext{
			ProfileID:   normalizeCreateProfileID(opts.ProfileID),
			SessionName: strings.TrimSpace(opts.Name),
			SessionType: string(normalizeSessionType(opts.Type)),
			AgentName:   strings.TrimSpace(opts.AgentName),
			WorkspaceID: strings.TrimSpace(opts.Workspace),
			Workspace:   strings.TrimSpace(opts.WorkspacePath),
			State:       string(StateStarting),
		},
	}
	payload, err := m.hooks.session().DispatchSessionPreCreate(ctx, request)
	if err != nil {
		return CreateOpts{}, fmt.Errorf("session: dispatch session.pre_create: %w", err)
	}
	if err := validateProtectedPreCreateSessionType(request.SessionContext, payload.SessionContext); err != nil {
		return CreateOpts{}, fmt.Errorf("session: validate session.pre_create patch: %w", err)
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

	request := hookspkg.SessionPreResumePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPreResume,
			Timestamp: m.now(),
		},
		SessionContext: hookspkg.SessionContext{
			ProfileID:   strings.TrimSpace(meta.ProfileID),
			SessionID:   strings.TrimSpace(meta.ID),
			SessionName: strings.TrimSpace(meta.Name),
			SessionType: string(normalizeSessionType(Type(meta.SessionType))),
			AgentName:   strings.TrimSpace(meta.AgentName),
			WorkspaceID: strings.TrimSpace(meta.WorkspaceID),
			SessionRuntimeContext: hookspkg.NewSessionRuntimeContext(
				meta.WorktreeIDValue(), derefString(meta.ACPSessionID),
			),
			State:     strings.TrimSpace(meta.State),
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
		},
	}
	payload, err := m.hooks.session().DispatchSessionPreResume(ctx, request)
	if err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: dispatch session.pre_resume: %w", err)
	}
	if err := validateImmutableSessionWorkspace(request.SessionContext, payload.SessionContext); err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: validate session.pre_resume patch: %w", err)
	}
	if err := validateImmutableSessionType(request.SessionContext, payload.SessionContext); err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: validate session.pre_resume patch: %w", err)
	}

	next := meta
	next.Name = strings.TrimSpace(payload.SessionName)
	next.AgentName = strings.TrimSpace(payload.AgentName)
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

	request := hookSessionLifecyclePayload(session, hookspkg.HookSessionPreStop, m.now())
	payload, err := m.hooks.session().DispatchSessionPreStop(ctx, request)
	if err != nil {
		return fmt.Errorf("session: dispatch session.pre_stop: %w", err)
	}
	if err := validateImmutableSessionWorkspace(request.SessionContext, payload.SessionContext); err != nil {
		return fmt.Errorf("session: validate session.pre_stop patch: %w", err)
	}
	if err := validateImmutableSessionType(request.SessionContext, payload.SessionContext); err != nil {
		return fmt.Errorf("session: validate session.pre_stop patch: %w", err)
	}

	session.applyHookSessionContext(payload.SessionContext, m.now())
	return nil
}

func (m *Manager) dispatchSessionPostStop(ctx context.Context, session *Session) {
	if session == nil {
		return
	}
	session.postStopOnce.Do(func() {
		m.dispatchSessionLifecycleObservation(ctx, session, hookspkg.HookSessionPostStop)
	})
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
	return m.lifecycleCtx
}

func (m *Manager) postLifecycleHookContext(ctx context.Context, session *Session) context.Context {
	if ctx == nil {
		return hookDispatchContext(m.fallbackLifecycleContext(), m, session)
	}
	ctx = context.WithoutCancel(ctx)
	return hookDispatchContext(ctx, m, session)
}
