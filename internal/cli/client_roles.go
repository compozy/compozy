package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// RoleRecord is one effective background-role configuration projection.
type RoleRecord = contract.RoleStatus

// RolesClient is the focused daemon surface for background-role inspection.
type RolesClient interface {
	ListRoles(context.Context, RoleQuery) ([]RoleRecord, error)
	GetRole(context.Context, string, RoleQuery) (RoleRecord, error)
}

// RoleQuery scopes role resolution to a workspace when supplied.
type RoleQuery struct {
	Workspace string
}

func (c *unixSocketClient) ListRoles(ctx context.Context, query RoleQuery) ([]RoleRecord, error) {
	var response contract.RolesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/roles", roleValues(query), nil, &response); err != nil {
		return nil, fmt.Errorf("cli: list roles: %w", err)
	}
	return response.Roles, nil
}

func (c *unixSocketClient) GetRole(ctx context.Context, role string, query RoleQuery) (RoleRecord, error) {
	var response contract.RoleStatusResponse
	path := "/api/roles/" + url.PathEscape(strings.TrimSpace(role))
	if err := c.doJSON(ctx, http.MethodGet, path, roleValues(query), nil, &response); err != nil {
		return RoleRecord{}, fmt.Errorf("cli: get role: %w", err)
	}
	return response.Role, nil
}

func roleValues(query RoleQuery) url.Values {
	values := url.Values{}
	if workspace := strings.TrimSpace(query.Workspace); workspace != "" {
		values.Set("workspace", workspace)
	}
	return values
}
