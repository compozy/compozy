package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

func getTaskExecution(c *gin.Context) {
	taskExecID := router.GetTaskExecID(c)
	appState := router.GetAppState(c)
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

func listAllExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
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

func listExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	taskID := router.GetTaskID(c)
	appState := router.GetAppState(c)
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
