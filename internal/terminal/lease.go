package terminal

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const defaultControllerGrace = 30 * time.Second

type PartialWriteError struct {
	Delivered int
	Err       error
}

func (e *PartialWriteError) Error() string {
	return fmt.Sprintf("terminal input stopped after %d bytes: %v", e.Delivered, e.Err)
}

func (e *PartialWriteError) Unwrap() error { return e.Err }

type leaseTransition func(from, to LeaseState, reason string, actor Actor)

type leaseMachine struct {
	mu           sync.Mutex
	state        LeaseState
	controller   *Actor
	fallback     Actor
	writer       io.Writer
	grace        time.Duration
	generation   uint64
	attachments  map[uint64]Actor
	nextAttach   uint64
	timer        *time.Timer
	onTransition leaseTransition
}

func newLeaseMachine(initial Actor, writer io.Writer, grace time.Duration, onTransition leaseTransition) *leaseMachine {
	if grace <= 0 {
		grace = defaultControllerGrace
	}
	machine := &leaseMachine{
		state:        LeaseAvailable,
		writer:       writer,
		grace:        grace,
		attachments:  make(map[uint64]Actor),
		onTransition: onTransition,
	}
	if initial.Kind == ActorKindHuman {
		machine.fallback = initial
		machine.state = LeaseHumanOwned
		machine.controller = cloneActor(&initial)
	} else if initial.Kind == ActorKindAgent {
		machine.fallback = Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: initial.ProfileID}
		machine.state = LeaseAgentOwned
		machine.controller = cloneActor(&initial)
	}
	return machine
}

func (m *leaseMachine) snapshot() (LeaseState, *Actor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state, cloneActor(m.controller)
}

func (m *leaseMachine) deliver(actor Actor, input []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.authorizeLocked(actor); err != nil {
		return err
	}
	delivered := 0
	for delivered < len(input) {
		written, err := m.writer.Write(input[delivered:])
		delivered += written
		if err != nil {
			return &PartialWriteError{Delivered: delivered, Err: fmt.Errorf("terminal: write input: %w", err)}
		}
		if written == 0 {
			return &PartialWriteError{Delivered: delivered, Err: io.ErrNoProgress}
		}
	}
	return nil
}

func (m *leaseMachine) authorize(actor Actor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.authorizeLocked(actor)
}

func (m *leaseMachine) authorizeLocked(actor Actor) error {
	if m.controller == nil {
		return &Error{Code: "write_owner_held", Message: "terminal has no active controller", Err: ErrWriteOwnerHeld}
	}
	if actor.Kind == ActorKindAgent && sameRun(actor, *m.controller) && actor.Generation != m.controller.Generation {
		return &Error{Code: "generation_fenced", Message: "terminal action came from a stale runtime generation", Err: ErrGenerationFenced}
	}
	if !sameActor(actor, *m.controller) {
		sentinel := ErrWriteOwnerHeld
		code := "write_owner_held"
		if actor.Kind == ActorKindAgent {
			sentinel = ErrLeaseRevoked
			code = "lease_revoked"
		}
		return &Error{Code: code, Message: "terminal input rejected because another actor controls it", Controller: cloneActor(m.controller), Err: sentinel}
	}
	return nil
}

func (m *leaseMachine) takeover(actor Actor, force bool) error {
	m.mu.Lock()
	if m.controller != nil && actor.Kind == ActorKindAgent && sameRun(actor, *m.controller) &&
		actor.Generation != m.controller.Generation {
		m.mu.Unlock()
		return &Error{Code: "generation_fenced", Message: "terminal action came from a stale runtime generation", Err: ErrGenerationFenced}
	}
	if m.controller != nil && sameActor(actor, *m.controller) {
		m.cancelGraceLocked()
		m.mu.Unlock()
		return nil
	}
	if actor.Kind != ActorKindHuman {
		controller := cloneActor(m.controller)
		m.mu.Unlock()
		return &Error{Code: "write_owner_held", Message: "agents cannot displace the current controller", Controller: controller, Err: ErrWriteOwnerHeld}
	}
	if m.controller != nil && m.controller.Kind == ActorKindHuman {
		controller := cloneActor(m.controller)
		m.mu.Unlock()
		return &Error{Code: "write_owner_held", Message: "another human controls the terminal", Controller: controller, Err: ErrWriteOwnerHeld}
	}
	if m.controller != nil && !force {
		controller := cloneActor(m.controller)
		m.mu.Unlock()
		return &Error{Code: "write_owner_held", Message: "terminal takeover requires force", Controller: controller, Err: ErrWriteOwnerHeld}
	}
	from := m.state
	m.controller = cloneActor(&actor)
	m.fallback = actor
	m.state = LeaseHumanOwned
	m.generation++
	m.cancelGraceLocked()
	m.mu.Unlock()
	m.emit(from, LeaseHumanOwned, "takeover", actor)
	return nil
}

