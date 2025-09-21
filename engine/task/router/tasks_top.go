package tkrouter

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/resourceutil"
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/gin-gonic/gin"
)

// listTasksTop handles GET /tasks.
//
// @Summary List tasks
// @Description List tasks with cursor pagination. Optionally filter by workflow usage.
// @Tags tasks
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param workflow_id query string false "Return only tasks referenced by the given workflow" example("wf1")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by task ID prefix"
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=object{tasks=[]map[string]any,page=object}} "Tasks retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid cursor"
// @Failure 404 {object} router.ProblemDocument "Workflow not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tasks [get]
func listTasksTop(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c.Query("limit"), 50, 500)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	input := &taskuc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
		WorkflowID:      strings.TrimSpace(c.Query("workflow_id")),
	}
	out, err := taskuc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondTaskError(c, err)
		return
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
	items := make([]map[string]any, 0, len(out.Items))
	for i := range out.Items {
		filtered := router.FilterMapFields(out.Items[i], fields)
		if len(fields) != 0 && !fields["_etag"] {
			filtered["_etag"] = out.Items[i]["_etag"]
		}
		items = append(items, filtered)
	}
	page := map[string]any{"limit": limit, "total": out.Total}
	if nextCursor != "" {
		page["next_cursor"] = nextCursor
	}
	if prevCursor != "" {
		page["prev_cursor"] = prevCursor
	}
	router.RespondOK(c, "tasks retrieved", gin.H{"tasks": items, "page": page})
}

// getTaskTop handles GET /tasks/{task_id}.
//
// @Summary Get task
// @Description Retrieve a task configuration by ID.
// @Tags tasks
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID" example("approve-request")
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=map[string]any} "Task retrieved"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid input"
// @Failure 404 {object} router.ProblemDocument "Task not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tasks/{task_id} [get]
func getTaskTop(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
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
	fields := router.ParseFieldsQuery(c.Query("fields"))
	out, err := taskuc.NewGet(store).Execute(c.Request.Context(), &taskuc.GetInput{Project: project, ID: taskID})
	if err != nil {
		respondTaskError(c, err)
		return
	}
	filtered := router.FilterMapFields(out.Task, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Task["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	router.RespondOK(c, "task retrieved", filtered)
}

// upsertTaskTop handles PUT /tasks/{task_id}.
//
// @Summary Create or update task
// @Description Create a task configuration when absent or update an existing one using strong ETag concurrency.
// @Tags tasks
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID" example("approve-request")
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Task configuration payload"
// @Success 200 {object} router.Response{data=map[string]any} "Task updated"
// @Success 201 {object} router.Response{data=map[string]any} "Task created"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the task"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid request"
// @Failure 404 {object} router.ProblemDocument "Task not found"
// @Failure 409 {object} router.ProblemDocument "Task referenced"
// @Failure 412 {object} router.ProblemDocument "ETag mismatch"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tasks/{task_id} [put]
func upsertTaskTop(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
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
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	input := &taskuc.UpsertInput{Project: project, ID: taskID, Body: body, IfMatch: ifMatch}
	out, execErr := taskuc.NewUpsert(store).Execute(c.Request.Context(), input)
	if execErr != nil {
		respondTaskError(c, execErr)
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	filtered := router.FilterMapFields(out.Task, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Task["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	status := http.StatusOK
	message := "task updated"
	if out.Created {
		status = http.StatusCreated
		message = "task created"
		c.Header("Location", routes.Tasks()+"/"+taskID)
	}
	if status == http.StatusCreated {
		router.RespondCreated(c, message, filtered)
		return
	}
	router.RespondOK(c, message, filtered)
}

// deleteTaskTop handles DELETE /tasks/{task_id}.
//
// @Summary Delete task
// @Description Delete a task configuration. Returns conflict when referenced.
// @Tags tasks
// @Produce json
// @Param task_id path string true "Task ID" example("approve-request")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Failure 404 {object} router.ProblemDocument "Task not found"
// @Failure 409 {object} router.ProblemDocument "Task referenced"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tasks/{task_id} [delete]
func deleteTaskTop(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
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
	deleteInput := &taskuc.DeleteInput{Project: project, ID: taskID}
	if err := taskuc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondTaskError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondTaskError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, taskuc.ErrInvalidInput),
		errors.Is(err, taskuc.ErrProjectMissing),
		errors.Is(err, taskuc.ErrIDMissing):
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, taskuc.ErrNotFound):
		router.RespondProblem(c, router.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, taskuc.ErrETagMismatch),
		errors.Is(err, taskuc.ErrStaleIfMatch):
		router.RespondProblem(c, router.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	case errors.Is(err, taskuc.ErrWorkflowNotFound):
		router.RespondProblem(c, router.Problem{Status: http.StatusNotFound, Detail: "workflow not found"})
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			respondConflict(c, err, conflict.Details)
			return
		}
		router.RespondProblem(c, router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}

func respondConflict(c *gin.Context, err error, details []resourceutil.ReferenceDetail) {
	extras := map[string]any{}
	if len(details) > 0 {
		refs := make([]map[string]any, 0, len(details))
		for i := range details {
			d := map[string]any{"resource": details[i].Resource, "ids": details[i].IDs}
			refs = append(refs, d)
		}
		extras["references"] = refs
	}
	detail := err.Error()
	if detail == "" {
		detail = "resource has active references"
	}
	router.RespondProblem(c, router.Problem{Status: http.StatusConflict, Detail: detail, Extras: extras})
}
