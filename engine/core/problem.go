package core

import (
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ProblemDocument models an RFC 7807 error envelope for API responses.
type ProblemDocument struct {
	Type     string `json:"type,omitempty"     example:"about:blank"`
	Title    string `json:"title"              example:"Bad Request"`
	Status   int    `json:"status"             example:"400"`
	Detail   string `json:"detail,omitempty"   example:"Invalid cursor parameter"`
	Instance string `json:"instance,omitempty" example:"/api/v0/workflows"`
	Code     string `json:"code,omitempty"     example:"invalid_cursor"`
}

type Problem struct {
	Type     string
	Title    string
	Status   int
	Detail   string
	Instance string
	Extras   map[string]any
}

// RespondProblem writes an RFC7807 problem+json response.
// Accepts a pointer to avoid copying a potentially large struct and to satisfy linters.
func RespondProblem(c *gin.Context, problem *Problem) {
	prepared := ensureProblemDefaults(problem)
	body := buildProblemBody(prepared)
	writeProblemResponse(c, prepared, body)
}

// ensureProblemDefaults normalizes a problem payload before rendering.
func ensureProblemDefaults(problem *Problem) *Problem {
	if problem == nil {
		problem = &Problem{}
	}
	if problem.Status == 0 {
		problem.Status = http.StatusInternalServerError
	}
	if problem.Title == "" {
		problem.Title = http.StatusText(problem.Status)
	}
	return problem
}

// buildProblemBody assembles the RFC7807 payload.
func buildProblemBody(problem *Problem) gin.H {
	body := gin.H{
		"status": problem.Status,
		"title":  problem.Title,
	}
	if problem.Type != "" {
		body["type"] = problem.Type
	}
	if problem.Detail != "" {
		body["detail"] = problem.Detail
	}
	if problem.Instance != "" {
		body["instance"] = problem.Instance
	}
	for key, value := range problem.Extras {
		if !isReservedProblemKey(key) {
			body[key] = value
		}
	}
	return body
}

// isReservedProblemKey reports whether a key is already populated by the problem envelope.
func isReservedProblemKey(key string) bool {
	switch key {
	case "status", "title", "type", "detail", "instance":
		return true
	default:
		return false
	}
}

// writeProblemResponse writes headers, logs, and JSON body for a problem response.
func writeProblemResponse(c *gin.Context, problem *Problem, body gin.H) {
	c.Header("Content-Type", "application/problem+json")
	logProblem(c, problem)
	c.JSON(problem.Status, body)
}

// logProblem reports problem responses using the context logger.
func logProblem(c *gin.Context, problem *Problem) {
	log := logger.FromContext(c.Request.Context())
	fields := []any{
		"status", problem.Status,
		"title", problem.Title,
		"detail", problem.Detail,
		"path", c.FullPath(),
	}
	if problem.Status >= http.StatusInternalServerError {
		log.Error("request failed", fields...)
		return
	}
	log.Warn("request failed", fields...)
}

func RespondProblemWithCode(c *gin.Context, status int, code string, detail string) {
	problem := Problem{
		Status: status,
		Title:  http.StatusText(status),
		Detail: detail,
		Extras: map[string]any{"code": code},
	}
	RespondProblem(c, &problem)
}
