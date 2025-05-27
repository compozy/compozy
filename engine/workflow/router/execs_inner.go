package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

func listChildrenExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	appState := router.GetAppState(c)
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

func listChildrenExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	appState := router.GetAppState(c)
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

func listTaskExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	appState := router.GetAppState(c)
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

func listTaskExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	appState := router.GetAppState(c)
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

func listAgentExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	appState := router.GetAppState(c)
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

func listAgentExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	appState := router.GetAppState(c)
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

func listToolExecutions(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	appState := router.GetAppState(c)
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

func listToolExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	appState := router.GetAppState(c)
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
