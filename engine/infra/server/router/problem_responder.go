package router

import (
	"encoding/json"
	"net/http"

	"github.com/compozy/compozy/engine/core"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// RespondProblem writes a canonical RFC 7807 error response.
func RespondProblem(c *gin.Context, problem *core.Problem) {
	prepared := core.NormalizeProblem(problem)
	body := core.BuildProblemBody(prepared)
	writeProblemResponse(c, prepared, body)
}

// RespondProblemWithCode writes a problem response embedding a code and detail.
func RespondProblemWithCode(c *gin.Context, status int, code string, detail string) {
	RespondProblem(c, &core.Problem{
		Status: status,
		Title:  http.StatusText(status),
		Detail: detail,
		Extras: map[string]any{"code": code},
	})
}

// RespondConflict writes a conflict response including contextual references.
func RespondConflict(c *gin.Context, err error, details []resourceutil.ReferenceDetail) {
	RespondProblem(c, resourceutil.BuildConflictProblem(err, details))
}

func writeProblemResponse(c *gin.Context, problem *core.Problem, body map[string]any) {
	logProblem(c, problem)
	payload, err := json.Marshal(body)
	if err != nil {
		logger.FromContext(c.Request.Context()).Error("failed to marshal problem", "err", err)
		fallback := []byte(`{"status":500,"error":"Internal Server Error"}`)
		c.Data(http.StatusInternalServerError, "application/problem+json", fallback)
		c.Abort()
		return
	}
	c.Data(problem.Status, "application/problem+json", payload)
	c.Abort()
}

func logProblem(c *gin.Context, problem *core.Problem) {
	log := logger.FromContext(c.Request.Context())
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	fields := []any{
		"status", problem.Status,
		"title", problem.Title,
		"detail", problem.Detail,
		"route", route,
		"path", c.Request.URL.Path,
	}
	if problem.Instance != "" {
		fields = append(fields, "instance", problem.Instance)
	}
	if code, ok := problem.Extras["code"]; ok {
		fields = append(fields, "code", code)
	}
	if correlationID := c.Request.Header.Get("X-Correlation-ID"); correlationID != "" {
		fields = append(fields, "correlation_id", correlationID)
	} else if requestID := c.Request.Header.Get("X-Request-ID"); requestID != "" {
		fields = append(fields, "request_id", requestID)
	} else if correlationID := c.Writer.Header().Get("X-Correlation-ID"); correlationID != "" {
		fields = append(fields, "correlation_id", correlationID)
	} else if requestID := c.Writer.Header().Get("X-Request-ID"); requestID != "" {
		fields = append(fields, "request_id", requestID)
	}
	if problem.Status >= http.StatusInternalServerError {
		log.Error("request failed", fields...)
		return
	}
	log.Warn("request failed", fields...)
}
