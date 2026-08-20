package cmdpalette

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
)

const viewSessionFrameBuffer = 16

type viewSession struct {
	id                 string
	streamToken        string
	workspace          WorkspaceID
	client             ClientID
	view               string
	extension          string
	instanceGeneration uint64
	kind               ViewKind
	cancel             context.CancelFunc
	ctx                context.Context

	lastFrame       ViewFrame
	currentRevision string
	frameNumber     uint64
	handlers        map[string]uint64
	ackedEffects    map[string]struct{}
	subscribers     map[uint64]chan ViewFrame
	nextSubscriber  uint64

	lastSeq             int64
	nextGeneration      uint64
	rejectedGenerations map[uint64]struct{}
	coalescible         *viewEventFlight
	actions             map[uint64]viewEventFlight
	hardMisses          int
}

type viewEventFlight struct {
	seq        int64
	generation uint64
	cancel     context.CancelFunc
}

// SubscribeSessionFrames authorizes one SSE reader and returns a replay-safe frame.
func (s *Service) SubscribeSessionFrames(
	ctx context.Context,
	token SessionToken,
) (ViewFrame, <-chan ViewFrame, func(), error) {
	if ctx == nil {
		return ViewFrame{}, nil, nil, errors.New("cmd palette view: context is required")
	}
	s.viewSessionMu.Lock()
	session := s.viewSessions[strings.TrimSpace(token.ViewSession)]
	if session == nil {
		s.viewSessionMu.Unlock()
		return ViewFrame{}, nil, nil, ErrViewSessionGone
	}
	if token.StreamToken == "" || token.StreamToken != session.streamToken {
		s.viewSessionMu.Unlock()
		return ViewFrame{}, nil, nil, ErrViewSessionForbidden
	}
	session.nextSubscriber++
	id := session.nextSubscriber
	frames := make(chan ViewFrame, viewSessionFrameBuffer)
	session.subscribers[id] = frames
	replay := cloneViewFrame(session.lastFrame)
	s.viewSessionMu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.viewSessionMu.Lock()
			current := s.viewSessions[session.id]
			if current == session {
				if subscriber, exists := session.subscribers[id]; exists {
					delete(session.subscribers, id)
					close(subscriber)
				}
			}
			s.viewSessionMu.Unlock()
		})
	}
	return replay, frames, cancel, nil
}

// CloseSession tears down one session. A repeated close is successful.
func (s *Service) CloseSession(ctx context.Context, token SessionToken, reason string) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	session, err := s.authorizeViewSession(ctx, token, false)
	if err != nil {
		if errors.Is(err, ErrViewSessionGone) {
			return nil
		}
		return err
	}
	return s.removeViewSession(session.id, session, reason, true)
}

// CloseClientSessions tears down every programmable view owned by a detached client.
func (s *Service) CloseClientSessions(
	ctx context.Context,
	workspace WorkspaceID,
	client ClientID,
) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	s.viewSessionMu.Lock()
	closed := make([]*viewSession, 0)
	for id, session := range s.viewSessions {
		if session.workspace != workspace || session.client != client {
			continue
		}
		delete(s.viewSessions, id)
		cancelViewSessionLocked(session)
		closed = append(closed, session)
	}
	s.viewSessionMu.Unlock()

	var closeErr error
	for _, session := range closed {
		s.emitViewSessionEvent(ctx, EventViewSessionClosed, session)
		if err := s.closeViewProgram(ctx, session.workspace, session.extension, ViewCloseRequest{
			ViewSession: session.id,
			Reason:      "client_disconnected",
		}); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close session %q: %w", session.id, err))
		}
	}
	if closeErr != nil {
		return fmt.Errorf("cmd palette view: close detached client sessions: %w", closeErr)
	}
	return nil
}

// InvalidateInstance disposes sessions from every replaced process generation.
func (s *Service) InvalidateInstance(
	ctx context.Context,
	workspace WorkspaceID,
	extension string,
	generation uint64,
) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	extension = strings.TrimSpace(extension)
	s.viewSessionMu.Lock()
	invalidated := make([]*viewSession, 0)
	for id, session := range s.viewSessions {
		if session.workspace != workspace || session.extension != extension {
			continue
		}
		if generation != 0 && session.instanceGeneration == generation {
			continue
		}
		delete(s.viewSessions, id)
		cancelViewSessionLocked(session)
		invalidated = append(invalidated, session)
	}
	s.viewSessionMu.Unlock()
	for _, session := range invalidated {
		s.emitViewSessionEvent(ctx, EventViewSessionClosed, session)
		s.logger.Info(
			"cmd palette view session invalidated",
			"view_session", session.id,
			"extension", session.extension,
		)
	}
	return nil
}

func (s *Service) authorizeViewSession(
	ctx context.Context,
	token SessionToken,
	extensionOnly bool,
) (*viewSession, error) {
	id := strings.TrimSpace(token.ViewSession)
	s.viewSessionMu.Lock()
	session := s.viewSessions[id]
	s.viewSessionMu.Unlock()
	if session == nil {
		return nil, ErrViewSessionGone
	}
	if token.Extension != "" {
		if token.Extension != session.extension {
			return nil, ErrViewSessionForbidden
		}
		return session, nil
	}
	if extensionOnly || s.clients == nil || strings.TrimSpace(token.AttachmentToken) == "" {
		return nil, ErrViewSessionForbidden
	}
	if err := s.clients.Authorize(
		ctx,
		session.workspace,
		session.client,
		token.AttachmentToken,
	); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrViewSessionForbidden, err)
	}
	s.viewSessionMu.Lock()
	current := s.viewSessions[id]
	s.viewSessionMu.Unlock()
	if current != session {
		return nil, ErrViewSessionGone
	}
	return session, nil
}

func (s *Service) removeViewSession(
	id string,
	session *viewSession,
	reason string,
	notifyProgram bool,
) error {
	s.viewSessionMu.Lock()
	current := s.viewSessions[id]
	if current != session {
		s.viewSessionMu.Unlock()
		return nil
	}
	delete(s.viewSessions, id)
	cancelViewSessionLocked(session)
	s.viewSessionMu.Unlock()
	s.emitViewSessionEvent(session.ctx, EventViewSessionClosed, session)
	if !notifyProgram || s.viewPrograms == nil {
		return nil
	}
	if err := s.closeViewProgram(session.ctx, session.workspace, session.extension, ViewCloseRequest{
		ViewSession: id,
		Reason:      strings.TrimSpace(reason),
	}); err != nil {
		return fmt.Errorf("cmd palette view: close program session: %w", err)
	}
	return nil
}

func cancelViewSessionLocked(session *viewSession) {
	session.cancel()
	if session.coalescible != nil {
		session.coalescible.cancel()
		session.coalescible = nil
	}
	for generation, flight := range session.actions {
		flight.cancel()
		delete(session.actions, generation)
	}
	for id, subscriber := range session.subscribers {
		close(subscriber)
		delete(session.subscribers, id)
	}
}

func cloneViewArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(args))
	maps.Copy(cloned, args)
	return cloned
}
