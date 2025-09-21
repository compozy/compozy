package wfrouter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
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
//	@Param			fields		query	string	false	"Comma-separated list of fields to include"\t	example("id,name,tasks,_etag")
//	@Param			expand		query	string	false	"Comma-separated child collections to expand (tasks,agents,tools)"\texample("tasks,agents")
//	@Success		200		{object}	router.Response{data=wfrouter.WorkflowDocument}	"Workflow retrieved"
//	@Header			200		{string}	ETag		"Strong entity tag for concurrency control"
//	@Header			200		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			200		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			200		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400		{object}	router.ProblemDocument	"Invalid request"
//	@Failure		404		{object}	router.ProblemDocument	"Workflow not found"
//	@Failure		500		{object}	router.ProblemDocument	"Internal server error"
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
	expandSet := router.ParseExpandQuery(c.Query("expand"))
	fieldsSet := router.ParseFieldsQuery(c.Query("fields"))
	out, err := wfuc.NewGet(store).Execute(c.Request.Context(), &wfuc.GetInput{Project: project, ID: workflowID})
	if err != nil {
		respondWorkflowError(c, err)
		return
	}
	body, buildErr := buildWorkflowResponse(out.Config, expandSet)
	if buildErr != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: buildErr.Error()})
		return
	}
	body = filterWorkflowFields(body, fieldsSet)
	body["_etag"] = string(out.ETag)
	c.Header("ETag", string(out.ETag))
	router.RespondOK(c, "workflow retrieved", body)
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
//	@Param			fields	query	string	false	"Comma-separated list of fields to include"\t	example("id,name,_etag")
//	@Param			expand	query	string	false	"Comma-separated child collections to expand (tasks,agents,tools)"\texample("tasks")
//	@Success		200	{object}	router.Response{data=wfrouter.WorkflowListDocument}	"Workflows retrieved"
//	@Header			200	{string}	Link		"RFC 8288 pagination links for next/prev"
//	@Header			200	{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			200	{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			200	{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400	{object}	router.ProblemDocument	"Invalid cursor"
//	@Failure		500	{object}	router.ProblemDocument	"Internal server error"
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
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	prefix := strings.TrimSpace(c.Query("q"))
	expandSet := router.ParseExpandQuery(c.Query("expand"))
	fieldsSet := router.ParseFieldsQuery(c.Query("fields"))
	out, err := wfuc.NewList(store).
		Execute(c.Request.Context(), &wfuc.ListInput{
			Project:         project,
			Prefix:          prefix,
			CursorValue:     cursor.Value,
			CursorDirection: resourceutil.CursorDirection(cursor.Direction),
			Limit:           limit,
		})
	if err != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
		return
	}
	responses := make([]map[string]any, 0, len(out.Items))
	for i := range out.Items {
		body, buildErr := buildWorkflowResponse(out.Items[i].Config, expandSet)
		if buildErr != nil {
			router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: buildErr.Error()})
			return
		}
		body = filterWorkflowFields(body, fieldsSet)
		body["_etag"] = string(out.Items[i].ETag)
		responses = append(responses, body)
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
	page := map[string]any{"limit": limit, "total": out.Total}
	if nextCursor != "" {
		page["next_cursor"] = nextCursor
	}
	if prevCursor != "" {
		page["prev_cursor"] = prevCursor
	}
	router.RespondOK(c, "workflows retrieved", gin.H{"workflows": responses, "page": page})
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
//	@Param			fields		query	string	false	"Comma-separated list of fields to include"\t	example("id,name,_etag")
//	@Param			expand		query	string	false	"Comma-separated child collections to expand (tasks,agents,tools)"\texample("tasks,agents")
//	@Param			If-Match	header	string	false	"Strong ETag for optimistic concurrency"\texample("\"6b1c1d7f448c1c76\"")
//	@Param			payload		body	workflow.Config	true	"Workflow definition payload"
//	@Success		200		{object}	router.Response{data=wfrouter.WorkflowDocument}	"Workflow updated"
//	@Header			200		{string}	ETag		"Strong entity tag for the stored workflow"
//	@Header			200		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			200		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			200		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Success		201		{object}	router.Response{data=wfrouter.WorkflowDocument}	"Workflow created"
//	@Header			201		{string}	ETag		"Strong entity tag for the stored workflow"
//	@Header			201		{string}	Location	"Absolute URL for the created workflow"
//	@Header			201		{string}	RateLimit-Limit		"Requests allowed in the current window"
//	@Header			201		{string}	RateLimit-Remaining	"Remaining requests in the current window"
//	@Header			201		{string}	RateLimit-Reset		"Seconds until the window resets"
//	@Failure		400		{object}	router.ProblemDocument	"Invalid request body"
//	@Failure		404		{object}	router.ProblemDocument	"Workflow not found"
//	@Failure		412		{object}	router.ProblemDocument	"ETag mismatch"
//	@Failure		500		{object}	router.ProblemDocument	"Internal server error"
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
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	out, execErr := wfuc.NewUpsert(store).
		Execute(c.Request.Context(), &wfuc.UpsertInput{Project: project, ID: workflowID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondWorkflowError(c, execErr)
		return
	}
	expandSet := router.ParseExpandQuery(c.Query("expand"))
	fieldsSet := router.ParseFieldsQuery(c.Query("fields"))
	respBody, buildErr := buildWorkflowResponse(out.Config, expandSet)
	if buildErr != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: buildErr.Error()})
		return
	}
	respBody = filterWorkflowFields(respBody, fieldsSet)
	respBody["_etag"] = string(out.ETag)
	c.Header("ETag", string(out.ETag))
	status := http.StatusOK
	if out.Created {
		status = http.StatusCreated
		c.Header("Location", fmt.Sprintf("%s/workflows/%s", routes.Base(), out.Config.ID))
	}
	c.JSON(status, router.NewResponse(status, "workflow stored", respBody))
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
//	@Failure		400		{object}	router.ProblemDocument	"Invalid input"
//	@Failure		404		{object}	router.ProblemDocument	"Workflow not found"
//	@Failure		412		{object}	router.ProblemDocument	"ETag mismatch"
//	@Failure		500		{object}	router.ProblemDocument	"Internal server error"
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
	switch err {
	case wfuc.ErrInvalidInput, wfuc.ErrProjectMissing, wfuc.ErrIDMismatch:
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case wfuc.ErrNotFound:
		router.RespondProblem(c, &router.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case wfuc.ErrETagMismatch, wfuc.ErrStaleIfMatch:
		router.RespondProblem(c, &router.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}

func buildWorkflowResponse(cfg *workflow.Config, expand map[string]bool) (map[string]any, error) {
	m, err := cfg.AsMap()
	if err != nil {
		return nil, err
	}
	m["tasks"] = projectTasks(cfg.Tasks, expand["tasks"])
	m["agents"] = projectAgents(cfg.Agents, expand["agents"])
	m["tools"] = projectTools(cfg.Tools, expand["tools"])
	m["task_count"] = len(cfg.Tasks)
	m["agent_count"] = len(cfg.Agents)
	m["tool_count"] = len(cfg.Tools)
	m["task_ids"] = collectIDs(cfg.Tasks, func(t task.Config) string { return t.ID })
	m["agent_ids"] = collectIDs(cfg.Agents, func(a agent.Config) string { return a.ID })
	m["tool_ids"] = collectIDs(cfg.Tools, func(t tool.Config) string { return t.ID })
	return m, nil
}

func projectTasks(items []task.Config, expanded bool) any {
	return projectCollection(
		items,
		expanded,
		func(cfg task.Config) string { return cfg.ID },
		func(cfg task.Config) (map[string]any, error) {
			clone := cfg
			return clone.AsMap()
		},
	)
}

func projectAgents(items []agent.Config, expanded bool) any {
	return projectCollection(
		items,
		expanded,
		func(cfg agent.Config) string { return cfg.ID },
		func(cfg agent.Config) (map[string]any, error) {
			clone := cfg
			return clone.AsMap()
		},
	)
}

func projectTools(items []tool.Config, expanded bool) any {
	return projectCollection(
		items,
		expanded,
		func(cfg tool.Config) string { return cfg.ID },
		func(cfg tool.Config) (map[string]any, error) {
			clone := cfg
			return clone.AsMap()
		},
	)
}

func projectCollection[T any](
	items []T,
	expanded bool,
	idFn func(T) string,
	mapFn func(T) (map[string]any, error),
) any {
	if !expanded {
		return collectIDs(items, idFn)
	}
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		mapped, err := mapFn(items[i])
		if err != nil {
			continue
		}
		out = append(out, mapped)
	}
	return out
}

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

func filterWorkflowFields(body map[string]any, fields map[string]bool) map[string]any {
	if len(fields) == 0 {
		return body
	}
	filtered := make(map[string]any, len(fields))
	for field := range fields {
		if value, ok := body[field]; ok {
			filtered[field] = value
		}
	}
	if _, ok := filtered["id"]; !ok {
		if value, has := body["id"]; has {
			filtered["id"] = value
		}
	}
	return filtered
}
