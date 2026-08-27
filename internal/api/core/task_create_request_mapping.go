package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskCreateFields struct {
	ID                   string
	Identifier           string
	Title                string
	Description          string
	Priority             taskpkg.Priority
	MaxAttempts          *int
	AutoEnqueueOnReady   bool
	Draft                bool
	ApprovalPolicy       taskpkg.ApprovalPolicy
	Owner                *taskpkg.Ownership
	WakeCreator          *bool
	Expect               json.RawMessage
	ResultBudget         string
	ResultOverflow       string
	NetworkParticipation *participation.Request
	Metadata             json.RawMessage
}

func createTaskProjection(
	profileID string,
	scope taskpkg.Scope,
	workspaceID string,
	fields taskCreateFields,
) (taskpkg.CreateTask, error) {
	resultBudget, err := taskResultBudgetFromWire(fields.ResultBudget, fields.ResultOverflow)
	if err != nil {
		return taskpkg.CreateTask{}, err
	}
	return taskpkg.CreateTask{
		ID:                 fields.ID,
		ProfileID:          profileID,
		Identifier:         fields.Identifier,
		Scope:              scope,
		WorkspaceID:        workspaceID,
		Title:              fields.Title,
		Description:        fields.Description,
		Priority:           fields.Priority,
		MaxAttempts:        fields.MaxAttempts,
		AutoEnqueueOnReady: fields.AutoEnqueueOnReady,
		Draft:              fields.Draft,
		ApprovalPolicy:     fields.ApprovalPolicy,
		Owner:              fields.Owner,
		WakeCreator: cloneBoolPtr(
			fields.WakeCreator,
		),
		Expect:       cloneRawMessage(fields.Expect),
		ResultBudget: resultBudget,
		NetworkParticipation: participation.CloneRequest(
			fields.NetworkParticipation,
		),
		Metadata: cloneRawMessage(fields.Metadata),
	}, nil
}

func taskPatchFromRequest(req contract.UpdateTaskRequest) (taskpkg.Patch, error) {
	resultBudget, err := taskResultBudgetFromWire(req.ResultBudget, req.ResultOverflow)
	if err != nil {
		return taskpkg.Patch{}, err
	}
	patch := taskpkg.Patch{
		Title:                trimStringPtr(req.Title),
		Description:          trimStringPtr(req.Description),
		Priority:             normalizePriorityPtr(req.Priority),
		MaxAttempts:          req.MaxAttempts,
		AutoEnqueueOnReady:   req.AutoEnqueueOnReady,
		ApprovalPolicy:       normalizeApprovalPolicyPtr(req.ApprovalPolicy),
		Expect:               cloneRawMessagePtr(req.Expect),
		ResultBudget:         resultBudget,
		Metadata:             cloneRawMessagePtr(req.Metadata),
		Owner:                cloneOwnership(req.Owner),
		ClearOwner:           req.ClearOwner,
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
	}
	if err := patch.Validate("task_patch"); err != nil {
		return taskpkg.Patch{}, err
	}
	return patch, nil
}

func taskResultBudgetFromWire(budgetRaw, overflowRaw string) (*contracts.ByteBudget, error) {
	budgetRaw = strings.TrimSpace(budgetRaw)
	overflowRaw = strings.TrimSpace(overflowRaw)
	if budgetRaw == "" && overflowRaw == "" {
		return nil, nil
	}
	budget := &contracts.ByteBudget{}
	if budgetRaw != "" {
		maxBytes, err := config.ParseByteSize(budgetRaw)
		if err != nil {
			return nil, NewTaskValidationError(fmt.Errorf("result_budget: %w", err))
		}
		budget.MaxBytes = maxBytes
	}
	if overflowRaw != "" {
		budget.Overflow = contracts.OverflowMode(overflowRaw)
	}
	return budget, nil
}
