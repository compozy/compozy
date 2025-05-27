package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

func listChildrenExecutions(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	appState := router.GetAppState(c)
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
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}

func listChildrenExecutionsByID(c *gin.Context) {
	taskID := router.GetTaskID(c)
	appState := router.GetAppState(c)
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
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}

func listAgentExecutions(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	appState := router.GetAppState(c)
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

func listAgentExecutionsByID(c *gin.Context) {
	taskID := router.GetTaskID(c)
	appState := router.GetAppState(c)
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

func listToolExecutions(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	appState := router.GetAppState(c)
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

func listToolExecutionsByID(c *gin.Context) {
	taskID := router.GetTaskID(c)
	appState := router.GetAppState(c)
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
