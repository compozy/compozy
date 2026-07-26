package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TaskDesignationRollup stores the terminal summary for a designated run group.
type TaskDesignationRollup struct {
	DesignationGroupID string
	TaskID             string
	SummaryJSON        json.RawMessage
	CreatedAt          time.Time
}

// TaskDesignationRollupQuery filters designation rollup reads.
type TaskDesignationRollupQuery struct {
	DesignationGroupID string
	TaskID             string
	Limit              int
}

// Validate ensures rollup reads cannot become unbounded.
func (q TaskDesignationRollupQuery) Validate() error {
	if strings.TrimSpace(q.DesignationGroupID) == "" && strings.TrimSpace(q.TaskID) == "" {
		return fmt.Errorf("store: task designation rollup query requires group_id or task_id")
	}
	return requirePositiveLimit(q.Limit, "task designation rollup limit")
}

// Validate ensures the rollup can be retrieved by task and group.
func (r TaskDesignationRollup) Validate() error {
	if err := requireField(r.DesignationGroupID, "task designation rollup group_id"); err != nil {
		return err
	}
	if err := requireField(r.TaskID, "task designation rollup task_id"); err != nil {
		return err
	}
	if len(r.SummaryJSON) == 0 || !json.Valid(r.SummaryJSON) {
		return fmt.Errorf("store: task designation rollup summary_json must be valid JSON")
	}
	return nil
}
