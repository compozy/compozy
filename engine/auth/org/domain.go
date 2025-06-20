package org

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
)

var (
	// validOrgNameRegex matches organization names with letters, numbers, spaces, hyphens, and underscores
	validOrgNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
	// consecutiveHyphensRegex matches one or more consecutive hyphens
	consecutiveHyphensRegex = regexp.MustCompile(`-+`)
	// startsWithLetterRegex matches strings that start with a lowercase letter
	startsWithLetterRegex = regexp.MustCompile(`^[a-z]`)
)

// OrganizationStatus represents the status of an organization
type OrganizationStatus string

const (
	// StatusProvisioning indicates the organization is being set up
	StatusProvisioning OrganizationStatus = "provisioning"
	// StatusActive indicates the organization is active and usable
	StatusActive OrganizationStatus = "active"
	// StatusSuspended indicates the organization is suspended
	StatusSuspended OrganizationStatus = "suspended"
	// StatusProvisioningFailed indicates the organization provisioning failed
	StatusProvisioningFailed OrganizationStatus = "provisioning_failed"
)

// IsValid checks if the organization status is valid
func (s OrganizationStatus) IsValid() bool {
	switch s {
	case StatusProvisioning, StatusActive, StatusSuspended, StatusProvisioningFailed:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if the status can transition to the target status
func (s OrganizationStatus) CanTransitionTo(target OrganizationStatus) bool {
	switch s {
	case StatusProvisioning:
		return target == StatusActive || target == StatusProvisioningFailed
	case StatusActive:
		return target == StatusSuspended
	case StatusSuspended:
		return target == StatusActive
	case StatusProvisioningFailed:
		return target == StatusProvisioning // Allow retry
	default:
		return false
	}
}

// Organization represents a tenant in the multi-tenant system
type Organization struct {
	ID                core.ID            `json:"id"                 db:"id"`
	Name              string             `json:"name"               db:"name"`
	TemporalNamespace string             `json:"temporal_namespace" db:"temporal_namespace"`
	Status            OrganizationStatus `json:"status"             db:"status"`
	CreatedAt         time.Time          `json:"created_at"         db:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"         db:"updated_at"`
}

// NewOrganization creates a new organization with the given name
func NewOrganization(name string) (*Organization, error) {
	if err := ValidateOrganizationName(name); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	id, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate organization ID: %w", err)
	}
	return &Organization{
		ID:                id,
		Name:              name,
		TemporalNamespace: GenerateNamespace(name),
		Status:            StatusProvisioning,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// ValidateOrganizationName validates the organization name
func ValidateOrganizationName(name string) error {
	if name == "" {
		return fmt.Errorf("organization name cannot be empty")
	}
	if len(name) < 3 {
		return fmt.Errorf("organization name must be at least 3 characters long")
	}
	if len(name) > 255 {
		return fmt.Errorf("organization name must be at most 255 characters long")
	}
	// Allow alphanumeric, spaces, hyphens, and underscores
	if !validOrgNameRegex.MatchString(name) {
		return fmt.Errorf("organization name can only contain letters, numbers, spaces, hyphens, and underscores")
	}
	return nil
}

// GenerateNamespace generates a Temporal namespace for the organization
func GenerateNamespace(orgName string) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(orgName)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove any consecutive hyphens
	slug = consecutiveHyphensRegex.ReplaceAllString(slug, "-")
	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")
	// Ensure the namespace starts with a letter
	if slug != "" && !startsWithLetterRegex.MatchString(slug) {
		slug = "org-" + slug
	}
	return fmt.Sprintf("compozy-%s", slug)
}

// Validate validates the organization entity
func (o *Organization) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("organization ID cannot be empty")
	}
	if err := ValidateOrganizationName(o.Name); err != nil {
		return err
	}
	if o.TemporalNamespace == "" {
		return fmt.Errorf("temporal namespace cannot be empty")
	}
	if !o.Status.IsValid() {
		return fmt.Errorf("invalid organization status: %s", o.Status)
	}
	return nil
}

// CanTransitionTo checks if the organization can transition to the target status
func (o *Organization) CanTransitionTo(target OrganizationStatus) bool {
	return o.Status.CanTransitionTo(target)
}

// TransitionTo transitions the organization to the target status
func (o *Organization) TransitionTo(target OrganizationStatus) error {
	if !o.CanTransitionTo(target) {
		return fmt.Errorf("cannot transition from %s to %s", o.Status, target)
	}
	o.Status = target
	o.UpdatedAt = time.Now().UTC()
	return nil
}

// IsActive returns true if the organization is active
func (o *Organization) IsActive() bool {
	return o.Status == StatusActive
}

// IsSuspended returns true if the organization is suspended
func (o *Organization) IsSuspended() bool {
	return o.Status == StatusSuspended
}

// IsProvisioning returns true if the organization is being provisioned
func (o *Organization) IsProvisioning() bool {
	return o.Status == StatusProvisioning
}

// IsProvisioningFailed returns true if the organization provisioning failed
func (o *Organization) IsProvisioningFailed() bool {
	return o.Status == StatusProvisioningFailed
}
