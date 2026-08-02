package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/task"
)

const deathResumeContinuationKind = "death_resume"

// DeadNodeResumeRequest is the complete CAS identity for one confirmed-dead worker.
type DeadNodeResumeRequest struct {
	WorkspaceID      WorkspaceID
	RunID            RunID
	Generation       int
	NodeID           NodeID
	ItemIndex        int
	TaskRunID        string
	SourceSessionID  string
	Checkpoint       *DeathResumeCheckpoint
	ExpectedEpoch    int64
	DeathStreakLimit int
	Cause            string
	Actor            task.ActorContext
	ConfirmedAt      time.Time
}

// Validate rejects ambiguous or unscoped death observations.
func (r DeadNodeResumeRequest) Validate() error {
	if strings.TrimSpace(string(r.WorkspaceID)) == "" || strings.TrimSpace(string(r.RunID)) == "" ||
		r.Generation < 1 || strings.TrimSpace(string(r.NodeID)) == "" || r.ItemIndex < 0 ||
		strings.TrimSpace(r.TaskRunID) == "" || strings.TrimSpace(r.SourceSessionID) == "" ||
		r.ExpectedEpoch < 0 || r.DeathStreakLimit <= 0 ||
		strings.TrimSpace(r.Cause) == "" || r.ConfirmedAt.IsZero() {
		return fmt.Errorf("%w: dead node resume identity is incomplete", ErrValidation)
	}
	if r.Checkpoint != nil {
		if err := r.Checkpoint.Validate(r.SourceSessionID); err != nil {
			return err
		}
	}
	if err := r.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: dead node resume actor: %w", ErrValidation, err)
	}
	return nil
}

// DeadNodeResumeRequestForTaskRun derives the exact cell identity pinned into one worker run.
func DeadNodeResumeRequestForTaskRun(
	run task.Run,
	workspaceID WorkspaceID,
	sourceSessionID string,
	checkpoint *DeathResumeCheckpoint,
	deathStreakLimit int,
	cause string,
	actor task.ActorContext,
	confirmedAt time.Time,
) (DeadNodeResumeRequest, error) {
	metadata, err := parseCoordinatorActionRunMetadata(run.Metadata)
	if err != nil {
		return DeadNodeResumeRequest{}, err
	}
	request := DeadNodeResumeRequest{
		WorkspaceID:      workspaceID,
		RunID:            RunID(strings.TrimSpace(run.LoopRunID)),
		Generation:       metadata.Generation,
		NodeID:           NodeID(metadata.NodeID),
		ItemIndex:        metadata.ItemIndex,
		TaskRunID:        strings.TrimSpace(run.ID),
		SourceSessionID:  strings.TrimSpace(sourceSessionID),
		Checkpoint:       checkpoint.Clone(),
		ExpectedEpoch:    metadata.Epoch,
		DeathStreakLimit: deathStreakLimit,
		Cause:            strings.TrimSpace(cause),
		Actor:            actor,
		ConfirmedAt:      confirmedAt.UTC(),
	}
	if err := request.Validate(); err != nil {
		return DeadNodeResumeRequest{}, err
	}
	return request, nil
}

// DeathResumePartial is one bounded, operator-safe progress fragment from the dead session.
type DeathResumePartial struct {
	Sequence int64  `json:"sequence"`
	Type     string `json:"type"`
	Text     string `json:"text"`
}

// DeathResumeCheckpoint pins the last persisted session-event window carried into a continuation.
type DeathResumeCheckpoint struct {
	SessionID     string               `json:"session_id"`
	EventStartSeq int64                `json:"event_start_seq"`
	EventEndSeq   int64                `json:"event_end_seq"`
	Partials      []DeathResumePartial `json:"partials,omitempty"`
}

// Validate rejects a checkpoint that does not describe the confirmed-dead source session.
func (c DeathResumeCheckpoint) Validate(sourceSessionID string) error {
	if strings.TrimSpace(c.SessionID) == "" ||
		strings.TrimSpace(c.SessionID) != strings.TrimSpace(sourceSessionID) ||
		c.EventStartSeq <= 0 || c.EventEndSeq < c.EventStartSeq {
		return fmt.Errorf("%w: death-resume checkpoint identity is invalid", ErrValidation)
	}
	previous := int64(0)
	for _, partial := range c.Partials {
		if partial.Sequence < c.EventStartSeq || partial.Sequence > c.EventEndSeq ||
			partial.Sequence <= previous || strings.TrimSpace(partial.Type) == "" ||
			strings.TrimSpace(partial.Text) == "" {
			return fmt.Errorf("%w: death-resume checkpoint partial is invalid", ErrValidation)
		}
		previous = partial.Sequence
	}
	return nil
}

// Clone returns an ownership-safe checkpoint copy.
func (c *DeathResumeCheckpoint) Clone() *DeathResumeCheckpoint {
	if c == nil {
		return nil
	}
	cloned := *c
	cloned.Partials = append([]DeathResumePartial(nil), c.Partials...)
	return &cloned
}

// DeathResumeContext identifies the continuation provenance exposed to an action executor.
type DeathResumeContext struct {
	SourceTaskRunID string
	SourceSessionID string
	Checkpoint      *DeathResumeCheckpoint
}

func runAgentPromptWithDeathResume(
	prompt string,
	resume *DeathResumeContext,
) (string, error) {
	if resume == nil {
		return prompt, nil
	}
	payload, err := json.Marshal(struct {
		Kind               string                 `json:"kind"`
		SourceTaskRunID    string                 `json:"source_task_run_id"`
		SourceSessionID    string                 `json:"source_session_id"`
		ProgressCheckpoint *DeathResumeCheckpoint `json:"progress_checkpoint,omitempty"`
	}{
		Kind:               deathResumeContinuationKind,
		SourceTaskRunID:    resume.SourceTaskRunID,
		SourceSessionID:    resume.SourceSessionID,
		ProgressCheckpoint: resume.Checkpoint.Clone(),
	})
	if err != nil {
		return "", fmt.Errorf("loop: marshal death-resume checkpoint: %w", err)
	}
	return strings.TrimSpace(prompt) +
		"\n\nConfirmed-death continuation context:\n" + string(payload) +
		"\nContinue from the recorded progress. Do not repeat completed work. " +
		"If no progress checkpoint is present, restart this node safely from the beginning.", nil
}

// DeadNodeResumeResult reports whether one deterministic continuation was committed.
type DeadNodeResumeResult struct {
	Run               task.Run
	Continued         bool
	Replay            bool
	AttentionRequired bool
	IssuedEpoch       int64
}

// DeadNodeResumer is the single atomic authority for confirmed-death continuation.
type DeadNodeResumer interface {
	ResumeDeadNode(context.Context, DeadNodeResumeRequest) (DeadNodeResumeResult, error)
}
