package user

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
)

var (
	// emailRegex matches valid email addresses
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// Role represents the user's role within an organization
type Role string

const (
	// RoleSystemAdmin has global permissions across all organizations
	RoleSystemAdmin Role = "system_admin"
	// RoleOrgAdmin has full permissions within their organization
	RoleOrgAdmin Role = "org_admin"
	// RoleOrgManager has management permissions within their organization
	RoleOrgManager Role = "org_manager"
	// RoleOrgCustomer has read/execute permissions within their organization
	RoleOrgCustomer Role = "org_customer"
)

// Permission constants for RBAC
const (
	// System permissions
	PermSystemManage = "system:manage"
	// Workflow permissions
	PermWorkflowRead    = "workflow:read"
	PermWorkflowWrite   = "workflow:write"
	PermWorkflowExecute = "workflow:execute"
	PermWorkflowDelete  = "workflow:delete"
	// Task permissions
	PermTaskRead    = "task:read"
	PermTaskWrite   = "task:write"
	PermTaskExecute = "task:execute"
	PermTaskDelete  = "task:delete"
	// User permissions
	PermUserRead  = "user:read"
	PermUserWrite = "user:write"
	// API Key permissions
	PermAPIKeyRead  = "apikey:read"
	PermAPIKeyWrite = "apikey:write"
)

// IsValid checks if the role is valid
func (r Role) IsValid() bool {
	switch r {
	case RoleSystemAdmin, RoleOrgAdmin, RoleOrgManager, RoleOrgCustomer:
		return true
	default:
		return false
	}
}

// HasPermission checks if the role has a specific permission
func (r Role) HasPermission(permission string) bool {
	switch r {
	case RoleSystemAdmin:
		// System admins have all permissions
		return true
	case RoleOrgAdmin:
		// Org admins have all permissions within their organization
		return permission != PermSystemManage
	case RoleOrgManager:
		// Org managers have management permissions within their organization
		switch permission {
		case PermWorkflowRead, PermWorkflowWrite, PermWorkflowExecute, PermWorkflowDelete,
			PermTaskRead, PermTaskWrite, PermTaskExecute, PermTaskDelete,
			PermUserRead, PermUserWrite, PermAPIKeyRead, PermAPIKeyWrite:
			return true
		default:
			return false
		}
	case RoleOrgCustomer:
		// Org customers have limited permissions
		switch permission {
		case PermWorkflowRead, PermWorkflowExecute, PermTaskRead, PermTaskExecute:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// Status represents the status of a user
type Status string

const (
	// StatusActive indicates the user is active
	StatusActive Status = "active"
	// StatusSuspended indicates the user is suspended
	StatusSuspended Status = "suspended"
)

// IsValid checks if the user status is valid
func (s Status) IsValid() bool {
	switch s {
	case StatusActive, StatusSuspended:
		return true
	default:
		return false
	}
}

// User represents a user in the multi-tenant system
type User struct {
	ID        core.ID   `json:"id"         db:"id"`
	OrgID     core.ID   `json:"org_id"     db:"org_id"`
	Email     string    `json:"email"      db:"email"`
	Role      Role      `json:"role"       db:"role"`
	Status    Status    `json:"status"     db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewUser creates a new user with the given details
func NewUser(orgID core.ID, email string, role Role) (*User, error) {
	// Normalize email before validation
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if err := ValidateEmail(normalizedEmail); err != nil {
		return nil, err
	}
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role: %s", role)
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization ID cannot be empty")
	}
	now := time.Now().UTC()
	id, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user ID: %w", err)
	}
	return &User{
		ID:        id,
		OrgID:     orgID,
		Email:     normalizedEmail,
		Role:      role,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if len(email) > 255 {
		return fmt.Errorf("email must be at most 255 characters long")
	}
	// Basic email validation
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// Validate validates the user entity
func (u *User) Validate() error {
	if u.ID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if u.OrgID == "" {
		return fmt.Errorf("organization ID cannot be empty")
	}
	if err := ValidateEmail(u.Email); err != nil {
		return err
	}
	if !u.Role.IsValid() {
		return fmt.Errorf("invalid role: %s", u.Role)
	}
	if !u.Status.IsValid() {
		return fmt.Errorf("invalid user status: %s", u.Status)
	}
	return nil
}

// IsActive returns true if the user is active
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// IsSuspended returns true if the user is suspended
func (u *User) IsSuspended() bool {
	return u.Status == StatusSuspended
}

// Suspend suspends the user
func (u *User) Suspend() {
	u.Status = StatusSuspended
	u.UpdatedAt = time.Now().UTC()
}

// Activate activates the user
func (u *User) Activate() {
	u.Status = StatusActive
	u.UpdatedAt = time.Now().UTC()
}

// UpdateRole updates the user's role
func (u *User) UpdateRole(role Role) error {
	if !role.IsValid() {
		return fmt.Errorf("invalid role: %s", role)
	}
	u.Role = role
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// HasPermission checks if the user has a specific permission
func (u *User) HasPermission(permission string) bool {
	if !u.IsActive() {
		return false
	}
	return u.Role.HasPermission(permission)
}

// IsSystemAdmin returns true if the user is a system admin
func (u *User) IsSystemAdmin() bool {
	return u.Role == RoleSystemAdmin
}

// IsOrgAdmin returns true if the user is an org admin
func (u *User) IsOrgAdmin() bool {
	return u.Role == RoleOrgAdmin
}

// IsOrgManager returns true if the user is an org manager
func (u *User) IsOrgManager() bool {
	return u.Role == RoleOrgManager
}

// IsOrgCustomer returns true if the user is an org customer
func (u *User) IsOrgCustomer() bool {
	return u.Role == RoleOrgCustomer
}
