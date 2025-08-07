package domain

import (
	"time"
)

// WorkflowStatus represents the overall status of a release workflow
type WorkflowStatus string

const (
	WorkflowStatusPending    WorkflowStatus = "pending"
	WorkflowStatusRunning    WorkflowStatus = "running"
	WorkflowStatusCompleted  WorkflowStatus = "completed"
	WorkflowStatusFailed     WorkflowStatus = "failed"
	WorkflowStatusRolledBack WorkflowStatus = "rolled_back"
)

// OperationStatus represents the status of an individual operation
type OperationStatus string

const (
	OperationStatusPending    OperationStatus = "pending"
	OperationStatusRunning    OperationStatus = "running"
	OperationStatusCompleted  OperationStatus = "completed"
	OperationStatusFailed     OperationStatus = "failed"
	OperationStatusRolledBack OperationStatus = "rolled_back"
)

// OperationType identifies the type of operation
type OperationType string

const (
	OperationTypeCheckChanges      OperationType = "check_changes"
	OperationTypeCalculateVersion  OperationType = "calculate_version"
	OperationTypeCreateBranch      OperationType = "create_branch"
	OperationTypeCheckoutBranch    OperationType = "checkout_branch"
	OperationTypeUpdatePackages    OperationType = "update_packages"
	OperationTypeGenerateChangelog OperationType = "generate_changelog"
	OperationTypeCommitChanges     OperationType = "commit_changes"
	OperationTypePushBranch        OperationType = "push_branch"
	OperationTypeCreatePR          OperationType = "create_pr"
)

// RollbackState represents the state of a release workflow for rollback purposes
type RollbackState struct {
	SessionID      string            `json:"session_id"`
	StartedAt      time.Time         `json:"started_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Version        string            `json:"version"`
	BranchName     string            `json:"branch_name"`
	OriginalBranch string            `json:"original_branch"`
	Operations     []OperationRecord `json:"operations"`
	Status         WorkflowStatus    `json:"status"`
	Error          string            `json:"error,omitempty"`
}

// OperationRecord represents a single operation in the workflow
type OperationRecord struct {
	ID           string          `json:"id"`
	Type         OperationType   `json:"type"`
	Status       OperationStatus `json:"status"`
	StartedAt    time.Time       `json:"started_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	RollbackData map[string]any  `json:"rollback_data,omitempty"`
	Error        string          `json:"error,omitempty"`
}

// NewRollbackState creates a new rollback state
func NewRollbackState(sessionID string) *RollbackState {
	now := time.Now()
	return &RollbackState{
		SessionID:  sessionID,
		StartedAt:  now,
		UpdatedAt:  now,
		Operations: []OperationRecord{},
		Status:     WorkflowStatusPending,
	}
}

// AddOperation adds a new operation record to the state
func (rs *RollbackState) AddOperation(opType OperationType) *OperationRecord {
	op := OperationRecord{
		ID:        generateOperationID(opType),
		Type:      opType,
		Status:    OperationStatusPending,
		StartedAt: time.Now(),
	}
	rs.Operations = append(rs.Operations, op)
	rs.UpdatedAt = time.Now()
	return &rs.Operations[len(rs.Operations)-1]
}

// GetLastOperation returns the most recent operation
func (rs *RollbackState) GetLastOperation() *OperationRecord {
	if len(rs.Operations) == 0 {
		return nil
	}
	return &rs.Operations[len(rs.Operations)-1]
}

// GetCompletedOperations returns all successfully completed operations in reverse order
func (rs *RollbackState) GetCompletedOperations() []OperationRecord {
	var completed []OperationRecord
	for i := len(rs.Operations) - 1; i >= 0; i-- {
		if rs.Operations[i].Status == OperationStatusCompleted {
			completed = append(completed, rs.Operations[i])
		}
	}
	return completed
}

// MarkOperationStarted marks an operation as started
func (rs *RollbackState) MarkOperationStarted(opType OperationType) {
	for i := range rs.Operations {
		if rs.Operations[i].Type == opType && rs.Operations[i].Status == OperationStatusPending {
			rs.Operations[i].Status = OperationStatusRunning
			rs.Operations[i].StartedAt = time.Now()
			rs.UpdatedAt = time.Now()
			break
		}
	}
}

// MarkOperationCompleted marks an operation as completed with rollback data
func (rs *RollbackState) MarkOperationCompleted(opType OperationType, rollbackData map[string]any) {
	now := time.Now()
	for i := range rs.Operations {
		if rs.Operations[i].Type == opType && rs.Operations[i].Status == OperationStatusRunning {
			rs.Operations[i].Status = OperationStatusCompleted
			rs.Operations[i].CompletedAt = &now
			rs.Operations[i].RollbackData = rollbackData
			rs.UpdatedAt = now
			break
		}
	}
}

// MarkOperationFailed marks an operation as failed
func (rs *RollbackState) MarkOperationFailed(opType OperationType, err error) {
	now := time.Now()
	for i := range rs.Operations {
		if rs.Operations[i].Type == opType && rs.Operations[i].Status == OperationStatusRunning {
			rs.Operations[i].Status = OperationStatusFailed
			rs.Operations[i].CompletedAt = &now
			rs.Operations[i].Error = err.Error()
			rs.UpdatedAt = now
			break
		}
	}
	rs.Status = WorkflowStatusFailed
	rs.Error = err.Error()
}

// generateOperationID creates a unique ID for an operation
func generateOperationID(opType OperationType) string {
	return string(opType) + "_" + time.Now().Format("20060102150405")
}
