package terminal

import "context"

func (m *Service) closeJournalTerminal(ctx context.Context, item *session) error {
	return m.journal.CloseTerminal(ctx, item.Info())
}

func (m *Service) registerJournalTerminal(item *session) {
	if m == nil || item == nil {
		return
	}
	m.journal.RegisterTerminal(item.Info(), item.audit.SetBlocked, func(event Event) {
		m.events.Notify(item.ctx, event)
	})
}

func (m *Service) reserveJournalInput(info Info, input JournalInput) (JournalInputReservation, bool) {
	return m.journal.ReserveInput(info, input)
}

func (m *Service) observeJournalOutput(info Info, output []byte) {
	m.journal.ObserveOutput(info, output)
}
