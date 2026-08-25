package daemon

type nativeSessionTargetInput struct {
	SessionID string `json:"session_id"`
	Subtree   bool   `json:"subtree,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type nativeSessionWaitInput struct {
	SessionID string   `json:"session_id"`
	Until     []string `json:"until"`
	TimeoutMS int64    `json:"timeout_ms"`
}

type nativeSessionApproveInput struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
}

type nativeSessionClarifyAnswerInput struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Choice    *int   `json:"choice"`
	Text      string `json:"text"`
}
