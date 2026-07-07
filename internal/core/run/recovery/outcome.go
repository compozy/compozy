// Package recovery defines run recovery contracts and helpers.
package recovery

// ResultSchemaVersion is the supported persisted result.json schema version.
const ResultSchemaVersion = 1

// TimeoutExitCode is the executor exit code used for activity timeouts.
const TimeoutExitCode = -2

// RunStatus mirrors the persisted executor run status strings.
type RunStatus string

const (
	// StatusSucceeded marks a run or job that completed successfully.
	StatusSucceeded RunStatus = "succeeded"
	// StatusFailed marks a run or job that failed.
	StatusFailed RunStatus = "failed"
	// StatusCanceled marks a run or job that was canceled.
	StatusCanceled RunStatus = "canceled"
	// StatusParked marks a job that stalled again after its clean-state retry and
	// was parked for triage. It is terminal and distinct from failed and canceled.
	StatusParked RunStatus = "parked"
	// StatusUnknown marks a job whose terminal state was unavailable.
	StatusUnknown RunStatus = "unknown"
)

// RunOutcome is the typed recovery view of one persisted run result.
type RunOutcome struct {
	RunID        string       `json:"run_id"`
	Status       RunStatus    `json:"status"`
	ArtifactsDir string       `json:"artifacts_dir"`
	ResultPath   string       `json:"result_path,omitempty"`
	Jobs         []JobOutcome `json:"jobs"`
}

// JobOutcome is the typed recovery view of one persisted job result.
type JobOutcome struct {
	SafeName string    `json:"safe_name"`
	Status   RunStatus `json:"status"`
	ExitCode int       `json:"exit_code"`
	OutLog   string    `json:"stdout_log_path,omitempty"`
	ErrLog   string    `json:"stderr_log_path,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// FailedJobIDs returns stable job IDs for jobs that failed.
func (o RunOutcome) FailedJobIDs() []string {
	failed := make([]string, 0)
	for _, job := range o.Jobs {
		if job.Status != StatusFailed || job.SafeName == "" {
			continue
		}
		failed = append(failed, job.SafeName)
	}
	return failed
}

// Canceled reports whether the run or any job was canceled.
func (o RunOutcome) Canceled() bool {
	if o.Status == StatusCanceled {
		return true
	}
	for _, job := range o.Jobs {
		if job.Status == StatusCanceled {
			return true
		}
	}
	return false
}

// TimedOut reports whether any job failed with the timeout exit code.
func (o RunOutcome) TimedOut() bool {
	return len(o.TimeoutJobIDs()) > 0
}

// TimeoutJobIDs returns stable job IDs for jobs that timed out.
func (o RunOutcome) TimeoutJobIDs() []string {
	timedOut := make([]string, 0)
	for _, job := range o.Jobs {
		if !job.TimedOut() || job.SafeName == "" {
			continue
		}
		timedOut = append(timedOut, job.SafeName)
	}
	return timedOut
}

// TimedOut reports whether the job failed with the timeout exit code.
func (j JobOutcome) TimedOut() bool {
	return j.ExitCode == TimeoutExitCode
}
