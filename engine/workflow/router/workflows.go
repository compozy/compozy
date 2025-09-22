package wfrouter

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/compozy/compozy/engine/tool"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	"github.com/compozy/compozy/engine/workflow"
	wfuc "github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

// getWorkflowByID retrieves a workflow definition by its ID.
//
//	@Summary		Get workflow
//	@Description	Retrieve a workflow configuration with optional field selection and expansion.
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string		true	"Workflow ID"\t		example("data-processing")
//	@Param			project		query	string	false	"Project override"\t		example("staging")
//	@Param			expand		query	string	false	"Comma-separated child collections to expand (tasks,agents,tools)"\texample("tasks,agents")
//	@Success		200		{object}	router.Response{data=wfrouter.WorkflowDTO}	"Workflow retrieved"
//	@Header			200		{string}	ETag		"Strong entity tag for concurrency control"
//	@Header			200		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			200		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			200		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400		{object}	core.ProblemDocument	"Invalid request"
//	@Failure		404		{object}	core.ProblemDocument	"Workflow not found"
//	@Failure		500		{object}	core.ProblemDocument	"Internal server error"
//	@Router			/workflows/{workflow_id} [get]
func getWorkflowByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	expandSet := router.ParseExpandQueries(c.QueryArray("expand"))
	out, err := wfuc.NewGet(store).Execute(c.Request.Context(), &wfuc.GetInput{Project: project, ID: workflowID})
	if err != nil {
		respondWorkflowError(c, err)
		return
	}
	dto := makeWorkflowDTO(out.Config, expandSet)
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	router.RespondOK(c, "workflow retrieved", dto)
}

// listWorkflows returns paginated workflows for the active project.
//
//	@Summary		List workflows
//	@Description	List workflows with cursor pagination, optional prefix search, field selection, or expansion.
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			project	query	string	false	"Project override"\t		example("staging")
//	@Param			limit	query	int		false	"Page size (max 500)"\t		example(50)
//	@Param			cursor	query	string	false	"Opaque pagination cursor"\t	example("djI6YWZ0ZXI6d29ya2Zsb3ctMDAwMQ==")
//	@Param			q		query	string	false	"Filter by workflow ID prefix"\t	example("data-")
//	@Param			expand	query	string	false	"Comma-separated child collections to expand (tasks,agents,tools)"\texample("tasks")
//	@Success		200	{object}	router.Response{data=wfrouter.WorkflowsListResponse}	"workflows retrieved"
//	@Header			200	{string}	Link		"RFC 8288 pagination links for next/prev"
//	@Header			200	{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			200	{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			200	{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400	{object}	core.ProblemDocument	"Invalid cursor"
//	@Failure		500	{object}	core.ProblemDocument	"Internal server error"
//	@Router			/workflows [get]
func listWorkflows(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c, c.Query("limit"), 50, 500)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	prefix := strings.TrimSpace(c.Query("q"))
	expandSet := router.ParseExpandQueries(c.QueryArray("expand"))
	out, err := wfuc.NewList(store).
		Execute(c.Request.Context(), &wfuc.ListInput{
			Project:         project,
			Prefix:          prefix,
			CursorValue:     cursor.Value,
			CursorDirection: resourceutil.CursorDirection(cursor.Direction),
			Limit:           limit,
		})
	if err != nil {
		respondWorkflowError(c, err)
		return
	}
	list := make([]WorkflowListItem, 0, len(out.Items))
	for i := range out.Items {
		dto := makeWorkflowDTO(out.Items[i].Config, expandSet)
		list = append(list, WorkflowListItem{WorkflowDTO: dto, ETag: string(out.Items[i].ETag)})
	}
	nextCursor := ""
	prevCursor := ""
	if out.NextCursorValue != "" && out.NextCursorDirection != resourceutil.CursorDirectionNone {
		nextCursor = router.EncodeCursor(string(out.NextCursorDirection), out.NextCursorValue)
	}
	if out.PrevCursorValue != "" && out.PrevCursorDirection != resourceutil.CursorDirectionNone {
		prevCursor = router.EncodeCursor(string(out.PrevCursorDirection), out.PrevCursorValue)
	}
	router.SetLinkHeaders(c, nextCursor, prevCursor)
	page := router.PageInfoDTO{Limit: limit, Total: out.Total, NextCursor: nextCursor, PrevCursor: prevCursor}
	router.RespondOK(c, "workflows retrieved", WorkflowsListResponse{Workflows: list, Page: page})
}

