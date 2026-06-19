package recovery

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

var (
	errNoFailedJobIDs        = errors.New("recovery restart: no failed job IDs supplied")
	errNoPreparedJobsMatched = errors.New("recovery restart: no prepared jobs matched failed job IDs")
	errFailedJobNotPrepared  = errors.New("recovery restart: failed job was not prepared for restart")
)

// FilterJobsBySafeName keeps only prepared jobs whose SafeName appears in
// failedJobIDs, preserving prepared order and failing loudly on stale IDs.
func FilterJobsBySafeName(jobs []model.Job, failedJobIDs []string) ([]model.Job, error) {
	failedSet := make(map[string]struct{}, len(failedJobIDs))
	for _, id := range failedJobIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			failedSet[trimmed] = struct{}{}
		}
	}
	if len(failedSet) == 0 {
		return nil, errNoFailedJobIDs
	}

	filtered := make([]model.Job, 0, len(failedSet))
	matched := make([]string, 0, len(failedSet))
	for i := range jobs {
		job := jobs[i]
		safeName := strings.TrimSpace(job.SafeName)
		if _, ok := failedSet[safeName]; !ok {
			continue
		}
		filtered = append(filtered, job)
		matched = append(matched, safeName)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("%w: %v", errNoPreparedJobsMatched, failedJobIDs)
	}
	for id := range failedSet {
		if !slices.Contains(matched, id) {
			return nil, fmt.Errorf("%w: %q", errFailedJobNotPrepared, id)
		}
	}
	return filtered, nil
}
