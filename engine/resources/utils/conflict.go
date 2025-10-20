package resourceutil

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/core"
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
	totalRefs := 0
	for i := range e.Details {
		r := strings.TrimSpace(e.Details[i].Resource)
		totalRefs += len(e.Details[i].IDs)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		types = append(types, r)
	}
	if totalRefs == 0 {
		totalRefs = len(types)
		if totalRefs == 0 {
			totalRefs = len(e.Details)
		}
	}
	label := "collections"
	if totalRefs == 1 {
		label = "collection"
	}
	if len(types) == 0 {
		return fmt.Sprintf("resource referenced by %d %s", totalRefs, label)
	}
	sort.Strings(types)
	return fmt.Sprintf("resource referenced by %d %s: %s", totalRefs, label, strings.Join(types, ", "))
}

// BuildConflictProblem constructs a problem payload representing a conflict error.
func BuildConflictProblem(err error, details []ReferenceDetail) *core.Problem {
	var extras map[string]any
	if len(details) > 0 {
		references := make([]map[string]any, 0, len(details))
		for i := range details {
			references = append(references, map[string]any{
				"resource": strings.TrimSpace(details[i].Resource),
				"ids":      details[i].IDs,
			})
		}
		extras = map[string]any{"references": references}
	}
	detail := "resource has active references"
	if err != nil {
		if trimmed := strings.TrimSpace(err.Error()); trimmed != "" {
			detail = trimmed
		}
	}
	problem := &core.Problem{
		Status: http.StatusConflict,
		Title:  http.StatusText(http.StatusConflict),
		Detail: detail,
	}
	if extras != nil {
		problem.Extras = extras
	}
	return core.NormalizeProblem(problem)
}
