package tkrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

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
	uc := uc.NewGetTask(appState.Workflows, workflowID, taskID)
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
	router.RespondOK(c, "task retrieved", task)
}

func listTasks(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := uc.NewListTasks(appState.Workflows, workflowID)
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
	router.RespondOK(c, "tasks retrieved", gin.H{
		"tasks": tasks,
	})
}
