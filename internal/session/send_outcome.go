package session

import "github.com/compozy/compozy/internal/store"

type Disposition string

const (
	DispositionDirect       Disposition = "direct"
	DispositionSteering     Disposition = "steering"
	DispositionQueued       Disposition = "queued"
	DispositionInterrupting Disposition = "interrupting"
)

func DispositionValues() []string {
	return []string{
		string(DispositionDirect),
		string(DispositionSteering),
		string(DispositionQueued),
		string(DispositionInterrupting),
	}
}

// SendOutcome is the immutable acceptance shared by all prompt transports.
type SendOutcome struct {
	Disposition    Disposition
	Delivery       store.SteerDeliveryMode
	TurnID         string
	EntryID        string
	MessageID      string
	IdempotencyKey string
	QueuePosition  int
	Replayed       bool
}

func (r SendPromptResult) Outcome() SendOutcome {
	outcome := SendOutcome{
		Delivery: r.SteerDelivery, TurnID: r.PreviousTurnID, EntryID: r.QueueEntryID,
		MessageID: r.MessageID, IdempotencyKey: r.IdempotencyKey,
		QueuePosition: r.QueuePosition, Replayed: r.Replayed,
	}
	if outcome.TurnID == "" {
		outcome.TurnID = r.NewTurnID
	}
	switch r.Status {
	case store.SessionPromptResultStatusAccepted:
		outcome.Disposition = DispositionDirect
	case store.SessionPromptResultStatusSteering:
		outcome.Disposition = DispositionSteering
	case store.SessionPromptResultStatusQueued:
		outcome.Disposition = DispositionQueued
	case store.SessionPromptResultStatusInterrupting:
		outcome.Disposition = DispositionInterrupting
	}
	return outcome
}
