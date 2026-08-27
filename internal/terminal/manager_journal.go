package terminal

import "context"

type journalTerminalRegistrar interface {
	RegisterTerminal(context.Context, Info, func(bool), func(Event))
}

type journalTerminalCloser interface {
	CloseTerminal(context.Context, Info) error
}

type journalCommandObserver interface {
	ReserveInput(Info, []byte) (int, bool)
	CommitInput(Info, Actor, []byte, int)
	ReleaseInput(Info, int)
	ObserveOutput(Info)
}

func (m *Service) closeJournalTerminal(ctx context.Context, item *session) error {
	closer, ok := m.markers.(journalTerminalCloser)
	if !ok {
		return nil
	}
	return closer.CloseTerminal(ctx, item.Info())
}

func (m *Service) registerJournalTerminal(item *session) {
	if m == nil || item == nil {
		return
	}
	registrar, ok := m.markers.(journalTerminalRegistrar)
	if !ok {
		return
	}
	registrar.RegisterTerminal(item.ctx, item.Info(), item.audit.SetBlocked, func(event Event) {
		m.events.Notify(item.ctx, event)
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
