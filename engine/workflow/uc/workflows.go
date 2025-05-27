package uc

import (
	"context"

	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// GetWorkflows
// -----------------------------------------------------------------------------

type GetWorkflow struct {
	workflows  []*workflow.Config
	workflowID string
}

func NewGetWorkflow(workflows []*workflow.Config, workflowID string) *GetWorkflow {
	return &GetWorkflow{
		workflows:  workflows,
		workflowID: workflowID,
	}
}

func (uc *GetWorkflow) Execute(ctx context.Context) (*workflow.Config, error) {
	return workflow.FindConfig(uc.workflows, uc.workflowID)
}

// -----------------------------------------------------------------------------
// ListWorkflows
// -----------------------------------------------------------------------------

type ListWorkflows struct {
	workflows []*workflow.Config
}

func NewListWorkflows(workflows []*workflow.Config) *ListWorkflows {
	return &ListWorkflows{
		workflows: workflows,
	}
}

func (uc *ListWorkflows) Execute(ctx context.Context) ([]*workflow.Config, error) {
	return uc.workflows, nil
}
