package loop

import "context"

// ActionPromptOwner is the complete CAS identity for one managed Goal prompt.
type ActionPromptOwner struct {
	LoopRunID     RunID
	TaskRunID     string
	Generation    int
	NodeID        NodeID
	ItemIndex     int
	Turn          int
	PromptAttempt int
	ControlEpoch  int64
	BindingEpoch  int64
}

// ActionPromptTicket identifies one prepared, non-dispatchable managed prompt.
type ActionPromptTicket struct {
	QueueEntryID string
	PromptID     string
}

// ManagedActionSessionBinder exposes durable prepare/await/cancel semantics for Goal.
type ManagedActionSessionBinder interface {
	BindActionSession(context.Context, ActionSessionBindRequest) (ActionSessionBinding, error)
	AdvanceActionSessionRetry(context.Context, *ActionSessionRetryRequest) error
	PrepareActionPrompt(
		context.Context,
		ActionSessionBinding,
		ActionPromptRequest,
	) (ActionPromptTicket, error)
	AwaitActionPrompt(context.Context, ActionPromptTicket) (ActionPromptResult, error)
	CancelActionPrompts(context.Context, ActionPromptOwner) error
}
