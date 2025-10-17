package knowledgerouter

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/knowledge/uc"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/gin-gonic/gin"
)

const (
	defaultKnowledgeLimit = 50
	maxKnowledgeLimit     = 500
)

// listKnowledgeBases handles GET /knowledge-bases.
//
// @Summary List knowledge bases
// @Description List knowledge bases with cursor-based pagination.
// @Tags knowledge
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} router.Response{data=knowledgerouter.KnowledgeBaseListResponse} "Knowledge bases retrieved. Example: {\"status\":200,\"message\":\"knowledge bases retrieved\",\"data\":{\"knowledge_bases\":[{\"id\":\"support\",\"embedder\":\"default-embedder\",\"vector_db\":\"default-vector\",\"ingest\":\"manual\"}],\"page\":{\"limit\":1,\"next_cursor\":\"eyJpZCI6ICJiIn0=\",\"prev_cursor\":\"\"}},\"error\":null}"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Failure 400 {object} core.ProblemDocument "Invalid cursor parameter"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /knowledge-bases [get]
func listKnowledgeBases(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c, c.Query("limit"), defaultKnowledgeLimit, maxKnowledgeLimit)
	cursor, err := router.DecodeCursor(c.Query("cursor"))
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	input := &uc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
	}
	out, err := uc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondKnowledgeError(c, err)
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
	resp := KnowledgeBaseListResponse{
		KnowledgeBases: out.Items,
		Page: httpdto.PageInfoDTO{
			Limit:      limit,
			Total:      out.Total,
			NextCursor: nextCursor,
			PrevCursor: prevCursor,
		},
	}
	router.RespondOK(c, "knowledge bases retrieved", resp)
}

// getKnowledgeBase handles GET /knowledge-bases/{kb_id}.
//
// @Summary Get knowledge base
// @Description Retrieve a knowledge base by ID.
// @Tags knowledge
// @Accept json
// @Produce json
// @Param kb_id path string true "Knowledge base ID" example("support-docs")
// @Param project query string false "Project override" example("demo")
// @Success 200 {object} router.Response{data=knowledgerouter.KnowledgeBaseResponse} "Knowledge base retrieved. Example: {\"status\":200,\"message\":\"knowledge base retrieved\",\"data\":{\"knowledge_base\":{\"id\":\"support\",\"embedder\":\"default-embedder\",\"vector_db\":\"default-vector\",\"ingest\":\"manual\",\"_etag\":\"etag-value\"}},\"error\":null}"
// @Header 200 {string} ETag "Strong entity tag for caching"
// @Failure 400 {object} core.ProblemDocument "Invalid input"
// @Failure 404 {object} core.ProblemDocument "Knowledge base not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /knowledge-bases/{kb_id} [get]
func getKnowledgeBase(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	kbID := router.GetURLParam(c, "kb_id")
	if kbID == "" {
		return
	}
	ifNoneMatch, err := router.ParseStrongETag(c.GetHeader("If-None-Match"))
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid If-None-Match header"})
		return
	}
	out, err := uc.NewGet(store).Execute(c.Request.Context(), &uc.GetInput{Project: project, ID: kbID})
	if err != nil {
		respondKnowledgeError(c, err)
		return
	}
	etag := string(out.ETag)
	if ifNoneMatch != "" && etag == ifNoneMatch {
		c.Header("ETag", strconv.Quote(etag))
		c.Status(http.StatusNotModified)
		return
	}
	payload, err := core.AsMapDefault(out.KnowledgeBase)
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
		return
	}
	payload["_etag"] = etag
	c.Header("ETag", strconv.Quote(etag))
	router.RespondOK(c, "knowledge base retrieved", KnowledgeBaseResponse{KnowledgeBase: payload})
}

