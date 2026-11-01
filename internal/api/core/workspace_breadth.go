package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/gin-gonic/gin"
)

// WorkspaceBreadth preserves route selectors while expressing aggregate narrowing.
type WorkspaceBreadth struct {
	AllWorkspaces bool
	WorkspaceID   string
	WorkspaceIDs  []string
}

type breadthEncoding struct {
	Selector               string
	OmissionIsAll          bool
	CatalogFiltersArchived bool
}

type workspaceBreadthError struct {
	code    string
	message string
}

func (e *workspaceBreadthError) Error() string { return e.message }

func invalidWorkspaceBreadth(message string) error {
	return &workspaceBreadthError{code: workspaceBreadthInvalidCode, message: message}
}

func parseWorkspaceBreadth(
	c *gin.Context,
	routeParam string,
	enc breadthEncoding,
	workspaces ...WorkspaceGetter,
) (WorkspaceBreadth, error) {
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
	var getter WorkspaceGetter
	if len(workspaces) > 0 {
		getter = workspaces[0]
	}
	selector, err = resolveLegacyGlobalSelector(c, selector, getter)
	if err != nil {
		return WorkspaceBreadth{}, err
	}
	if all && (len(selectors) != 0 || route != "") {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth("all_workspaces conflicts with a workspace selector")
	}
	normalized, err := parseBreadthWorkspaceIDs(c, values["workspace_ids"], all, getter)
	if err != nil {
		return WorkspaceBreadth{}, err
	}
	if !all && selector == "" && !enc.OmissionIsAll {
		return WorkspaceBreadth{}, invalidWorkspaceBreadth(
			fmt.Sprintf("choose %s or all_workspaces=true", enc.Selector),
		)
	}
	if enc.filtersArchived(c, all, selector) {
		normalized, err = defaultActiveWorkspaceIDs(c.Request.Context(), getter, normalized)
		if err != nil {
			return WorkspaceBreadth{}, err
		}
	}
	return WorkspaceBreadth{
		AllWorkspaces: all || selector == "" && enc.OmissionIsAll,
		WorkspaceID:   selector,
		WorkspaceIDs:  normalized,
	}, nil
}

// shouldFilterArchivedWorkspaces narrows default aggregates, preserving explicit history selectors.
func shouldFilterArchivedWorkspaces(c *gin.Context, all bool, selector string, omissionIsAll bool) bool {
	scope := strings.TrimSpace(c.Query("scope"))
	return all || omissionIsAll && strings.TrimSpace(selector) == "" && strings.TrimSpace(c.Query("workspace")) == "" &&
		(scope == "" || scope == queryFilterAllValue)
}

// parseBreadthWorkspaceIDs normalizes the All projects narrowing: sorted, unique, legacy alias resolved.
func parseBreadthWorkspaceIDs(c *gin.Context, ids []string, all bool, getter WorkspaceGetter) ([]string, error) {
	if ids != nil && !all {
		return nil, invalidWorkspaceBreadth("workspace_ids requires all_workspaces=true")
	}
	normalized := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, invalidWorkspaceBreadth("workspace_ids must not be empty; use @user")
		}
		id, err := resolveLegacyGlobalSelector(c, id, getter)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, id)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

func resolveLegacyGlobalSelector(c *gin.Context, value string, workspaces WorkspaceGetter) (string, error) {
	if value != "global" {
		return value, nil
	}
	if workspaces != nil {
		ws, err := workspaces.Get(c.Request.Context(), value)
		if err == nil {
			return ws.ID, nil
		}
		if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) && !errors.Is(err, workspacepkg.ErrWorkspaceReserved) {
			return "", err
		}
	}
	return "", &workspaceBreadthError{
		code:    "workspace_reserved",
		message: "global is reserved without a migration alias",
	}
}

func defaultActiveWorkspaceIDs(ctx context.Context, getter WorkspaceGetter, ids []string) ([]string, error) {
	if len(ids) != 0 {
		return ids, nil
	}
	catalog, ok := getter.(interface {
		List(context.Context) ([]workspacepkg.Workspace, error)
	})
	if !ok {
		return ids, nil
	}
	workspaces, err := catalog.List(ctx)
	if err != nil {
		return nil, err
	}
	active := []string{userScopeWorkspaceRef}
	hasArchived := false
	for _, ws := range workspaces {
		if ws.ArchivedAt != nil {
			hasArchived = true
		} else {
			active = append(active, ws.ID)
		}
	}
	if !hasArchived {
		return ids, nil
	}
	slices.Sort(active)
	return active, nil
}

func (enc breadthEncoding) filtersArchived(c *gin.Context, all bool, selector string) bool {
	return !enc.CatalogFiltersArchived && shouldFilterArchivedWorkspaces(c, all, selector, enc.OmissionIsAll)
}
