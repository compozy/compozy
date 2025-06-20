package org

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status OrganizationStatus
		want   bool
	}{
		{
			name:   "Should validate provisioning status",
			status: StatusProvisioning,
			want:   true,
		},
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
			name:   "Should validate provisioning_failed status",
			status: StatusProvisioningFailed,
			want:   true,
		},
		{
			name:   "Should reject invalid status",
			status: OrganizationStatus("invalid"),
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

func TestOrganizationStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from OrganizationStatus
		to   OrganizationStatus
		want bool
	}{
		{
			name: "Should allow provisioning to active",
			from: StatusProvisioning,
			to:   StatusActive,
			want: true,
		},
		{
			name: "Should allow provisioning to provisioning_failed",
			from: StatusProvisioning,
			to:   StatusProvisioningFailed,
			want: true,
		},
		{
			name: "Should allow active to suspended",
			from: StatusActive,
			to:   StatusSuspended,
			want: true,
		},
		{
			name: "Should allow suspended to active",
			from: StatusSuspended,
			to:   StatusActive,
			want: true,
		},
		{
			name: "Should allow provisioning_failed to provisioning for retry",
			from: StatusProvisioningFailed,
			to:   StatusProvisioning,
			want: true,
		},
		{
			name: "Should not allow provisioning to suspended",
			from: StatusProvisioning,
			to:   StatusSuspended,
			want: false,
		},
		{
			name: "Should not allow active to provisioning",
			from: StatusActive,
			to:   StatusProvisioning,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewOrganization(t *testing.T) {
	t.Run("Should create new organization with valid name", func(t *testing.T) {
		org, err := NewOrganization("Acme Corporation")
		require.NoError(t, err)
		require.NotNil(t, org)
		assert.NotEmpty(t, org.ID)
		assert.Equal(t, "Acme Corporation", org.Name)
		assert.Equal(t, "compozy-acme-corporation", org.TemporalNamespace)
		assert.Equal(t, StatusProvisioning, org.Status)
		assert.False(t, org.CreatedAt.IsZero())
		assert.False(t, org.UpdatedAt.IsZero())
	})
	t.Run("Should reject empty name", func(t *testing.T) {
		org, err := NewOrganization("")
		assert.Error(t, err)
		assert.Nil(t, org)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
	t.Run("Should reject short name", func(t *testing.T) {
		org, err := NewOrganization("AB")
		assert.Error(t, err)
		assert.Nil(t, org)
		assert.Contains(t, err.Error(), "at least 3 characters")
	})
	t.Run("Should reject long name", func(t *testing.T) {
		longName := make([]byte, 256)
		for i := range longName {
			longName[i] = 'a'
		}
		org, err := NewOrganization(string(longName))
		assert.Error(t, err)
		assert.Nil(t, org)
		assert.Contains(t, err.Error(), "at most 255 characters")
	})
	t.Run("Should reject invalid characters", func(t *testing.T) {
		org, err := NewOrganization("Acme@Corp")
		assert.Error(t, err)
		assert.Nil(t, org)
		assert.Contains(t, err.Error(), "can only contain")
	})
}

func TestGenerateNamespace(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		expected string
	}{
		{
			name:     "Should generate namespace from simple name",
			orgName:  "Acme",
			expected: "compozy-acme",
		},
		{
			name:     "Should handle spaces",
			orgName:  "Acme Corporation",
			expected: "compozy-acme-corporation",
		},
		{
			name:     "Should handle underscores",
			orgName:  "Acme_Corp",
			expected: "compozy-acme-corp",
		},
		{
			name:     "Should handle multiple spaces",
			orgName:  "Acme   Corporation",
			expected: "compozy-acme-corporation",
		},
		{
			name:     "Should handle mixed case",
			orgName:  "ACME Corporation",
			expected: "compozy-acme-corporation",
		},
		{
			name:     "Should handle leading/trailing spaces",
			orgName:  " Acme Corporation ",
			expected: "compozy-acme-corporation",
		},
		{
			name:     "Should handle number prefix",
			orgName:  "123 Corp",
			expected: "compozy-org-123-corp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateNamespace(tt.orgName)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestOrganization_Validate(t *testing.T) {
	t.Run("Should validate valid organization", func(t *testing.T) {
		org := &Organization{
			ID:                core.MustNewID(),
			Name:              "Acme Corporation",
			TemporalNamespace: "compozy-acme-corporation",
			Status:            StatusActive,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		err := org.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should reject empty ID", func(t *testing.T) {
		org := &Organization{
			ID:                "",
			Name:              "Acme Corporation",
			TemporalNamespace: "compozy-acme-corporation",
			Status:            StatusActive,
		}
		err := org.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID cannot be empty")
	})
	t.Run("Should reject invalid name", func(t *testing.T) {
		org := &Organization{
			ID:                core.MustNewID(),
			Name:              "",
			TemporalNamespace: "compozy-acme-corporation",
			Status:            StatusActive,
		}
		err := org.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
	t.Run("Should reject empty namespace", func(t *testing.T) {
		org := &Organization{
			ID:                core.MustNewID(),
			Name:              "Acme Corporation",
			TemporalNamespace: "",
			Status:            StatusActive,
		}
		err := org.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "namespace cannot be empty")
	})
	t.Run("Should reject invalid status", func(t *testing.T) {
		org := &Organization{
			ID:                core.MustNewID(),
			Name:              "Acme Corporation",
			TemporalNamespace: "compozy-acme-corporation",
			Status:            OrganizationStatus("invalid"),
		}
		err := org.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid organization status")
	})
}

func TestOrganization_TransitionTo(t *testing.T) {
	t.Run("Should transition from provisioning to active", func(t *testing.T) {
		org, _ := NewOrganization("Test Org")
		err := org.TransitionTo(StatusActive)
		assert.NoError(t, err)
		assert.Equal(t, StatusActive, org.Status)
		assert.True(t, org.UpdatedAt.After(org.CreatedAt))
	})
	t.Run("Should reject invalid transition", func(t *testing.T) {
		org, _ := NewOrganization("Test Org")
		err := org.TransitionTo(StatusSuspended)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot transition from provisioning to suspended")
		assert.Equal(t, StatusProvisioning, org.Status)
	})
}

func TestOrganization_StatusChecks(t *testing.T) {
	t.Run("Should check active status", func(t *testing.T) {
		org, _ := NewOrganization("Test Org")
		assert.False(t, org.IsActive())
		org.Status = StatusActive
		assert.True(t, org.IsActive())
	})
	t.Run("Should check suspended status", func(t *testing.T) {
		org, _ := NewOrganization("Test Org")
		assert.False(t, org.IsSuspended())
		org.Status = StatusSuspended
		assert.True(t, org.IsSuspended())
	})
	t.Run("Should check provisioning status", func(t *testing.T) {
		org, _ := NewOrganization("Test Org")
		assert.True(t, org.IsProvisioning())
		org.Status = StatusActive
		assert.False(t, org.IsProvisioning())
	})
	t.Run("Should check provisioning failed status", func(t *testing.T) {
		org, _ := NewOrganization("Test Org")
		assert.False(t, org.IsProvisioningFailed())
		org.Status = StatusProvisioningFailed
		assert.True(t, org.IsProvisioningFailed())
	})
}
