package model

type FetchConfig struct {
	ReviewsDir      string `json:"reviews_dir,omitempty"`
	IncludeResolved bool   `json:"include_resolved,omitempty"`
}

type FixOutcome struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type JobResult struct {
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Attempts   int    `json:"attempts,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

type RunSummary struct {
	Status        string `json:"status"`
	JobsTotal     int    `json:"jobs_total"`
	JobsSucceeded int    `json:"jobs_succeeded,omitempty"`
	JobsFailed    int    `json:"jobs_failed,omitempty"`
	JobsCanceled  int    `json:"jobs_canceled,omitempty"`
	Error         string `json:"error,omitempty"`
	TeardownError string `json:"teardown_error,omitempty"`
}
