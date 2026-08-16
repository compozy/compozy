package update

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ApplyStatus is the settings/app request acceptance vocabulary.
type ApplyStatus string

const (
	ApplyStatusAccepted ApplyStatus = "accepted"
	ApplyStatusBlocked  ApplyStatus = "blocked"
	ApplyStatusFailed   ApplyStatus = "failed"
)

// AppApplyRequest binds a shell apply request to one verified artifact identity.
type AppApplyRequest struct {
	RequestedBy Actor
	App         AppOperationState
	Holder      Holder
	Deadline    time.Time
}

// AppApplyResult reports durable acquisition, never a terminal install verdict.
type AppApplyResult struct {
	Target      Target      `json:"target"`
	Status      ApplyStatus `json:"status"`
	OperationID string      `json:"operation_id,omitempty"`
	Message     string      `json:"message"`
	Holder      *Holder     `json:"holder,omitempty"`
}

// RequestAppApply durably records a verified app asset and returns acceptance truth.
func (s *OperationStore) RequestAppApply(ctx context.Context, request AppApplyRequest) (AppApplyResult, error) {
	if strings.TrimSpace(request.App.AttemptID) == "" || strings.TrimSpace(request.App.Asset) == "" ||
		strings.TrimSpace(request.App.Digest) == "" {
		return AppApplyResult{
				Target:  TargetApp,
				Status:  ApplyStatusFailed,
				Message: "Verified app asset identity is required.",
			},
			errors.New(
				"update: verified app asset identity and attempt id are required",
			)
	}
	request.App.Phase = PhasePending
	operation, err := s.Acquire(ctx, OperationRequest{
		RequestedBy: request.RequestedBy,
		Targets:     []Target{TargetApp},
		App:         &request.App,
		Holder:      request.Holder,
		Deadline:    request.Deadline,
	})
	if err != nil {
		var blocked *BlockedError
		if errors.As(err, &blocked) {
			return AppApplyResult{
				Target: TargetApp, Status: ApplyStatusBlocked, OperationID: blocked.Operation.ID,
				Message: "An update operation is already active.", Holder: cloneHolder(blocked.Operation.Holder),
			}, nil
		}
		return AppApplyResult{Target: TargetApp, Status: ApplyStatusFailed, Message: err.Error()}, err
	}
	return AppApplyResult{
		Target: TargetApp, Status: ApplyStatusAccepted, OperationID: operation.ID,
		Message: "App update accepted.",
	}, nil
}
