package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

// listChildrenExecutions retrieves all child executions for a workflow execution
//
//	@Summary		List child executions by workflow execution ID
//	@Description	Retrieve all child executions (tasks, agents, tools) for a specific workflow execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_exec_id	path		string														true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=object{executions=[]core.Execution}}	"Child executions retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/workflows/{workflow_exec_id}/executions [get]
func listChildrenExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	if workflowExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewListChildrenExecutions(repo, workflowExecID)
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
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}

// listChildrenExecutionsByID retrieves all child executions for a workflow
//
//	@Summary		List child executions by workflow ID
//	@Description	Retrieve all child executions (tasks, agents, tools) for a specific workflow
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]core.Execution}}	"Child executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid workflow ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/executions/children [get]
func listChildrenExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewListChildrenExecutionsByID(repo, workflowID)
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
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}

// listTaskExecutions retrieves task executions for a workflow execution
//
//	@Summary		List task executions by workflow execution ID
//	@Description	Retrieve all task executions for a specific workflow execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_exec_id	path		string														true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=object{executions=[]task.Execution}}	"Task executions retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/workflows/{workflow_exec_id}/executions/tasks [get]
func listTaskExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	if workflowExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListTaskExecutionsByExecID(repo, workflowExecID)
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

// listTaskExecutionsByID retrieves task executions for a workflow
//
//	@Summary		List task executions by workflow ID
//	@Description	Retrieve all task executions for a specific workflow
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]task.Execution}}	"Task executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid workflow ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/executions/children/tasks [get]
func listTaskExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().TaskRepoFactory()
	uc := uc.NewListTaskExecutionsByID(repo, workflowID)
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

// listAgentExecutions retrieves agent executions for a workflow execution
//
//	@Summary		List agent executions by workflow execution ID
//	@Description	Retrieve all agent executions for a specific workflow execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_exec_id	path		string														true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=object{executions=[]agent.Execution}}	"Agent executions retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/workflows/{workflow_exec_id}/executions/agents [get]
func listAgentExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	if workflowExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewListAgentExecutionsByExecID(repo, workflowExecID)
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

// listAgentExecutionsByID retrieves agent executions for a workflow
//
//	@Summary		List agent executions by workflow ID
//	@Description	Retrieve all agent executions for a specific workflow
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]agent.Execution}}	"Agent executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid workflow ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/executions/children/agents [get]
func listAgentExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewListAgentExecutionsByID(repo, workflowID)
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

// listToolExecutions retrieves tool executions for a workflow execution
//
//	@Summary		List tool executions by workflow execution ID
//	@Description	Retrieve all tool executions for a specific workflow execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_exec_id	path		string														true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=object{executions=[]tool.Execution}}	"Tool executions retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}						"Invalid execution ID"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/workflows/{workflow_exec_id}/executions/tools [get]
func listToolExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	if workflowExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewListToolExecutionsByExecID(repo, workflowExecID)
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

// listToolExecutionsByID retrieves tool executions for a workflow
//
//	@Summary		List tool executions by workflow ID
//	@Description	Retrieve all tool executions for a specific workflow
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string														true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]tool.Execution}}	"Tool executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid workflow ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows/{workflow_id}/executions/children/tools [get]
func listToolExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewListToolExecutionsByID(repo, workflowID)
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
