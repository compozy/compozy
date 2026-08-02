package loop

import (
	"encoding/json"
	"fmt"
	"strings"
)

type coordinatorActionRunMetadata struct {
	Generation          int                    `json:"generation"`
	NodeID              string                 `json:"node_id"`
	ItemIndex           int                    `json:"item_index"`
	Attempt             int                    `json:"attempt"`
	Epoch               int64                  `json:"epoch"`
	GoalSegmentEpoch    int64                  `json:"goal_segment_epoch,omitempty"`
	ContinuationKind    string                 `json:"continuation_kind,omitempty"`
	ResumeFromTaskRunID string                 `json:"resume_from_task_run_id,omitempty"`
	ResumeFromSessionID string                 `json:"resume_from_session_id,omitempty"`
	DeathCheckpoint     *DeathResumeCheckpoint `json:"death_resume_checkpoint,omitempty"`
}

func parseCoordinatorActionRunMetadata(raw json.RawMessage) (coordinatorActionRunMetadata, error) {
	var meta coordinatorActionRunMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return coordinatorActionRunMetadata{}, fmt.Errorf("loop: decode action task metadata: %w", err)
	}
	meta.NodeID = strings.TrimSpace(meta.NodeID)
	meta.ContinuationKind = strings.TrimSpace(meta.ContinuationKind)
	meta.ResumeFromTaskRunID = strings.TrimSpace(meta.ResumeFromTaskRunID)
	meta.ResumeFromSessionID = strings.TrimSpace(meta.ResumeFromSessionID)
	if meta.Generation <= 0 {
		return coordinatorActionRunMetadata{}, fmt.Errorf("%w: action generation must be positive", ErrValidation)
	}
	if meta.NodeID == "" {
		return coordinatorActionRunMetadata{}, fmt.Errorf("%w: action node_id is required", ErrValidation)
	}
	if meta.ItemIndex < 0 {
		return coordinatorActionRunMetadata{}, fmt.Errorf(
			"%w: action item_index must be zero or positive",
			ErrValidation,
		)
	}
	if meta.Attempt < 1 {
		return coordinatorActionRunMetadata{}, fmt.Errorf("%w: action attempt must be positive", ErrValidation)
	}
	if meta.Epoch < 0 {
		return coordinatorActionRunMetadata{}, fmt.Errorf("%w: action epoch must be zero or positive", ErrValidation)
	}
	if meta.GoalSegmentEpoch < 0 {
		return coordinatorActionRunMetadata{}, fmt.Errorf(
			"%w: action goal_segment_epoch must be zero or positive",
			ErrValidation,
		)
	}
	if err := validateCoordinatorContinuationMetadata(meta); err != nil {
		return coordinatorActionRunMetadata{}, err
	}
	return meta, nil
}

func validateCoordinatorContinuationMetadata(meta coordinatorActionRunMetadata) error {
	if meta.ContinuationKind == "" {
		if meta.ResumeFromTaskRunID != "" || meta.ResumeFromSessionID != "" || meta.DeathCheckpoint != nil {
			return fmt.Errorf("%w: action continuation provenance requires a kind", ErrValidation)
		}
		return nil
	}
	if meta.ContinuationKind != deathResumeContinuationKind || meta.ResumeFromTaskRunID == "" ||
		meta.ResumeFromSessionID == "" {
		return fmt.Errorf("%w: death-resume continuation provenance is incomplete", ErrValidation)
	}
	if meta.DeathCheckpoint != nil {
		return meta.DeathCheckpoint.Validate(meta.ResumeFromSessionID)
	}
	return nil
}
