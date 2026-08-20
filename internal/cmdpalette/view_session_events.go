package cmdpalette

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxViewActionFlights     = 4
	defaultViewHardAckBudget = 3 * time.Second
	viewSessionCircuitMisses = 3
)

// AdmitEvent applies effect acknowledgements, enforces admission caps, and
// starts the extension handler without tying its lifetime to the HTTP request.
func (s *Service) AdmitEvent(ctx context.Context, token SessionToken, event ViewEvent) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	session, err := s.authorizeViewSession(ctx, token, false)
	if err != nil {
		return err
	}
	if strings.TrimSpace(event.Handler) == "" || event.Seq <= 0 || strings.TrimSpace(event.Revision) == "" {
		return errors.New("cmd palette view: handler, positive seq, and revision are required")
	}
	if event.ViewSession != "" && event.ViewSession != session.id {
		return ErrViewSessionForbidden
	}

	s.viewSessionMu.Lock()
	current := s.viewSessions[session.id]
	if current != session {
		s.viewSessionMu.Unlock()
		return ErrViewSessionGone
	}
	if event.Seq <= session.lastSeq {
		s.viewSessionMu.Unlock()
		return errors.New("cmd palette view: event seq must increase")
	}
	if event.Revision != session.currentRevision {
		s.viewSessionMu.Unlock()
		return errors.New("cmd palette view: event revision is stale")
	}
	if !s.handlerAcceptsEventLocked(session, event.Handler) {
		session.lastSeq = event.Seq
		s.viewSessionMu.Unlock()
		return nil
	}
	s.ackEffectsLocked(session, event.AckEffects)
	session.lastSeq = event.Seq
	session.nextGeneration++
	event.Generation = session.nextGeneration
	event.ViewSession = session.id
	event.Args = append([]any(nil), event.Args...)
	coalescible := viewEventIsCoalescible(event)
	if !coalescible && len(session.actions) >= maxViewActionFlights {
		s.viewSessionMu.Unlock()
		return ErrViewBusy
	}
	flightCtx, cancel := context.WithCancel(session.ctx)
	flight := viewEventFlight{seq: event.Seq, generation: event.Generation, cancel: cancel}
	if coalescible {
		if session.coalescible != nil {
			session.coalescible.cancel()
			session.rejectedGenerations[session.coalescible.generation] = struct{}{}
		}
		session.coalescible = &flight
	} else {
		session.actions[event.Generation] = flight
	}
	extension := session.extension
	s.viewSessionMu.Unlock()

	go s.watchViewEventDeadline(flightCtx, session, event.Generation)
	go s.runViewEvent(flightCtx, session, extension, event, coalescible)
	return nil
}

func (s *Service) watchViewEventDeadline(
	ctx context.Context,
	session *viewSession,
	generation uint64,
) {
	budget := s.viewAckBudget
	if budget <= 0 {
		budget = defaultViewHardAckBudget
	}
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	s.viewSessionMu.Lock()
	if s.viewSessions[session.id] != session || !viewGenerationActiveLocked(session, generation) ||
		session.hardMisses >= viewSessionCircuitMisses {
		s.viewSessionMu.Unlock()
		return
	}
	session.hardMisses++
	name := EventViewSessionDegraded
	if session.hardMisses == viewSessionCircuitMisses {
		name = EventViewSessionCircuitBroken
	}
	s.viewSessionMu.Unlock()
	s.emitViewSessionEvent(session.ctx, name, session)
}

func viewGenerationActiveLocked(session *viewSession, generation uint64) bool {
	if session.coalescible != nil && session.coalescible.generation == generation {
		return true
	}
	_, exists := session.actions[generation]
	return exists
}

// PublishFrame validates extension ownership, causal freshness, revision order,
// and effect acknowledgement before fanning a frame out to session subscribers.
func (s *Service) PublishFrame(ctx context.Context, token SessionToken, frame ViewFrame) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	session, err := s.authorizeViewSession(ctx, token, true)
	if err != nil {
		return err
	}
	validated, err := validateViewFrame(session.kind, frame)
	if err != nil {
		return err
	}
	if validated.ViewSession != session.id {
		return ErrViewSessionForbidden
	}

	s.viewSessionMu.Lock()
	defer s.viewSessionMu.Unlock()
	if s.viewSessions[session.id] != session {
		return ErrViewSessionGone
	}
	if err := validateViewFrameCausalityLocked(session, validated); err != nil {
		return err
	}
	if validated.Patch != nil && validated.Patch.From != session.currentRevision {
		return fmt.Errorf(
			"cmd palette view: patch revision %q does not match %q",
			validated.Patch.From,
			session.currentRevision,
		)
	}
	s.acceptViewFrameLocked(session, validated)
	return nil
}

