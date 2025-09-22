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
	if problem.Status == 0 {
		problem.Status = http.StatusInternalServerError
	}
	if problem.Title == "" {
		problem.Title = http.StatusText(problem.Status)
	}
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
	for k, v := range problem.Extras {
		if k != "status" && k != "title" && k != "type" && k != "detail" && k != "instance" {
			body[k] = v
		}
	}
	c.Header("Content-Type", "application/problem+json")
	log := logger.FromContext(c.Request.Context())
	if problem.Status >= http.StatusInternalServerError {
		log.Error(
			"request failed",
			"status",
			problem.Status,
			"title",
			problem.Title,
			"detail",
			problem.Detail,
			"path",
			c.FullPath(),
		)
	} else {
		log.Warn(
			"request failed",
			"status",
			problem.Status,
			"title",
			problem.Title,
			"detail",
			problem.Detail,
			"path",
			c.FullPath(),
		)
	}
	c.JSON(problem.Status, body)
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
