package auth

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
)

// PermissionService handles role-based permission checking and validation
type PermissionService struct {
	userService *user.Service
}

// NewPermissionService creates a new permission service instance
func NewPermissionService(userService *user.Service) *PermissionService {
	return &PermissionService{
		userService: userService,
	}
}

// CheckPermission verifies if a user has a specific permission
func (p *PermissionService) CheckPermission(
	ctx context.Context,
	orgID, userID core.ID,
	permission string,
) (bool, error) {
	return p.userService.CheckPermission(ctx, orgID, userID, permission)
}

// RequirePermission checks permission and returns an error if not granted
func (p *PermissionService) RequirePermission(ctx context.Context, orgID, userID core.ID, permission string) error {
	hasPermission, err := p.CheckPermission(ctx, orgID, userID, permission)
	if err != nil {
		return fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return ErrPermissionDenied
	}
	return nil
}

// CheckAnyPermission verifies if a user has any of the specified permissions
func (p *PermissionService) CheckAnyPermission(
	ctx context.Context,
	orgID, userID core.ID,
	permissions ...string,
) (bool, error) {
	for _, permission := range permissions {
		hasPermission, err := p.CheckPermission(ctx, orgID, userID, permission)
		if err != nil {
			return false, fmt.Errorf("failed to check permission %s: %w", permission, err)
		}
		if hasPermission {
			return true, nil
		}
	}
	return false, nil
}

// CheckAllPermissions verifies if a user has all of the specified permissions
func (p *PermissionService) CheckAllPermissions(
	ctx context.Context,
	orgID, userID core.ID,
	permissions ...string,
) (bool, error) {
	for _, permission := range permissions {
		hasPermission, err := p.CheckPermission(ctx, orgID, userID, permission)
		if err != nil {
			return false, fmt.Errorf("failed to check permission %s: %w", permission, err)
		}
		if !hasPermission {
			return false, nil
		}
	}
	return true, nil
}

// GetUserPermissions returns all permissions for a user based on their role
func (p *PermissionService) GetUserPermissions(ctx context.Context, orgID, userID core.ID) ([]string, error) {
	// Get user to determine their role
	u, err := p.userService.GetUser(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Check if user is active
	if u.Status != user.StatusActive {
		return []string{}, nil
	}
	// Return permissions based on role
	return p.getPermissionsForRole(u.Role), nil
}

// getPermissionsForRole returns all permissions for a given role
func (p *PermissionService) getPermissionsForRole(role user.Role) []string {
	switch role {
	case user.RoleSystemAdmin:
		// System admins have all permissions
		return []string{
			user.PermSystemManage,
			user.PermWorkflowRead, user.PermWorkflowWrite, user.PermWorkflowExecute, user.PermWorkflowDelete,
			user.PermTaskRead, user.PermTaskWrite, user.PermTaskExecute, user.PermTaskDelete,
			user.PermUserRead, user.PermUserWrite,
			user.PermAPIKeyRead, user.PermAPIKeyWrite,
		}
	case user.RoleOrgAdmin:
		// Org admins have all permissions except system management
		return []string{
			user.PermWorkflowRead, user.PermWorkflowWrite, user.PermWorkflowExecute, user.PermWorkflowDelete,
			user.PermTaskRead, user.PermTaskWrite, user.PermTaskExecute, user.PermTaskDelete,
			user.PermUserRead, user.PermUserWrite,
			user.PermAPIKeyRead, user.PermAPIKeyWrite,
		}
	case user.RoleOrgManager:
		// Org managers have management permissions within their organization
		// They can manage workflows and tasks but have limited user management
		return []string{
			user.PermWorkflowRead, user.PermWorkflowWrite, user.PermWorkflowExecute, user.PermWorkflowDelete,
			user.PermTaskRead, user.PermTaskWrite, user.PermTaskExecute, user.PermTaskDelete,
			user.PermUserRead, // Can read users but not write
			user.PermAPIKeyRead, user.PermAPIKeyWrite,
		}
	case user.RoleOrgCustomer:
		// Org customers have limited permissions
		return []string{
			user.PermWorkflowRead, user.PermWorkflowExecute,
			user.PermTaskRead, user.PermTaskExecute,
		}
	default:
		return []string{}
	}
}

// CanManageUser checks if one user can manage another user
func (p *PermissionService) CanManageUser(ctx context.Context, orgID, managerID, targetUserID core.ID) (bool, error) {
	// Get both users
	manager, err := p.userService.GetUser(ctx, orgID, managerID)
	if err != nil {
		return false, fmt.Errorf("failed to get manager: %w", err)
	}
	targetUser, err := p.userService.GetUser(ctx, orgID, targetUserID)
	if err != nil {
		return false, fmt.Errorf("failed to get target user: %w", err)
	}
	// Check if manager is active
	if manager.Status != user.StatusActive {
		return false, nil
	}
	// System admins can manage anyone
	if manager.Role == user.RoleSystemAdmin {
		return true, nil
	}
	// Org admins can manage anyone in their organization
	if manager.Role == user.RoleOrgAdmin {
		return true, nil
	}
	// Org managers can manage customers but not admins or other managers
	if manager.Role == user.RoleOrgManager {
		return targetUser.Role == user.RoleOrgCustomer, nil
	}
	// Customers cannot manage anyone
	return false, nil
}
