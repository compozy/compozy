package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
)

const GetDataLabel = "GetWorkflowData"

type GetDataInput struct {
	WorkflowID string `json:"workflow_id"`
}

type GetData struct {
	ProjectConfig  *project.Config
	Workflows      []*workflow.Config
	WorkflowConfig *workflow.Config
	AppConfig      *config.Config
}

func NewGetData(projectConfig *project.Config, workflows []*workflow.Config, appConfig *config.Config) *GetData {
	return &GetData{ProjectConfig: projectConfig, Workflows: workflows, AppConfig: appConfig}
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
				AppConfig:      a.AppConfig,
			}, nil
		}
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	return &GetData{
		ProjectConfig:  a.ProjectConfig,
		Workflows:      a.Workflows,
		WorkflowConfig: workflowConfig,
		AppConfig:      a.AppConfig,
	}, nil
}
