package daemon

import (
	"context"

	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	harnessDetachedMetadataSchema  = "agh.harness.detached.v1"
	harnessDetachedTaskMetadataKey = "harness_detached_task"
	harnessDetachedRunMetadataKey  = "harness_detached_run"
	harnessDetachedActorRefPrefix  = "harness-detached"
	harnessDetachedOriginRefPrefix = "daemon.harness.detached"
	defaultDetachedHarnessSummary  = "Detached harness work"
)

type detachedHarnessWakeTargetInput struct {
	SessionID string
}

type detachedHarnessWakeTarget struct {
	SessionID   string `json:"session_id"`
	SessionType string `json:"session_type,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type detachedHarnessTaskMetadata struct {
	Schema               string                    `json:"schema"`
	Kind                 string                    `json:"kind"`
	SubmissionKey        string                    `json:"submission_key"`
	Summary              string                    `json:"summary,omitempty"`
	SubmissionTurnSource string                    `json:"submission_turn_source,omitempty"`
	OwnerSessionID       string                    `json:"owner_session_id"`
	OwnerSessionType     string                    `json:"owner_session_type,omitempty"`
	OwnerWorkspaceID     string                    `json:"owner_workspace_id,omitempty"`
	WakeTarget           detachedHarnessWakeTarget `json:"wake_target"`
}

type detachedHarnessRunMetadata struct {
	Schema               string                    `json:"schema"`
	Kind                 string                    `json:"kind"`
	SubmissionKey        string                    `json:"submission_key"`
	Summary              string                    `json:"summary,omitempty"`
	SubmissionTurnSource string                    `json:"submission_turn_source,omitempty"`
	OwnerSessionID       string                    `json:"owner_session_id"`
	OwnerSessionType     string                    `json:"owner_session_type,omitempty"`
	OwnerWorkspaceID     string                    `json:"owner_workspace_id,omitempty"`
	WakeTarget           detachedHarnessWakeTarget `json:"wake_target"`
	Reentry              *detachedHarnessReentry   `json:"reentry,omitempty"`
}

type detachedHarnessReentry struct {
	Outcome     string    `json:"outcome,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
}

type detachedHarnessSubmitRequest struct {
	SubmissionKey  string
	OwnerSessionID string
	Scope          taskpkg.Scope
	WorkspaceID    string
	Summary        string
	Description    string
	TurnSource     session.TurnSource
	WakeTarget     detachedHarnessWakeTargetInput
}

type detachedHarnessSubmission struct {
	Task         taskpkg.Task
	Run          taskpkg.Run
	ExistingTask bool
	ExistingRun  bool
}

type harnessDetachedSessionReader interface {
	Status(ctx context.Context, id string) (*session.Info, error)
}

type harnessDetachedWorkBridge struct {
	tasks    taskpkg.Manager
	store    taskStore
	sessions harnessDetachedSessionReader
}

type normalizedDetachedHarnessSubmitRequest struct {
	TaskID           string
	SubmissionKey    string
	Scope            taskpkg.Scope
	WorkspaceID      string
	Summary          string
	Description      string
	TurnSource       session.TurnSource
	OwnerSessionID   string
	OwnerSessionType string
	OwnerWorkspaceID string
	WakeTarget       detachedHarnessWakeTarget
}

func newHarnessDetachedWorkBridge(
	tasks taskpkg.Manager,
	store taskStore,
	sessions harnessDetachedSessionReader,
) (*harnessDetachedWorkBridge, error) {
	if tasks == nil {
		return nil, errors.New("daemon: harness detached work bridge requires task manager")
	}
	if store == nil {
		return nil, errors.New("daemon: harness detached work bridge requires task store")
	}
	if sessions == nil {
		return nil, errors.New("daemon: harness detached work bridge requires session reader")
	}
	return &harnessDetachedWorkBridge{
		tasks:    tasks,
		store:    store,
		sessions: sessions,
	}, nil
}

