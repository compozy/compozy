package resourceutil

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ReferenceDetail describes a resource reference that prevents deletion.
// It specifies the referencing resource type and the set of IDs that refer to
// the target resource.
type ReferenceDetail struct {
	Resource string
	IDs      []string
}

// ConflictError represents a deletion conflict due to existing references.
// It aggregates all reference details to produce an informative error message.
type ConflictError struct {
	Details []ReferenceDetail
}

func (e ConflictError) Error() string {
	if len(e.Details) == 0 {
		return "resource has conflicting references"
	}
	types := make([]string, 0, len(e.Details))
	seen := make(map[string]bool, len(e.Details))
	for i := range e.Details {
		r := strings.TrimSpace(e.Details[i].Resource)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		types = append(types, r)
	}
	if len(types) == 0 {
		return fmt.Sprintf("resource referenced by %d collections", len(e.Details))
	}
	return fmt.Sprintf("resource referenced by %d collections: %s", len(e.Details), strings.Join(types, ", "))
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

func RespondConflict(c *gin.Context, err error, details []ReferenceDetail) {
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
	RespondProblem(c, &Problem{Status: http.StatusConflict, Detail: detail, Extras: extras})
}
