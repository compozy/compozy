package task

import (
	"encoding/json"
	"strings"
)

// RunDesignation is the compact assignment slice carried by designated fan-out runs.
type RunDesignation struct {
	GroupID string
	Index   int
	Brief   string
}

// RunDesignationSummary is the prompt/API-safe designation projection for run summaries.
type RunDesignationSummary struct {
	Index int    `json:"index"`
	Brief string `json:"brief,omitempty"`
}

// DesignationFromRun extracts the fan-out assignment metadata from a task run.
func DesignationFromRun(run Run) (RunDesignation, bool) {
	groupID := strings.TrimSpace(run.DesignationGroupID)
	if groupID == "" {
		return RunDesignation{}, false
	}
	designation := RunDesignation{GroupID: groupID}
	if len(run.Metadata) == 0 {
		return designation, true
	}
	var payload struct {
		Designation struct {
			Index int    `json:"index"`
			Brief string `json:"brief"`
		} `json:"designation"`
	}
	if err := json.Unmarshal(run.Metadata, &payload); err != nil {
		return designation, true
	}
	designation.Index = payload.Designation.Index
	designation.Brief = RedactClaimTokens(strings.TrimSpace(payload.Designation.Brief))
	return designation, true
}

// ApplyRunDesignationSummary projects designation metadata from a run into a summary.
func ApplyRunDesignationSummary(summary *RunSummary, run Run) {
	if summary == nil {
		return
	}
	designation, ok := DesignationFromRun(run)
	if !ok {
		return
	}
	summary.DesignationGroupID = designation.GroupID
	summary.Designation = designation.Summary()
}

// Summary returns the prompt/API-safe projection of a run designation.
func (d RunDesignation) Summary() *RunDesignationSummary {
	return &RunDesignationSummary{
		Index: d.Index,
		Brief: RedactClaimTokens(strings.TrimSpace(d.Brief)),
	}
}
