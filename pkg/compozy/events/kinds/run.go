package kinds

// RunQueuedPayload describes a queued run.
type RunQueuedPayload struct {
	Mode            string `json:"mode,omitempty"`
	Name            string `json:"name,omitempty"`
	WorkspaceRoot   string `json:"workspace_root,omitempty"`
	IDE             string `json:"ide,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	AccessMode      string `json:"access_mode,omitempty"`
}

// RunStartedPayload describes a started run.
type RunStartedPayload struct {
	Mode            string `json:"mode,omitempty"`
	Name            string `json:"name,omitempty"`
	WorkspaceRoot   string `json:"workspace_root,omitempty"`
	IDE             string `json:"ide,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	AccessMode      string `json:"access_mode,omitempty"`
	ArtifactsDir    string `json:"artifacts_dir,omitempty"`
	JobsTotal       int    `json:"jobs_total,omitempty"`
}

// RunCompletedPayload describes a completed run.
type RunCompletedPayload struct {
	ArtifactsDir   string `json:"artifacts_dir,omitempty"`
	JobsTotal      int    `json:"jobs_total,omitempty"`
	JobsSucceeded  int    `json:"jobs_succeeded,omitempty"`
	JobsFailed     int    `json:"jobs_failed,omitempty"`
	JobsCancelled  int    `json:"jobs_canceled,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
	ResultPath     string `json:"result_path,omitempty"`
	SummaryMessage string `json:"summary_message,omitempty"`
}

// RunFailedPayload describes a failed run.
type RunFailedPayload struct {
	ArtifactsDir string `json:"artifacts_dir,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	Error        string `json:"error,omitempty"`
	ResultPath   string `json:"result_path,omitempty"`
}

// RunCancelledPayload describes a canceled run.
type RunCancelledPayload struct {
	Reason      string `json:"reason,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
}
