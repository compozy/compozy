package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

// listChildrenExecutions retrieves all child executions for a task execution
//
//	@Summary		List child executions by task execution ID
//	@Description	Retrieve all child executions (agents, tools) for a specific task execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			task_exec_id	path		string														true	"Task Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200				{object}	router.Response{data=object{executions=[]core.Execution}}	"Child executions retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/tasks/{task_exec_id}/executions [get]
func listChildrenExecutions(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	if taskExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListChildrenExecutions(repo, taskExecID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "task children executions retrieved", gin.H{
		"executions": executions,
	})
}

// listChildrenExecutionsByID retrieves all child executions for a task
//
//	@Summary		List child executions by task ID
//	@Description	Retrieve all child executions (agents, tools) for a specific task
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Param			task_id		path		string														true	"Task ID"		example("validate-input")
//	@Success		200			{object}	router.Response{data=object{executions=[]core.Execution}}	"Child executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid task ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/tasks/{task_id}/executions/children [get]
func listChildrenExecutionsByID(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListChildrenExecutionsByID(repo, taskID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "task children executions retrieved", gin.H{
		"executions": executions,
	})
}

// listAgentExecutions retrieves agent executions for a task execution
//
//	@Summary		List agent executions by task execution ID
//	@Description	Retrieve all agent executions for a specific task execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			task_exec_id	path		string														true	"Task Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200				{object}	router.Response{data=object{executions=[]agent.Execution}}	"Agent executions retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/tasks/{task_exec_id}/executions/agents [get]
func listAgentExecutions(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	if taskExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewListAgentExecutionsByExecID(repo, taskExecID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list agent executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "agent executions retrieved", gin.H{
		"executions": executions,
	})
}

// listAgentExecutionsByID retrieves agent executions for a task
//
//	@Summary		List agent executions by task ID
//	@Description	Retrieve all agent executions for a specific task
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Param			task_id		path		string														true	"Task ID"		example("validate-input")
//	@Success		200			{object}	router.Response{data=object{executions=[]agent.Execution}}	"Agent executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid task ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/tasks/{task_id}/executions/agents [get]
func listAgentExecutionsByID(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewListAgentExecutionsByID(repo, taskID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list agent executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "agent executions retrieved", gin.H{
		"executions": executions,
	})
}

// listToolExecutions retrieves tool executions for a task execution
//
//	@Summary		List tool executions by task execution ID
//	@Description	Retrieve all tool executions for a specific task execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			task_exec_id	path		string														true	"Task Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200				{object}	router.Response{data=object{executions=[]tool.Execution}}	"Tool executions retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/tasks/{task_exec_id}/executions/tools [get]
func listToolExecutions(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	if taskExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewListToolExecutionsByExecID(repo, taskExecID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list tool executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tool executions retrieved", gin.H{
		"executions": executions,
	})
}

// listToolExecutionsByID retrieves tool executions for a task
//
//	@Summary		List tool executions by task ID
//	@Description	Retrieve all tool executions for a specific task
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Param			task_id		path		string														true	"Task ID"		example("validate-input")
//	@Success		200			{object}	router.Response{data=object{executions=[]tool.Execution}}	"Tool executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid task ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/tasks/{task_id}/executions/tools [get]
func listToolExecutionsByID(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewListToolExecutionsByID(repo, taskID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list tool executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tool executions retrieved", gin.H{
		"executions": executions,
	})
}
