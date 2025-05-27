package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

// Route: GET /api/workflows/:workflow_id/tasks/executions
func listWorkflowTaskExecutions(c *gin.Context) {
	workflowID := router.GetURLParam(c, "workflow_id")
	if workflowID == "" {
		return
	}

	appState := router.GetAppState(c)
	if appState == nil {
		return
	}

	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListWorkflowExecutionsUC(repo, &workflowID, nil)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list workflow task executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	router.RespondOK(c, "workflow task executions retrieved", gin.H{
		"executions": executions,
	})
}

// Route: GET /api/workflows/:workflow_id/tasks/:task_id/executions
func listTaskExecutions(c *gin.Context) {
	workflowID := router.GetURLParam(c, "workflow_id")
	if workflowID == "" {
		return
	}

	taskID := router.GetURLParam(c, "task_id")
	if taskID == "" {
		return
	}

	appState := router.GetAppState(c)
	if appState == nil {
		return
	}

	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListExecutionsUC(repo, workflowID, taskID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list task executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	router.RespondOK(c, "task executions retrieved", gin.H{
		"executions": executions,
	})
}

// Route: GET /api/workflows/:workflow_id/tasks/executions/:task_exec_id
func getTaskExecution(c *gin.Context) {
	taskExecID := core.ID(router.GetURLParam(c, "task_exec_id"))
	if taskExecID == "" {
		return
	}

	appState := router.GetAppState(c)
	if appState == nil {
		return
	}

	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewGetExecutionUC(repo, taskExecID)
	exec, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get task execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	router.RespondOK(c, "task execution retrieved", exec)
}
