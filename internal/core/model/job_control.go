package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const MaxJobControlMessageBytes = 64 * 1024

var (
	ErrJobControlNotFound        = errors.New("job control not found")
	ErrJobControlConflict        = errors.New("job control conflict")
	ErrJobControlMessageRequired = errors.New("job control message is required")
	ErrJobControlMessageTooLarge = errors.New("job control message is too large")
)

type JobControlStatus string

const (
	JobControlStatusPausing JobControlStatus = "pausing"
	JobControlStatusPaused  JobControlStatus = "paused"
	JobControlStatusResumed JobControlStatus = "resumed"
)

type JobControlRequest struct {
	RunID   string
	JobID   string
	Index   int
	Message string
}

type JobControlResponse struct {
	RunID     string           `json:"run_id"`
	JobID     string           `json:"job_id"`
	Index     int              `json:"index"`
	Status    JobControlStatus `json:"status"`
	SessionID string           `json:"session_id,omitempty"`
	MessageID string           `json:"message_id,omitempty"`
}

type JobController interface {
	Pause(context.Context, JobControlRequest) (JobControlResponse, error)
	SendMessage(context.Context, JobControlRequest) (JobControlResponse, error)
}

type JobControlRegistry struct {
	mu       sync.RWMutex
	controls map[string]jobControlEntry
}

type jobControlEntry struct {
	controller JobController
	index      int
}

func NewJobControlRegistry() *JobControlRegistry {
	return &JobControlRegistry{
		controls: make(map[string]jobControlEntry),
	}
}

func (r *JobControlRegistry) Register(
	runID string,
	index int,
	jobID string,
	controller JobController,
) func() {
	if r == nil || controller == nil {
		return func() {}
	}
	runID = strings.TrimSpace(runID)
	ids := jobControlIDs(index, jobID)

	r.mu.Lock()
	for _, id := range ids {
		r.controls[jobControlKey(runID, id)] = jobControlEntry{controller: controller, index: index}
	}
	r.mu.Unlock()

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, id := range ids {
			key := jobControlKey(runID, id)
			if r.controls[key].controller == controller {
				delete(r.controls, key)
			}
		}
	}
}

func (r *JobControlRegistry) Pause(ctx context.Context, runID string, jobID string) (JobControlResponse, error) {
	controller, req, err := r.lookup(runID, jobID)
	if err != nil {
		return JobControlResponse{}, err
	}
	resp, err := controller.Pause(ctx, req)
	return completeJobControlResponse(resp, req), err
}

func (r *JobControlRegistry) SendMessage(
	ctx context.Context,
	runID string,
	jobID string,
	message string,
) (JobControlResponse, error) {
	trimmed, err := ValidateJobControlMessage(message)
	if err != nil {
		return JobControlResponse{}, err
	}
	controller, req, err := r.lookup(runID, jobID)
	if err != nil {
		return JobControlResponse{}, err
	}
	req.Message = trimmed
	resp, err := controller.SendMessage(ctx, req)
	return completeJobControlResponse(resp, req), err
}

func ValidateJobControlMessage(message string) (string, error) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "", ErrJobControlMessageRequired
	}
	if len([]byte(trimmed)) > MaxJobControlMessageBytes {
		return "", fmt.Errorf("%w: max %d bytes", ErrJobControlMessageTooLarge, MaxJobControlMessageBytes)
	}
	return trimmed, nil
}

func (r *JobControlRegistry) lookup(runID string, jobID string) (JobController, JobControlRequest, error) {
	if r == nil {
		return nil, JobControlRequest{}, ErrJobControlNotFound
	}
	runID = strings.TrimSpace(runID)
	jobID = strings.TrimSpace(jobID)
	if runID == "" || jobID == "" {
		return nil, JobControlRequest{}, ErrJobControlNotFound
	}
	key := jobControlKey(runID, jobID)
	r.mu.RLock()
	entry := r.controls[key]
	r.mu.RUnlock()
	if entry.controller == nil {
		return nil, JobControlRequest{}, ErrJobControlNotFound
	}
	index := entry.index
	if index < 0 {
		index = jobIndexFromID(jobID)
	}
	return entry.controller, JobControlRequest{RunID: runID, JobID: jobID, Index: index}, nil
}

func completeJobControlResponse(resp JobControlResponse, req JobControlRequest) JobControlResponse {
	resp.RunID = req.RunID
	resp.JobID = req.JobID
	if req.Index >= 0 {
		resp.Index = req.Index
	} else if resp.Index < 0 {
		resp.Index = req.Index
	}
	return resp
}

func jobControlIDs(index int, jobID string) []string {
	seen := make(map[string]struct{}, 2)
	ids := make([]string, 0, 2)
	appendID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	appendID(jobID)
	if index >= 0 {
		appendID(fmt.Sprintf("job-%03d", index))
	}
	return ids
}

func jobControlKey(runID string, jobID string) string {
	return strings.TrimSpace(runID) + "\x00" + strings.TrimSpace(jobID)
}

func jobIndexFromID(jobID string) int {
	trimmed := strings.TrimSpace(jobID)
	if !strings.HasPrefix(trimmed, "job-") {
		return -1
	}
	var index int
	if _, err := fmt.Sscanf(trimmed, "job-%03d", &index); err != nil {
		return -1
	}
	return index
}
