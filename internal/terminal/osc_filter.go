package terminal

// outputFilter is the task_02 seam for the streaming OSC security parser.
// Task_01 owns its position before every byte consumer and starts with identity behavior.
type outputFilter interface {
	Filter(input []byte) FilterResult
}

type identityOutputFilter struct{}

func (identityOutputFilter) Filter(input []byte) FilterResult {
	return FilterResult{DisplayBytes: append([]byte(nil), input...)}
}