func (b *harnessDetachedWorkBridge) submit(
	ctx context.Context,
	req detachedHarnessSubmitRequest,
) (*detachedHarnessSubmission, error) {
	if ctx == nil {
		return nil, errors.New("daemon: detached harness submit context is required")
	}
	if b == nil {
		return nil, errors.New("daemon: detached harness work bridge is required")
	}

	normalized, err := b.normalizeSubmitRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	actor, err := detachedHarnessActorContext(normalized.OwnerSessionID)
	if err != nil {
		return nil, err
	}

	taskMetadata := buildDetachedHarnessTaskMetadata(normalized)
	runMetadata := buildDetachedHarnessRunMetadata(normalized)
	taskMetadataJSON, err := marshalDetachedHarnessMetadata(taskMetadata)
	if err != nil {
		return nil, err
	}
	runMetadataJSON, err := marshalDetachedHarnessMetadata(runMetadata)
	if err != nil {
		return nil, err
	}

	existingRun, existingRunFound, err := b.lookupExistingRun(ctx, normalized, actor.Origin, runMetadata)
	if err != nil {
		return nil, err
	}

	taskRecord, existingTask, err := b.ensureTask(ctx, normalized, actor, taskMetadata, taskMetadataJSON)
	if err != nil {
		return nil, err
	}
	if existingRunFound && strings.TrimSpace(existingRun.TaskID) != taskRecord.ID {
		return nil, fmt.Errorf(
			"%w: detached harness submission %q is already bound to task %q",
			taskpkg.ErrValidation,
			normalized.SubmissionKey,
			existingRun.TaskID,
		)
	}

	run, err := b.tasks.EnqueueRun(ctx, taskpkg.EnqueueRun{
		TaskID:         taskRecord.ID,
		IdempotencyKey: normalized.SubmissionKey,
		Metadata:       runMetadataJSON,
	}, actor)
	if err != nil {
		return nil, err
	}
	if run == nil || strings.TrimSpace(run.ID) == "" {
		return nil, errors.New("daemon: detached harness submit returned empty task run")
	}

	return &detachedHarnessSubmission{
		Task:         taskRecord,
		Run:          *run,
		ExistingTask: existingTask,
		ExistingRun:  existingRunFound,
	}, nil
}

func (b *harnessDetachedWorkBridge) normalizeSubmitRequest(
	ctx context.Context,
	req detachedHarnessSubmitRequest,
) (normalizedDetachedHarnessSubmitRequest, error) {
	submissionKey := strings.TrimSpace(req.SubmissionKey)
	if submissionKey == "" {
		return normalizedDetachedHarnessSubmitRequest{}, fmt.Errorf(
			"%w: detached harness submission key is required",
			taskpkg.ErrValidation,
		)
	}

	ownerInfo, err := b.lookupDetachedHarnessSession(
		ctx,
		strings.TrimSpace(req.OwnerSessionID),
		"owner_session_id",
	)
	if err != nil {
		return normalizedDetachedHarnessSubmitRequest{}, err
	}
	wakeInfo, err := b.lookupDetachedHarnessSession(
		ctx,
		strings.TrimSpace(req.WakeTarget.SessionID),
		"wake_target.session_id",
	)
	if err != nil {
		return normalizedDetachedHarnessSubmitRequest{}, err
	}

	scope := req.Scope.Normalize()
	workspaceID, err := normalizeDetachedHarnessWorkspace(
		scope,
		strings.TrimSpace(req.WorkspaceID),
		ownerInfo,
		wakeInfo,
	)
	if err != nil {
		return normalizedDetachedHarnessSubmitRequest{}, err
	}

	return normalizedDetachedHarnessSubmitRequest{
		TaskID:           detachedHarnessTaskID(ownerInfo.ID, submissionKey),
		SubmissionKey:    submissionKey,
		Scope:            scope,
		WorkspaceID:      workspaceID,
		Summary:          detachedHarnessSummary(req.Summary),
		Description:      strings.TrimSpace(req.Description),
		TurnSource:       normalizeDetachedHarnessTurnSource(req.TurnSource),
		OwnerSessionID:   strings.TrimSpace(ownerInfo.ID),
		OwnerSessionType: string(ownerInfo.Type),
		OwnerWorkspaceID: strings.TrimSpace(ownerInfo.WorkspaceID),
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   strings.TrimSpace(wakeInfo.ID),
			SessionType: string(wakeInfo.Type),
			WorkspaceID: strings.TrimSpace(wakeInfo.WorkspaceID),
		},
	}, nil
}