// upsertWorkflow creates or updates a workflow configuration.
//
//	@Summary		Create or update workflow
//	@Description	Create a workflow when absent or update an existing workflow using strong ETag concurrency.
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string		true	"Workflow ID"\t		example("data-processing")
//	@Param			project		query	string	false	"Project override"\t		example("staging")
//	@Param			expand		query	string	false	"Comma-separated child collections to expand (tasks,agents,tools)"\texample("tasks,agents")
//	@Param			If-Match	header	string	false	"Strong ETag for optimistic concurrency"\texample("\"6b1c1d7f448c1c76\"")
//	@Param			payload		body	workflow.Config	true	"Workflow definition payload"
//	@Success		200		{object}	router.Response{data=wfrouter.WorkflowDTO}	"workflow updated"
//	@Header			200		{string}	ETag		"Strong entity tag for the stored workflow"
//	@Header			200		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			200		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			200		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Success		201		{object}	router.Response{data=wfrouter.WorkflowDTO}	"workflow created"
//	@Header			201		{string}	ETag		"Strong entity tag for the stored workflow"
//	@Header			201		{string}	Location	"Relative URL for the created workflow"
//	@Header			201		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			201		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			201		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400		{object}	core.ProblemDocument	"Invalid request body"
//	@Failure		404		{object}	core.ProblemDocument	"Workflow not found"
//	@Failure		412		{object}	core.ProblemDocument	"ETag mismatch"
//	@Failure		500		{object}	core.ProblemDocument	"Internal server error"
//	@Router			/workflows/{workflow_id} [put]
func upsertWorkflow(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	body := make(map[string]any)
	if err := c.ShouldBindJSON(&body); err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	out, execErr := wfuc.NewUpsert(store).
		Execute(c.Request.Context(), &wfuc.UpsertInput{Project: project, ID: workflowID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondWorkflowError(c, execErr)
		return
	}
	expandSet := router.ParseExpandQueries(c.QueryArray("expand"))
	respBody := makeWorkflowDTO(out.Config, expandSet)
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	if out.Created {
		c.Header("Location", fmt.Sprintf("%s/workflows/%s", routes.Base(), out.Config.ID))
		router.RespondCreated(c, "workflow created", respBody)
		return
	}
	router.RespondOK(c, "workflow updated", respBody)
}

// deleteWorkflow removes a workflow definition.
//
//	@Summary		Delete workflow
//	@Description	Delete a workflow configuration using strong ETag concurrency.
//	@Tags			workflows
//	@Produce		json
//	@Param			workflow_id	path		string	true	"Workflow ID"\t		example("data-processing")
//	@Param			project		query	string	false	"Project override"\t		example("staging")
//	@Success		204		{string}	string	""
//	@Header			204		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			204		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			204		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400		{object}	core.ProblemDocument	"Invalid input"
//	@Failure		404		{object}	core.ProblemDocument	"Workflow not found"
//	@Failure		412		{object}	core.ProblemDocument	"ETag mismatch"
//	@Failure		500		{object}	core.ProblemDocument	"Internal server error"
//	@Router			/workflows/{workflow_id} [delete]
func deleteWorkflow(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	deleteInput := &wfuc.DeleteInput{Project: project, ID: workflowID}
	if err := wfuc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondWorkflowError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondWorkflowError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, wfuc.ErrInvalidInput) ||
		errors.Is(err, wfuc.ErrProjectMissing) ||
		errors.Is(err, wfuc.ErrIDMismatch):
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, wfuc.ErrNotFound):
		core.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, wfuc.ErrETagMismatch) || errors.Is(err, wfuc.ErrStaleIfMatch):
		core.RespondProblem(c, &core.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		core.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}

func makeWorkflowDTO(cfg *workflow.Config, expand map[string]bool) WorkflowDTO {
	dto := ConvertWorkflowConfigToDTO(cfg)
	if expand["tasks"] {
		dto.Tasks = TasksOrDTOs{Expanded: projectTasksExpanded(cfg)}
	} else {
		dto.Tasks = TasksOrDTOs{IDs: collectIDs(cfg.Tasks, func(t task.Config) string { return t.ID })}
	}
	if expand["agents"] {
		dto.Agents = AgentsOrDTOs{Expanded: projectAgentsExpanded(cfg)}
	} else {
		dto.Agents = AgentsOrDTOs{IDs: collectIDs(cfg.Agents, func(a agent.Config) string { return a.ID })}
	}
	if expand["tools"] {
		dto.Tools = ToolsOrDTOs{Expanded: projectToolsExpanded(cfg)}
	} else {
		dto.Tools = ToolsOrDTOs{IDs: collectIDs(cfg.Tools, func(t tool.Config) string { return t.ID })}
	}
	return dto
}

func projectTasksExpanded(cfg *workflow.Config) []tkrouter.TaskDTO {
	out := make([]tkrouter.TaskDTO, 0, len(cfg.Tasks))
	for i := range cfg.Tasks {
		m, err := cfg.Tasks[i].AsMap()
		if err != nil {
			continue
		}
		out = append(out, tkrouter.ToTaskDTOForWorkflow(m))
	}
	return out
}

func projectAgentsExpanded(cfg *workflow.Config) []agentrouter.AgentDTO {
	out := make([]agentrouter.AgentDTO, 0, len(cfg.Agents))
	for i := range cfg.Agents {
		m, err := cfg.Agents[i].AsMap()
		if err != nil {
			continue
		}
		out = append(out, agentrouter.ToAgentDTOForWorkflow(m))
	}
	return out
}

func projectToolsExpanded(cfg *workflow.Config) []toolrouter.ToolDTO {
	out := make([]toolrouter.ToolDTO, 0, len(cfg.Tools))
	for i := range cfg.Tools {
		m, err := cfg.Tools[i].AsMap()
		if err != nil {
			continue
		}
		out = append(out, toolrouter.ToToolDTOForWorkflow(m))
	}
	return out
}

// projectCollection no longer used (typed expand implemented)

func collectIDs[T any](items []T, idFn func(T) string) []string {
	out := make([]string, 0, len(items))
	for i := range items {
		id := strings.TrimSpace(idFn(items[i]))
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// fields filtering removed project-wide (legacy `fields=` feature deleted)
