package contract

import (
	"encoding/json"
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
)

// LoopProvenanceRole identifies how a task participates in one Loop run.
type LoopProvenanceRole string

const (
	// LoopProvenanceRoleCoordinator identifies the run's coordinator task.
	LoopProvenanceRoleCoordinator LoopProvenanceRole = "coordinator"
	// LoopProvenanceRoleCell identifies one executable Loop cell task.
	LoopProvenanceRoleCell LoopProvenanceRole = "cell"
)

// LoopProvenanceRoleValues returns every supported wire role.
func LoopProvenanceRoleValues() []string {
	return []string{
		string(LoopProvenanceRoleCoordinator),
		string(LoopProvenanceRoleCell),
	}
}

// LoopProvenance is the shared structured origin projected by task reads.
type LoopProvenance struct {
	RunID      string             `json:"run_id"`
	LoopName   string             `json:"loop_name,omitempty"`
	Role       LoopProvenanceRole `json:"role"`
	Generation *int               `json:"generation,omitempty"`
	NodeID     string             `json:"node_id,omitempty"`
	ItemIndex  *int               `json:"item_index,omitempty"`
}

type taskLoopMetadata struct {
	LoopRunID  string `json:"loop_run_id"`
	LoopName   string `json:"loop_name"`
	Generation *int   `json:"generation"`
	NodeID     string `json:"node_id"`
	ItemIndex  *int   `json:"item_index"`
}

// LoopProvenanceFromRun builds the public projection from relational run provenance.
func LoopProvenanceFromRun(record *taskpkg.RunProvenance) *LoopProvenance {
	if record == nil || strings.TrimSpace(record.LoopRunID) == "" {
		return nil
	}
	role := LoopProvenanceRoleCell
	if record.RunKind.Normalize() == taskpkg.RunKindCoordinator {
		role = LoopProvenanceRoleCoordinator
	}
	provenance := &LoopProvenance{
		RunID: strings.TrimSpace(record.LoopRunID),
		Role:  role,
	}
	metadata, ok := decodeTaskLoopMetadata(taskpkg.RedactClaimTokenJSON(record.Metadata), provenance.RunID)
	if !ok {
		return provenance
	}
	provenance.LoopName = strings.TrimSpace(metadata.LoopName)
	if role == LoopProvenanceRoleCell {
		provenance.Generation = metadata.Generation
		provenance.NodeID = strings.TrimSpace(metadata.NodeID)
		provenance.ItemIndex = metadata.ItemIndex
	}
	return provenance
}

// LoopProvenanceFromView selects the newest related run for one task detail.
func LoopProvenanceFromView(view *taskpkg.View) *LoopProvenance {
	if view == nil {
		return nil
	}
	var selected *taskpkg.Run
	for index := range view.Runs {
		candidate := &view.Runs[index]
		if strings.TrimSpace(candidate.LoopRunID) == "" {
			continue
		}
		if selected == nil || newerLoopProvenanceRun(candidate, selected) {
			selected = candidate
		}
	}
	if selected == nil {
		return nil
	}
	return LoopProvenanceFromRun(&taskpkg.RunProvenance{
		LoopRunID: selected.LoopRunID,
		RunKind:   selected.RunKind,
		Metadata:  view.Task.Metadata,
	})
}

func newerLoopProvenanceRun(candidate *taskpkg.Run, selected *taskpkg.Run) bool {
	if candidate.Attempt != selected.Attempt {
		return candidate.Attempt > selected.Attempt
	}
	if !candidate.QueuedAt.Equal(selected.QueuedAt) {
		return candidate.QueuedAt.After(selected.QueuedAt)
	}
	return strings.TrimSpace(candidate.ID) > strings.TrimSpace(selected.ID)
}

func decodeTaskLoopMetadata(raw json.RawMessage, runID string) (taskLoopMetadata, bool) {
	if len(raw) == 0 {
		return taskLoopMetadata{}, false
	}
	var metadata taskLoopMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return taskLoopMetadata{}, false
	}
	metadataRunID := strings.TrimSpace(metadata.LoopRunID)
	if metadataRunID != "" && metadataRunID != runID {
		return taskLoopMetadata{}, false
	}
	return metadata, true
}
