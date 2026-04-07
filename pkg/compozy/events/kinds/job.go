package kinds

// JobQueuedPayload describes a queued job.
type JobQueuedPayload struct {
	Index     int      `json:"index"`
	CodeFile  string   `json:"code_file,omitempty"`
	CodeFiles []string `json:"code_files,omitempty"`
	Issues    int      `json:"issues,omitempty"`
	TaskTitle string   `json:"task_title,omitempty"`
	TaskType  string   `json:"task_type,omitempty"`
	SafeName  string   `json:"safe_name,omitempty"`
	OutLog    string   `json:"out_log,omitempty"`
	ErrLog    string   `json:"err_log,omitempty"`
}

// JobStartedPayload describes a started job.
type JobStartedPayload struct {
	JobAttemptInfo
}

// JobAttemptInfo carries shared attempt counters for job lifecycle payloads.
type JobAttemptInfo struct {
	Index       int `json:"index"`
	Attempt     int `json:"attempt,omitempty"`
	MaxAttempts int `json:"max_attempts,omitempty"`
}

// JobAttemptStartedPayload describes the start of one job attempt.
type JobAttemptStartedPayload struct {
	JobAttemptInfo
}

// JobAttemptFinishedPayload describes the end of one job attempt.
type JobAttemptFinishedPayload struct {
	JobAttemptInfo
	Status    string `json:"status,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
	Error     string `json:"error,omitempty"`
}

// JobRetryScheduledPayload describes a retry decision.
type JobRetryScheduledPayload struct {
	JobAttemptInfo
	Reason string `json:"reason,omitempty"`
}

// JobCompletedPayload describes a completed job.
type JobCompletedPayload struct {
	JobAttemptInfo
	ExitCode   int   `json:"exit_code,omitempty"`
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// JobFailedPayload describes a failed job.
type JobFailedPayload struct {
	JobAttemptInfo
	CodeFile string `json:"code_file,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	OutLog   string `json:"out_log,omitempty"`
	ErrLog   string `json:"err_log,omitempty"`
	Error    string `json:"error,omitempty"`
}

// JobCancelledPayload describes a canceled job.
type JobCancelledPayload struct {
	JobAttemptInfo
	Reason string `json:"reason,omitempty"`
}
