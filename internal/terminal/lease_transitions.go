package terminal

func leaseClosedError() error {
	return &Error{Code: ErrorCodeExited, Message: errorMessageExited, Err: ErrExited}
}

type leaseTransition func(from, to LeaseState, reason string, actor Actor, controller *Actor)

type leaseTransitionEvent struct {
	from, to   LeaseState
	reason     string
	actor      Actor
	controller *Actor
}

func (m *leaseMachine) close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.transitionWG.Wait()
		return
	}
	m.closed = true
	m.cancelGraceLocked()
	m.attachments = make(map[uint64]Actor)
	queued := len(m.transitions)
	clear(m.transitions)
	m.transitions = nil
	for range queued {
		m.transitionWG.Done()
	}
	m.mu.Unlock()
	m.transitionWG.Wait()
}

func (m *leaseMachine) queueTransitionLocked(from, to LeaseState, reason string, actor Actor) bool {
	if m.closed || m.onTransition == nil {
		return false
	}
	m.transitionWG.Add(1)
	m.transitions = append(m.transitions, leaseTransitionEvent{
		from: from, to: to, reason: reason, actor: actor, controller: cloneActor(m.controller),
	})
	if m.publishing {
		return false
	}
	m.publishing = true
	return true
}

func (m *leaseMachine) publishTransitions() {
	for {
		m.mu.Lock()
		if len(m.transitions) == 0 {
			m.publishing = false
			m.mu.Unlock()
			return
		}
		event := m.transitions[0]
		m.transitions[0] = leaseTransitionEvent{}
		m.transitions = m.transitions[1:]
		m.mu.Unlock()
		m.onTransition(event.from, event.to, event.reason, event.actor, cloneActor(event.controller))
		m.transitionWG.Done()
	}
}
