package contract

type SessionInputStatus string

const (
	SessionInputQueued      SessionInputStatus = "queued"
	SessionInputDispatching SessionInputStatus = "dispatching"
	SessionInputSent        SessionInputStatus = "sent"
	SessionInputFailed      SessionInputStatus = "failed"
	SessionInputCanceled    SessionInputStatus = "canceled"
)

type SessionInputClearResponse struct {
	Inputs          []SessionInputPayload `json:"inputs"`
	ClearedCount    int                   `json:"cleared_count"`
	QueueGeneration int64                 `json:"queue_generation"`
}

func SessionInputStatusValues() []string {
	return []string{
		string(SessionInputQueued),
		string(SessionInputDispatching),
		string(SessionInputSent),
		string(SessionInputFailed),
		string(SessionInputCanceled),
	}
}