// upsertKnowledgeBase handles PUT /knowledge-bases/{kb_id}.
//
// @Summary Create or update knowledge base
// @Description Create a knowledge base when absent or update an existing one using strong ETag concurrency.
// @Tags knowledge
// @Accept json
// @Produce json
// @Param kb_id path string true "Knowledge base ID" example("support-docs")
// @Param project query string false "Project override" example("demo")
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"etag123\"")
// @Param payload body map[string]any true "Knowledge base definition" example({"id":"support","embedder":"default-embedder","vector_db":"default-vector","sources":[{"type":"markdown_glob","path":"docs/**/*.md"}],"description":"Support knowledge base"})
// @Success 200 {object} router.Response{data=knowledgerouter.KnowledgeBaseResponse} "Knowledge base updated. Example: {\"status\":200,\"message\":\"knowledge base updated\",\"data\":{\"knowledge_base\":{\"id\":\"support\",\"embedder\":\"default-embedder\",\"vector_db\":\"default-vector\",\"ingest\":\"manual\",\"description\":\"Support knowledge base\",\"_etag\":\"etag-value\"}},\"error\":null}"
// @Success 201 {object} router.Response{data=knowledgerouter.KnowledgeBaseResponse} "Knowledge base created. Example: {\"status\":201,\"message\":\"knowledge base created\",\"data\":{\"knowledge_base\":{\"id\":\"support\",\"embedder\":\"default-embedder\",\"vector_db\":\"default-vector\",\"ingest\":\"manual\",\"description\":\"Support knowledge base\",\"_etag\":\"etag-value\"}},\"error\":null}"
// @Header 200 {string} ETag "Strong entity tag"
// @Header 201 {string} Location "Relative URL for the knowledge base"
// @Failure 400 {object} core.ProblemDocument "Invalid request body"
// @Failure 404 {object} core.ProblemDocument "Knowledge base not found"
// @Failure 412 {object} core.ProblemDocument "ETag mismatch"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /knowledge-bases/{kb_id} [put]
func upsertKnowledgeBase(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	kbID := router.GetURLParam(c, "kb_id")
	if kbID == "" {
		return
	}
	body := router.GetRequestBody[map[string]any](c)
	if body == nil {
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	input := &uc.UpsertInput{Project: project, ID: kbID, Body: *body, IfMatch: ifMatch}
	out, err := uc.NewUpsert(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondKnowledgeError(c, err)
		return
	}
	etag := string(out.ETag)
	payload := out.KnowledgeBase
	if payload == nil {
		payload = map[string]any{}
	}
	payload["_etag"] = etag
	c.Header("ETag", strconv.Quote(etag))
	if out.Created {
		c.Header("Location", fmt.Sprintf("%s/%s", routes.KnowledgeBases(), kbID))
		router.RespondCreated(c, "knowledge base created", KnowledgeBaseResponse{KnowledgeBase: payload})
		return
	}
	router.RespondOK(c, "knowledge base updated", KnowledgeBaseResponse{KnowledgeBase: payload})
}

// deleteKnowledgeBase handles DELETE /knowledge-bases/{kb_id}.
//
// @Summary Delete knowledge base
// @Description Delete a knowledge base and remove persisted vectors.
// @Tags knowledge
// @Produce json
// @Param kb_id path string true "Knowledge base ID" example("support-docs")
// @Param project query string false "Project override" example("demo")
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"etag123\"")
// @Success 204 {string} string ""
// @Failure 404 {object} core.ProblemDocument "Knowledge base not found"
// @Failure 409 {object} core.ProblemDocument "Knowledge base referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /knowledge-bases/{kb_id} [delete]
func deleteKnowledgeBase(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	kbID := router.GetURLParam(c, "kb_id")
	if kbID == "" {
		return
	}
	if err := uc.NewDelete(store).Execute(c.Request.Context(), &uc.DeleteInput{Project: project, ID: kbID}); err != nil {
		respondKnowledgeError(c, err)
		return
	}
	router.RespondNoContent(c)
}

