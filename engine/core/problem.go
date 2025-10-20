package core

import (
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ProblemDocument models the canonical error envelope for API responses.
type ProblemDocument struct {
	Status   int    `json:"status"             example:"400"`
	Error    string `json:"error"              example:"Bad Request"`
	Details  string `json:"details,omitempty"  example:"Invalid cursor parameter"`
	Code     string `json:"code,omitempty"     example:"invalid_cursor"`
	Type     string `json:"type,omitempty"     example:"about:blank"`
	Instance string `json:"instance,omitempty" example:"/api/v0/workflows"`
}

type Problem struct {
	Type     string
	Title    string
	Status   int
	Detail   string
	Instance string
	Extras   map[string]any
}

// RespondProblem writes a canonical JSON error response.
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
	if problem.Type == "" {
		problem.Type = "about:blank"
	}
	return problem
}

// buildProblemBody assembles the error payload.
func buildProblemBody(problem *Problem) gin.H {
	body := gin.H{
		"status": problem.Status,
		"error":  problem.Title,
	}
	if problem.Detail != "" {
		body["details"] = problem.Detail
	}
	if code, ok := problem.Extras["code"]; ok {
		body["code"] = code
	}
	if problem.Type != "" {
		body["type"] = problem.Type
	}
	if problem.Instance != "" {
		body["instance"] = problem.Instance
	}
	filteredExtras := make(map[string]any)
	for key, value := range problem.Extras {
		if !isReservedProblemKey(key) {
			filteredExtras[key] = value
		}
	}
	if len(filteredExtras) == 0 {
		return body
	}
	merged := CopyMaps(map[string]any(body), filteredExtras)
	return gin.H(merged)
}

// isReservedProblemKey reports whether a key is already populated by the problem envelope.
func isReservedProblemKey(key string) bool {
	switch key {
	case "status", "error", "details", "code", "type", "instance":
		return true
	default:
		return false
	}
}

// writeProblemResponse writes headers, logs, and JSON body for a problem response.
func writeProblemResponse(c *gin.Context, problem *Problem, body gin.H) {
	c.Header("Content-Type", "application/json")
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
