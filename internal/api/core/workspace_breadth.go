package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// WorkspaceBreadth preserves route selectors while expressing aggregate narrowing.
type WorkspaceBreadth struct {
	AllWorkspaces bool
	WorkspaceID   string
	WorkspaceIDs  []string
}

type breadthEncoding struct {
	Selector      string
	OmissionIsAll bool
}

type workspaceBreadthError struct {
	code    string
	message string
}

func (e *workspaceBreadthError) Error() string { return e.message }

func invalidWorkspaceBreadth(message string) error {
	return &workspaceBreadthError{code: "workspace_breadth_invalid", message: message}
}

func parseWorkspaceBreadth(c *gin.Context, routeParam string, enc breadthEncoding) (WorkspaceBreadth, error) {
	values := c.Request.URL.Query()
	allValues := values["all_workspaces"]
	if len(allValues) > 1 {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth("all_workspaces must be specified once")
	}
	all, err := ParseOptionalBool(c.Query("all_workspaces"))
	if err != nil {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth("all_workspaces must be a boolean")
	}
	selectors := values[enc.Selector]
	if len(selectors) > 1 {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth(enc.Selector + " must be specified once")
	}
	selector := strings.TrimSpace(c.Query(enc.Selector))
	route := strings.TrimSpace(routeParam)
	if route != "" {
		if selector != "" && selector != route {
			return WorkspaceBreadth{}, invalidWorkspaceBreadth("conflicting workspace selectors")
		}
		selector = route
	}
	if selector == "global" {
		return WorkspaceBreadth{}, &workspaceBreadthError{code: "workspace_reserved", message: "global is reserved; use all_workspaces=true"}
	}
	if all && (len(selectors) != 0 || route != "") {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth("all_workspaces conflicts with a workspace selector")
	}
	ids, hasIDs := values["workspace_ids"]
	if hasIDs && !all {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth("workspace_ids requires all_workspaces=true")
	}
	normalized := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return WorkspaceBreadth{}, invalidWorkspaceBreadth("workspace_ids must not be empty; use @user")
		}
		if id == "global" {
			return WorkspaceBreadth{}, &workspaceBreadthError{code: "workspace_reserved", message: "global is reserved; use @user for User rows"}
		}
		normalized = append(normalized, id)
	}
	slices.Sort(normalized)
	normalized = slices.Compact(normalized)
	if !all && selector == "" && !enc.OmissionIsAll {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth(fmt.Sprintf("choose %s or all_workspaces=true", enc.Selector))
	}
	return WorkspaceBreadth{AllWorkspaces: all || selector == "" && enc.OmissionIsAll, WorkspaceID: selector, WorkspaceIDs: normalized}, nil
}