// ingestKnowledgeBase handles POST /knowledge-bases/{kb_id}/ingest.
//
// @Summary Ingest knowledge base
// @Description Trigger ingestion for configured sources using the requested strategy.
// @Tags knowledge
// @Accept json
// @Produce json
// @Param kb_id path string true "Knowledge base ID" example("support-docs")
// @Param project query string false "Project override" example("demo")
// @Param payload body knowledgerouter.KnowledgeIngestRequest true "Ingestion request" example({"strategy":"replace"})
// @Success 200 {object} router.Response{data=knowledgerouter.KnowledgeIngestResponse} "Ingestion summary. Example: {\"status\":200,\"message\":\"knowledge ingestion completed\",\"data\":{\"knowledge_base_id\":\"support\",\"binding_id\":\"binding-123\",\"documents\":2,\"chunks\":16,\"persisted\":16},\"error\":null}"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 404 {object} core.ProblemDocument "Knowledge base not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /knowledge-bases/{kb_id}/ingest [post]
func ingestKnowledgeBase(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	kbID := router.GetURLParam(c, "kb_id")
	if kbID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	body := router.GetRequestBody[KnowledgeIngestRequest](c)
	if body == nil {
		return
	}
	strategy, err := parseIngestStrategy(body.Strategy)
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
		return
	}
	input := &uc.IngestInput{Project: project, ID: kbID, Strategy: strategy, CWD: state.CWD}
	out, err := uc.NewIngest(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondKnowledgeError(c, err)
		return
	}
	resp := KnowledgeIngestResponse{
		KnowledgeBaseID: out.Result.KnowledgeBaseID,
		BindingID:       out.Result.BindingID,
		Documents:       out.Result.Documents,
		Chunks:          out.Result.Chunks,
		Persisted:       out.Result.Persisted,
	}
	router.RespondOK(c, "knowledge ingestion completed", resp)
}

// queryKnowledgeBase handles POST /knowledge-bases/{kb_id}/query.
//
// @Summary Query knowledge base
// @Description Execute a dense similarity query against a knowledge base.
// @Tags knowledge
// @Accept json
// @Produce json
// @Param kb_id path string true "Knowledge base ID" example("support-docs")
// @Param project query string false "Project override" example("demo")
// @Param payload body knowledgerouter.KnowledgeQueryRequest true "Query request" example({"query":"How do I reset my password?","top_k":3,"min_score":0.4})
// @Success 200 {object} router.Response{data=knowledgerouter.KnowledgeQueryResponse} "Query matches. Example: {\"status\":200,\"message\":\"knowledge query completed\",\"data\":{\"matches\":[{\"binding_id\":\"binding-123\",\"content\":\"Reset your password from the account settings page.\",\"score\":0.83,\"token_estimate\":120}]},\"error\":null}"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 404 {object} core.ProblemDocument "Knowledge base not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /knowledge-bases/{kb_id}/query [post]
func queryKnowledgeBase(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	kbID := router.GetURLParam(c, "kb_id")
	if kbID == "" {
		return
	}
	body := router.GetRequestBody[KnowledgeQueryRequest](c)
	if body == nil {
		return
	}
	input := &uc.QueryInput{
		Project:  project,
		ID:       kbID,
		Query:    body.Query,
		TopK:     body.TopK,
		MinScore: body.MinScore,
		Filters:  body.Filters,
	}
	out, err := uc.NewQuery(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondKnowledgeError(c, err)
		return
	}
	matches := make([]KnowledgeMatch, 0, len(out.Contexts))
	for i := range out.Contexts {
		matches = append(matches, toKnowledgeMatch(out.Contexts[i]))
	}
	router.RespondOK(c, "knowledge query completed", KnowledgeQueryResponse{Matches: matches})
}

func parseIngestStrategy(raw string) (ingest.Strategy, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ingest.StrategyUpsert, nil
	}
	switch ingest.Strategy(trimmed) {
	case ingest.StrategyUpsert, ingest.StrategyReplace:
		return ingest.Strategy(trimmed), nil
	default:
		return ingest.Strategy(""), fmt.Errorf("unsupported strategy %q", raw)
	}
}

func respondKnowledgeError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, uc.ErrInvalidInput),
		errors.Is(err, uc.ErrProjectMissing),
		errors.Is(err, uc.ErrIDMissing),
		errors.Is(err, uc.ErrIDMismatch):
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, uc.ErrValidationFail):
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, uc.ErrNotFound):
		core.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, uc.ErrAlreadyExists):
		core.RespondProblem(c, &core.Problem{Status: http.StatusConflict, Detail: err.Error()})
	case errors.Is(err, uc.ErrETagMismatch), errors.Is(err, uc.ErrStaleIfMatch):
		core.RespondProblem(c, &core.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			resourceutil.RespondConflict(c, err, conflict.Details)
			return
		}
		core.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}
