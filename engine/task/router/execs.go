package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

// getTaskExecution retrieves a task execution by ID
//
//	@Summary		Get task execution by ID
//	@Description	Retrieve a specific task execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			task_exec_id	path		string									true	"Task Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200				{object}	router.Response{data=task.Execution}	"Task execution retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		404				{object}	router.Response{error=router.ErrorInfo}	"Execution not found"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/tasks/{task_exec_id} [get]
func getTaskExecution(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	if taskExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewGetExecution(repo, taskExecID)
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

// listAllExecutions retrieves all task executions
//
//	@Summary		List all task executions
//	@Description	Retrieve a list of all task executions across all workflows
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{executions=[]task.Execution}}	"Task executions retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/tasks [get]
func listAllExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListAllExecutions(repo)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list all task executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	router.RespondOK(c, "all task executions retrieved", gin.H{
		"executions": executions,
	})
}

// listExecutionsByID retrieves executions for a specific task
//
//	@Summary		List executions by task ID
//	@Description	Retrieve all executions for a specific task within a workflow
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Param			task_id		path		string														true	"Task ID"		example("validate-input")
//	@Success		200			{object}	router.Response{data=object{executions=[]task.Execution}}	"Task executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid workflow or task ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/tasks/{task_id}/executions [get]
func listExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	taskID := router.GetTaskID(c)
	if taskID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListExecutionsByTaskID(repo, workflowID, taskID)
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