func (m *leaseMachine) yield(actor Actor) error {
	m.mu.Lock()
	if err := m.authorizeLocked(actor); err != nil {
		m.mu.Unlock()
		return err
	}
	if actor.Kind == ActorKindHuman {
		m.mu.Unlock()
		return nil
	}
	from := m.state
	m.controller = cloneActor(&m.fallback)
	m.state = LeaseHumanOwned
	m.generation++
	m.cancelGraceLocked()
	m.mu.Unlock()
	m.emit(from, LeaseHumanOwned, "yield", actor)
	return nil
}

func (m *leaseMachine) runEnded(actor Actor, reason string) {
	m.mu.Lock()
	if m.controller == nil || m.controller.Kind != ActorKindAgent || !sameActor(actor, *m.controller) {
		m.mu.Unlock()
		return
	}
	from := m.state
	m.controller = cloneActor(&m.fallback)
	m.state = LeaseHumanOwned
	m.generation++
	m.cancelGraceLocked()
	m.mu.Unlock()
	m.emit(from, LeaseHumanOwned, reason, actor)
}

func (m *leaseMachine) attachWriter(actor Actor) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextAttach++
	m.attachments[m.nextAttach] = actor
	if m.controller != nil && sameActor(actor, *m.controller) {
		m.cancelGraceLocked()
	}
	return m.nextAttach
}

func (m *leaseMachine) detachWriter(id uint64) {
	m.mu.Lock()
	actor, ok := m.attachments[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.attachments, id)
	if m.controller == nil || !sameActor(actor, *m.controller) || m.controllerAttachmentCountLocked() > 0 {
		m.mu.Unlock()
		return
	}
	m.generation++
	generation := m.generation
	m.timer = time.AfterFunc(m.grace, func() { m.expireGrace(generation, actor) })
	m.mu.Unlock()
}

func (m *leaseMachine) expireGrace(generation uint64, actor Actor) {
	m.mu.Lock()
	if generation != m.generation || m.controller == nil || !sameActor(actor, *m.controller) || m.controllerAttachmentCountLocked() > 0 {
		m.mu.Unlock()
		return
	}
	from := m.state
	m.controller = nil
	m.state = LeaseAvailable
	m.timer = nil
	m.generation++
	m.mu.Unlock()
	m.emit(from, LeaseAvailable, "grace_expired", actor)
}

func (m *leaseMachine) controllerAttachmentCountLocked() int {
	if m.controller == nil {
		return 0
	}
	count := 0
	for _, actor := range m.attachments {
		if sameActor(actor, *m.controller) {
			count++
		}
	}
	return count
}

func (m *leaseMachine) cancelGraceLocked() {
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	m.generation++
}

func (m *leaseMachine) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelGraceLocked()
	m.attachments = make(map[uint64]Actor)
}

func (m *leaseMachine) emit(from, to LeaseState, reason string, actor Actor) {
	if m.onTransition != nil && from != to {
		m.onTransition(from, to, reason, actor)
	}
}

func sameActor(left, right Actor) bool {
	return left.Kind == right.Kind && left.ID == right.ID && left.ProfileID == right.ProfileID &&
		left.SessionID == right.SessionID && left.RunID == right.RunID && left.Generation == right.Generation
}

func sameRun(left, right Actor) bool {
	return left.Kind == ActorKindAgent && right.Kind == ActorKindAgent && left.ProfileID == right.ProfileID &&
		left.SessionID == right.SessionID && left.RunID == right.RunID
}

func cloneActor(actor *Actor) *Actor {
	if actor == nil {
		return nil
	}
	copyOfActor := *actor
	return &copyOfActor
}

func isLeaseError(err error) bool {
	return errors.Is(err, ErrGenerationFenced) || errors.Is(err, ErrLeaseRevoked) || errors.Is(err, ErrWriteOwnerHeld)
}
