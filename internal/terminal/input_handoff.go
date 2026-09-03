package terminal

import "errors"

func (s *session) beginInputAnswerHandoff(actor Actor) (bool, error) {
	if err := s.lease.authorize(actor); err == nil {
		return false, nil
	}
	info := s.Info()
	if actor.Kind != ActorKindHuman || info.Controller == nil || info.Controller.Kind != ActorKindAgent {
		return false, &Error{
			Code:       ErrorCodeInputAnswerRequiresWrite,
			Message:    "input response requires the write lease or an agent-to-human handoff",
			Controller: info.Controller,
			Err:        ErrInputRequiresWrite,
		}
	}
	if err := s.lease.takeoverWithReason(actor, true, "answer_handoff"); err != nil {
		return false, err
	}
	return true, nil
}

func (s *session) finishInputAnswerHandoff(actor Actor, required bool) error {
	if !required {
		return nil
	}
	if err := s.lease.yieldWithReason(actor, "answer_return"); err != nil {
		return errors.Join(errors.New("terminal: return input answer lease"), err)
	}
	return nil
}
