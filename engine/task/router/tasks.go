package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

// getTaskByID retrieves a task by ID within a workflow
//
//	@Summary		Get task by ID
//	@Description	Retrieve a specific task configuration by its ID within a workflow
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"	example("data-processing")
//	@Param			task_id		path		string									true	"Task ID"		example("validate-input")
//	@Success		200			{object}	router.Response{data=tkrouter.TaskResponse}		"Task retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid workflow or task ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Task not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/workflows/{workflow_id}/tasks/{task_id} [get]
func getTaskByID(c *gin.Context) {
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
	uc := uc.NewGetTask(appState.GetWorkflows(), workflowID, taskID)
	task, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusNotFound,
			"task not found",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	resp, mapErr := ConvertTaskConfigToResponse(task)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map task configuration", mapErr)
		return
	}
	router.RespondOK(c, "task retrieved", resp)
}

// listTasks retrieves all tasks for a workflow
//
//	@Summary		List tasks for a workflow
//	@Description	Retrieve a list of all tasks within a specific workflow
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string												true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{tasks=[]tkrouter.TaskResponse}}	"Tasks retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}				"Invalid workflow ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}				"Workflow not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}				"Internal server error"
//	@Router			/workflows/{workflow_id}/tasks [get]
func listTasks(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := uc.NewListTasks(appState.GetWorkflows(), workflowID)
	tasks, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusNotFound,
			"workflow not found",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	taskResponses, mapErr := ConvertTaskConfigsToResponses(tasks)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map workflow tasks", mapErr)
		return
	}
	router.RespondOK(c, "tasks retrieved", gin.H{
		"tasks": taskResponses,
	})
}