// AckEffects fences acknowledged effect ids from every replay path.
func (s *Service) AckEffects(ctx context.Context, token SessionToken, effectIDs []string) error {
	if ctx == nil {
		return errors.New("cmd palette view: context is required")
	}
	session, err := s.authorizeViewSession(ctx, token, false)
	if err != nil {
		return err
	}
	s.viewSessionMu.Lock()
	defer s.viewSessionMu.Unlock()
	if s.viewSessions[session.id] != session {
		return ErrViewSessionGone
	}
	s.ackEffectsLocked(session, effectIDs)
	return nil
}

func (s *Service) runViewEvent(
	ctx context.Context,
	session *viewSession,
	extension string,
	event ViewEvent,
	coalescible bool,
) {
	frame, err := s.viewPrograms.HandleProgramEvent(ctx, session.workspace, extension, event)
	if err == nil && frame != nil {
		if publishErr := s.PublishFrame(
			context.WithoutCancel(ctx),
			SessionToken{ViewSession: session.id, Extension: extension},
			*frame,
		); publishErr != nil && !errors.Is(publishErr, ErrViewFrameStale) &&
			!errors.Is(publishErr, ErrViewSessionGone) {
			s.logger.Warn(
				"cmd palette view frame rejected",
				"view_session", session.id,
				"extension", extension,
				"error", publishErr,
			)
		}
	} else if err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Warn(
			"cmd palette view event failed",
			"view_session", session.id,
			"extension", extension,
			"error", err,
		)
	}
	s.finishViewEvent(session, event.Generation, coalescible)
}

func (s *Service) finishViewEvent(session *viewSession, generation uint64, coalescible bool) {
	s.viewSessionMu.Lock()
	defer s.viewSessionMu.Unlock()
	if s.viewSessions[session.id] != session {
		return
	}
	if coalescible {
		if session.coalescible != nil && session.coalescible.generation == generation {
			session.coalescible.cancel()
			session.coalescible = nil
		}
		return
	}
	if flight, exists := session.actions[generation]; exists {
		flight.cancel()
		delete(session.actions, generation)
	}
}

func validateViewFrameCausalityLocked(session *viewSession, frame ViewFrame) error {
	if frame.Generation > session.nextGeneration ||
		(frame.Generation == 0 && session.nextGeneration > 0) {
		return ErrViewFrameStale
	}
	if _, rejected := session.rejectedGenerations[frame.Generation]; rejected {
		return ErrViewFrameStale
	}
	if frame.InReplyTo == 0 {
		return nil
	}
	if session.coalescible != nil &&
		session.coalescible.seq == frame.InReplyTo &&
		session.coalescible.generation == frame.Generation {
		return nil
	}
	if flight, exists := session.actions[frame.Generation]; exists && flight.seq == frame.InReplyTo {
		return nil
	}
	return ErrViewFrameStale
}

func (s *Service) acceptViewFrameLocked(session *viewSession, frame ViewFrame) {
	if frame.InReplyTo > 0 {
		session.hardMisses = 0
	}
	session.frameNumber++
	for _, handler := range frame.Handlers {
		session.handlers[handler] = session.frameNumber
	}
	for handler, lastSeen := range session.handlers {
		if session.frameNumber-lastSeen > 2 {
			delete(session.handlers, handler)
		}
	}
	frame.Effects = unackedViewEffects(frame.Effects, session.ackedEffects)
	session.currentRevision = frame.Revision
	session.lastFrame = cloneViewFrame(frame)
	for id, subscriber := range session.subscribers {
		select {
		case subscriber <- cloneViewFrame(frame):
		default:
			close(subscriber)
			delete(session.subscribers, id)
		}
	}
}

func (s *Service) handlerAcceptsEventLocked(session *viewSession, handler string) bool {
	lastSeen, exists := session.handlers[handler]
	if !exists {
		s.logger.Warn(
			"cmd palette view event references unknown handler",
			"view_session", session.id,
			"handler", handler,
		)
		return false
	}
	return session.frameNumber-lastSeen <= 2
}

func (s *Service) ackEffectsLocked(session *viewSession, effectIDs []string) {
	for _, id := range effectIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			session.ackedEffects[id] = struct{}{}
		}
	}
	session.lastFrame.Effects = unackedViewEffects(session.lastFrame.Effects, session.ackedEffects)
}

func unackedViewEffects(effects []Effect, acknowledged map[string]struct{}) []Effect {
	result := make([]Effect, 0, len(effects))
	for _, effect := range effects {
		if _, acked := acknowledged[effect.ID]; !acked {
			result = append(result, effect)
		}
	}
	return result
}

func viewEventIsCoalescible(event ViewEvent) bool {
	if len(event.Args) == 0 {
		return false
	}
	switch event.Args[len(event.Args)-1].(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float64:
		return true
	default:
		return false
	}
}
