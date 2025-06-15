package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
)

const GetDataLabel = "GetWorkflowData"

type GetDataInput struct {
	WorkflowID string `json:"workflow_id"`
}

type GetData struct {
	ProjectConfig  *project.Config
	Workflows      []*workflow.Config
	WorkflowConfig *workflow.Config
}

func NewGetData(projectConfig *project.Config, workflows []*workflow.Config) *GetData {
	return &GetData{ProjectConfig: projectConfig, Workflows: workflows}
}

func (a *GetData) Run(_ context.Context, input *GetDataInput) (*GetData, error) {
	// Try to find specific workflow config
	workflowConfig, err := workflow.FindConfig(a.Workflows, input.WorkflowID)
	if err != nil {
		// If no specific workflow found but WorkflowID matches project name,
		// return project data for dispatcher (which needs access to all workflows)
		if input.WorkflowID == a.ProjectConfig.Name {
			return &GetData{
				ProjectConfig:  a.ProjectConfig,
				Workflows:      a.Workflows,
				WorkflowConfig: nil, // No specific workflow
			}, nil
		}
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	return &GetData{
		ProjectConfig:  a.ProjectConfig,
		Workflows:      a.Workflows,
		WorkflowConfig: workflowConfig,
	}, nil
}
