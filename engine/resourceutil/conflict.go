package resourceutil

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/core"
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
	sort.Strings(types)
	return fmt.Sprintf("resource referenced by %d collections: %s", len(e.Details), strings.Join(types, ", "))
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
	var detail string
	if err != nil {
		detail = strings.TrimSpace(err.Error())
	}
	if detail == "" {
		detail = "resource has active references"
	}
	core.RespondProblem(c, &core.Problem{Status: http.StatusConflict, Detail: detail, Extras: extras})
}
