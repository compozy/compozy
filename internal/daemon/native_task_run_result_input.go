package daemon

type taskRunResultInput struct {
	RunID  string `json:"run_id"`
	Offset int64  `json:"offset,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
}
