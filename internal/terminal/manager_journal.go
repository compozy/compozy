package terminal

import "context"

type journalTerminalRegistrar interface {
	RegisterTerminal(Info, func(bool), func(TerminalEvent))
}

type journalCommandObserver interface {
	ReserveInput(Info, []byte) (int, bool)
	CommitInput(Info, Actor, []byte, int)
	ReleaseInput(Info, int)
	ObserveOutput(Info)
}

func (m *Service) registerJournalTerminal(item *session) {
	if m == nil || item == nil {
		return
	}
	registrar, ok := m.markers.(journalTerminalRegistrar)
	if !ok {
		return
	}
	registrar.RegisterTerminal(item.Info(), item.audit.SetBlocked, func(event TerminalEvent) {
		m.events.Emit(context.Background(), event)
	})
}

func (m *Service) reserveJournalInput(info Info, input []byte) (int, bool) {
	if observer, ok := m.markers.(journalCommandObserver); ok {
		return observer.ReserveInput(info, input)
	}
	return 0, true
}

func (m *Service) commitJournalInput(info Info, actor Actor, input []byte, reservation int) {
	if observer, ok := m.markers.(journalCommandObserver); ok {
		observer.CommitInput(info, actor, input, reservation)
	}
}

func (m *Service) releaseJournalInput(info Info, reservation int) {
	if observer, ok := m.markers.(journalCommandObserver); ok {
		observer.ReleaseInput(info, reservation)
	}
}

func (m *Service) observeJournalOutput(info Info) {
	if observer, ok := m.markers.(journalCommandObserver); ok {
		observer.ObserveOutput(info)
	}
}
