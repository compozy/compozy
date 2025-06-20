package user

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want bool
	}{
		{
			name: "Should validate system admin role",
			role: RoleSystemAdmin,
			want: true,
		},
		{
			name: "Should validate org admin role",
			role: RoleOrgAdmin,
			want: true,
		},
		{
			name: "Should validate org manager role",
			role: RoleOrgManager,
			want: true,
		},
		{
			name: "Should validate org customer role",
			role: RoleOrgCustomer,
			want: true,
		},
		{
			name: "Should reject invalid role",
			role: Role("invalid"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.role.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRole_HasPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		permission string
		want       bool
	}{
		// System admin tests
		{
			name:       "System admin should have system:manage permission",
			role:       RoleSystemAdmin,
			permission: PermSystemManage,
			want:       true,
		},
		{
			name:       "System admin should have any permission",
			role:       RoleSystemAdmin,
			permission: "anything:else",
			want:       true,
		},
		// Org admin tests
		{
			name:       "Org admin should have workflow:read permission",
			role:       RoleOrgAdmin,
			permission: PermWorkflowRead,
			want:       true,
		},
		{
			name:       "Org admin should have user:manage permission",
			role:       RoleOrgAdmin,
			permission: "user:manage",
			want:       true,
		},
		{
			name:       "Org admin should not have system:manage permission",
			role:       RoleOrgAdmin,
			permission: PermSystemManage,
			want:       false,
		},
		// Org member tests
		{
			name:       "Org member should have workflow:read permission",
			role:       RoleOrgCustomer,
			permission: PermWorkflowRead,
			want:       true,
		},
		{
			name:       "Org member should have workflow:execute permission",
			role:       RoleOrgCustomer,
			permission: PermWorkflowExecute,
			want:       true,
		},
		{
			name:       "Org member should have task:read permission",
			role:       RoleOrgCustomer,
			permission: PermTaskRead,
			want:       true,
		},
		{
			name:       "Org member should have task:execute permission",
			role:       RoleOrgCustomer,
			permission: PermTaskExecute,
			want:       true,
		},
		{
			name:       "Org member should not have user:manage permission",
			role:       RoleOrgCustomer,
			permission: "user:manage",
			want:       false,
		},
		{
			name:       "Org member should not have workflow:create permission",
			role:       RoleOrgCustomer,
			permission: "workflow:create",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.role.HasPermission(tt.permission)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "Should validate active status",
			status: StatusActive,
			want:   true,
		},
		{
			name:   "Should validate suspended status",
			status: StatusSuspended,
			want:   true,
		},
		{
			name:   "Should reject invalid status",
			status: Status("invalid"),
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewUser(t *testing.T) {
	orgID := core.MustNewID()
	t.Run("Should create new user with valid details", func(t *testing.T) {
		user, err := NewUser(orgID, "john@example.com", RoleOrgCustomer)
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.NotEmpty(t, user.ID)
		assert.Equal(t, orgID, user.OrgID)
		assert.Equal(t, "john@example.com", user.Email)
		assert.Equal(t, RoleOrgCustomer, user.Role)
		assert.Equal(t, StatusActive, user.Status)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())
	})
	t.Run("Should normalize email to lowercase", func(t *testing.T) {
		user, err := NewUser(orgID, "John@EXAMPLE.COM", RoleOrgCustomer)
		require.NoError(t, err)
		assert.Equal(t, "john@example.com", user.Email)
	})
	t.Run("Should trim email whitespace", func(t *testing.T) {
		user, err := NewUser(orgID, "  john@example.com  ", RoleOrgCustomer)
		require.NoError(t, err)
		assert.Equal(t, "john@example.com", user.Email)
	})
	t.Run("Should reject invalid email", func(t *testing.T) {
		user, err := NewUser(orgID, "invalid-email", RoleOrgCustomer)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "invalid email format")
	})
	t.Run("Should reject empty org ID", func(t *testing.T) {
		user, err := NewUser("", "john@example.com", RoleOrgCustomer)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "organization ID cannot be empty")
	})
	t.Run("Should reject invalid role", func(t *testing.T) {
		user, err := NewUser(orgID, "john@example.com", Role("invalid"))
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "invalid role")
	})
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Should accept valid email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "Should accept email with dots",
			email:   "user.name@example.com",
			wantErr: false,
		},
		{
			name:    "Should accept email with plus",
			email:   "user+tag@example.com",
			wantErr: false,
		},
		{
			name:    "Should accept email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "Should reject empty email",
			email:   "",
			wantErr: true,
			errMsg:  "email cannot be empty",
		},
		{
			name:    "Should reject email without @",
			email:   "userexample.com",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "Should reject email without domain",
			email:   "user@",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "Should reject email without user part",
			email:   "@example.com",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "Should reject email with spaces",
			email:   "user name@example.com",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "Should reject very long email",
			email:   string(make([]byte, 256)) + "@example.com",
			wantErr: true,
			errMsg:  "at most 255 characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUser_Validate(t *testing.T) {
	orgID := core.MustNewID()
	t.Run("Should validate valid user", func(t *testing.T) {
		user := &User{
			ID:        core.MustNewID(),
			OrgID:     orgID,
			Email:     "john@example.com",
			Role:      RoleOrgCustomer,
			Status:    StatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := user.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should reject empty ID", func(t *testing.T) {
		user := &User{
			ID:     "",
			OrgID:  orgID,
			Email:  "john@example.com",
			Role:   RoleOrgCustomer,
			Status: StatusActive,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})
	t.Run("Should reject empty org ID", func(t *testing.T) {
		user := &User{
			ID:     core.MustNewID(),
			OrgID:  "",
			Email:  "john@example.com",
			Role:   RoleOrgCustomer,
			Status: StatusActive,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID cannot be empty")
	})
	t.Run("Should reject invalid email", func(t *testing.T) {
		user := &User{
			ID:     core.MustNewID(),
			OrgID:  orgID,
			Email:  "invalid-email",
			Role:   RoleOrgCustomer,
			Status: StatusActive,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email format")
	})
	t.Run("Should reject invalid role", func(t *testing.T) {
		user := &User{
			ID:     core.MustNewID(),
			OrgID:  orgID,
			Email:  "john@example.com",
			Role:   Role("invalid"),
			Status: StatusActive,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid role")
	})
	t.Run("Should reject invalid status", func(t *testing.T) {
		user := &User{
			ID:     core.MustNewID(),
			OrgID:  orgID,
			Email:  "john@example.com",
			Role:   RoleOrgCustomer,
			Status: Status("invalid"),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user status")
	})
}

func TestUser_StatusOperations(t *testing.T) {
	orgID := core.MustNewID()
	t.Run("Should check active status", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		assert.True(t, user.IsActive())
		assert.False(t, user.IsSuspended())
	})
	t.Run("Should suspend user", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		originalUpdatedAt := user.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time difference
		user.Suspend()
		assert.False(t, user.IsActive())
		assert.True(t, user.IsSuspended())
		assert.Equal(t, StatusSuspended, user.Status)
		assert.True(t, user.UpdatedAt.After(originalUpdatedAt))
	})
	t.Run("Should activate user", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		user.Status = StatusSuspended
		originalUpdatedAt := user.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time difference
		user.Activate()
		assert.True(t, user.IsActive())
		assert.False(t, user.IsSuspended())
		assert.Equal(t, StatusActive, user.Status)
		assert.True(t, user.UpdatedAt.After(originalUpdatedAt))
	})
}

func TestUser_UpdateRole(t *testing.T) {
	orgID := core.MustNewID()
	t.Run("Should update role successfully", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		originalUpdatedAt := user.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time difference
		err := user.UpdateRole(RoleOrgAdmin)
		assert.NoError(t, err)
		assert.Equal(t, RoleOrgAdmin, user.Role)
		assert.True(t, user.UpdatedAt.After(originalUpdatedAt))
	})
	t.Run("Should reject invalid role", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		err := user.UpdateRole(Role("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid role")
		assert.Equal(t, RoleOrgCustomer, user.Role) // Role should not change
	})
}

func TestUser_HasPermission(t *testing.T) {
	orgID := core.MustNewID()
	t.Run("Should check permission for active user", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		assert.True(t, user.HasPermission(PermWorkflowRead))
		assert.False(t, user.HasPermission(PermUserWrite))
	})
	t.Run("Should deny permission for suspended user", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgAdmin)
		user.Suspend()
		assert.False(t, user.HasPermission(PermWorkflowRead))
		assert.False(t, user.HasPermission(PermUserWrite))
	})
}

func TestUser_RoleChecks(t *testing.T) {
	orgID := core.MustNewID()
	t.Run("Should check system admin role", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleSystemAdmin)
		assert.True(t, user.IsSystemAdmin())
		assert.False(t, user.IsOrgAdmin())
		assert.False(t, user.IsOrgCustomer())
	})
	t.Run("Should check org admin role", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgAdmin)
		assert.False(t, user.IsSystemAdmin())
		assert.True(t, user.IsOrgAdmin())
		assert.False(t, user.IsOrgCustomer())
	})
	t.Run("Should check org member role", func(t *testing.T) {
		user, _ := NewUser(orgID, "test@example.com", RoleOrgCustomer)
		assert.False(t, user.IsSystemAdmin())
		assert.False(t, user.IsOrgAdmin())
		assert.True(t, user.IsOrgCustomer())
	})
}
