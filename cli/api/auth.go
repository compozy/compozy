package api

import (
	"context"
	"strings"
)

// AuthClient defines the interface for authentication operations
type AuthClient interface {
	// Key Management
	GenerateKey(ctx context.Context, req *GenerateKeyRequest) (string, error)
	ListKeys(ctx context.Context) ([]KeyInfo, error)
	List(ctx context.Context) ([]KeyInfo, error) // Alias for ListKeys to satisfy DataClient interface
	RevokeKey(ctx context.Context, keyID string) error

	// User Management (Admin only)
	ListUsers(ctx context.Context) ([]UserInfo, error)
	CreateUser(ctx context.Context, req CreateUserRequest) (*UserInfo, error)
	UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*UserInfo, error)
	DeleteUser(ctx context.Context, userID string) error

	// Client Info
	GetBaseURL() string
	GetAPIKey() string
}

// GenerateKeyRequest represents the request to generate an API key
type GenerateKeyRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Expires     string `json:"expires,omitempty"`
}

// KeyInfo represents an API key
type KeyInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prefix      string `json:"prefix"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	LastUsedAt  string `json:"last_used_at,omitempty"`
	IsActive    bool   `json:"is_active"`
}

// GetCreatedAt returns the creation timestamp for sorting compatibility
func (k *KeyInfo) GetCreatedAt() string {
	return k.CreatedAt
}

// GetName returns the name for sorting compatibility
func (k *KeyInfo) GetName() string {
	return k.Name
}

// GetLastUsed returns the last used timestamp for sorting compatibility
func (k *KeyInfo) GetLastUsed() string {
	return k.LastUsedAt
}

// GetPrefix returns the key prefix for sorting compatibility
func (k *KeyInfo) GetPrefix() string {
	return k.Prefix
}

// GetSortKey returns the sort key for a given field (Sortable interface)
//
//nolint:gocritic // Large struct but necessary for interface compatibility
func (k KeyInfo) GetSortKey(field string) any {
	switch field {
	case "name":
		return k.Name
	case "prefix":
		return k.Prefix
	case "created":
		return k.CreatedAt
	case "last_used":
		return k.LastUsedAt
	case "active":
		return k.IsActive
	default:
		return k.CreatedAt
	}
}

// GetDisplayFields returns the fields that can be displayed (Listable interface)
//
//nolint:gocritic // Large struct but necessary for interface compatibility
func (k KeyInfo) GetDisplayFields() []string {
	return []string{"name", "prefix", "created", "last_used", "active"}
}

// GetDisplayValue returns the display value for a given field (Listable interface)
//
//nolint:gocritic // Large struct but necessary for interface compatibility
func (k KeyInfo) GetDisplayValue(field string) string {
	switch field {
	case "name":
		return k.Name
	case "prefix":
		return k.Prefix
	case "created":
		return k.CreatedAt
	case "last_used":
		if k.LastUsedAt == "" {
			return "Never"
		}
		return k.LastUsedAt
	case "active":
		if k.IsActive {
			return "Yes"
		}
		return "No"
	default:
		return ""
	}
}

// MatchesSearch returns true if the key matches the search term (Searchable interface)
//
//nolint:gocritic // Large struct but necessary for interface compatibility
func (k KeyInfo) MatchesSearch(term string) bool {
	return contains(k.Name, term) || contains(k.Prefix, term) || contains(k.ID, term)
}

// contains checks if string s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// UserInfo represents a user
type UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email *string `json:"email,omitempty"`
	Name  *string `json:"name,omitempty"`
	Role  *string `json:"role,omitempty"`
}
