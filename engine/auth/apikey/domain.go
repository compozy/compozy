package apikey

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
)

// Status represents the status of an API key
type Status string

const (
	// StatusActive indicates the API key is active
	StatusActive Status = "active"
	// StatusRevoked indicates the API key has been revoked
	StatusRevoked Status = "revoked"
	// StatusExpired indicates the API key has expired.
	// Note: This status is typically set by a background job that cleans up
	// keys where ExpiresAt is in the past. The IsActive() method provides the
	// real-time validity check.
	StatusExpired Status = "expired"
)

const (
	// KeyLength is the length of the generated API key in bytes before encoding
	KeyLength = 16 // 16 bytes = 32 hex characters
	// KeyPrefix is the prefix for all API keys
	KeyPrefix = "cmpz_"
	// KeyPrefixLookupLength is the number of characters to include after the prefix for database lookups
	KeyPrefixLookupLength = 12
)

// IsValid checks if the API key status is valid
func (s Status) IsValid() bool {
	switch s {
	case StatusActive, StatusRevoked, StatusExpired:
		return true
	default:
		return false
	}
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID               core.ID    `json:"id"                     db:"id"`
	UserID           core.ID    `json:"user_id"                db:"user_id"`
	OrgID            core.ID    `json:"org_id"                 db:"org_id"`
	KeyHash          string     `json:"-"                      db:"key_hash"` // Never expose the hash
	KeyPrefix        string     `json:"key_prefix"             db:"key_prefix"`
	Name             string     `json:"name"                   db:"name"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"   db:"expires_at"`
	RateLimitPerHour int        `json:"rate_limit_per_hour"    db:"rate_limit_per_hour"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	Status           Status     `json:"status"                 db:"status"`
	CreatedAt        time.Time  `json:"created_at"             db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"             db:"updated_at"`
	// Key is the actual API key value, only populated when creating a new key
	Key string `json:"key,omitempty"          db:"-"`
	// Role is populated from the associated user, not stored in the API key table
	Role user.Role `json:"role,omitempty"         db:"-"`
}

// NewAPIKey creates a new API key
func NewAPIKey(userID, orgID core.ID, name string) (*APIKey, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization ID cannot be empty")
	}
	if err := ValidateAPIKeyName(name); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	id, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key ID: %w", err)
	}
	return &APIKey{
		ID:               id,
		UserID:           userID,
		OrgID:            orgID,
		Name:             name,
		RateLimitPerHour: 3600, // Default rate limit
		Status:           StatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// ValidateAPIKeyName validates the API key name
func ValidateAPIKeyName(name string) error {
	if name == "" {
		return fmt.Errorf("API key name cannot be empty")
	}
	if len(name) < 3 {
		return fmt.Errorf("API key name must be at least 3 characters long")
	}
	if len(name) > 255 {
		return fmt.Errorf("API key name must be at most 255 characters long")
	}
	return nil
}

// Validate validates the API key entity
func (k *APIKey) Validate() error {
	if k.ID == "" {
		return fmt.Errorf("API key ID cannot be empty")
	}
	if k.UserID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if k.OrgID == "" {
		return fmt.Errorf("organization ID cannot be empty")
	}
	if k.KeyHash == "" {
		return fmt.Errorf("key hash cannot be empty")
	}
	if k.KeyPrefix == "" {
		return fmt.Errorf("key prefix cannot be empty")
	}
	if err := ValidateAPIKeyName(k.Name); err != nil {
		return err
	}
	if k.RateLimitPerHour <= 0 {
		return fmt.Errorf("rate limit must be greater than 0")
	}
	if !k.Status.IsValid() {
		return fmt.Errorf("invalid API key status: %s", k.Status)
	}
	return nil
}

// IsActive returns true if the API key is active and not expired
func (k *APIKey) IsActive() bool {
	if k.Status != StatusActive {
		return false
	}
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now().UTC()) {
		return false
	}
	return true
}

// IsExpired returns true if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return k.ExpiresAt.Before(time.Now().UTC())
}

// IsRevoked returns true if the API key has been revoked
func (k *APIKey) IsRevoked() bool {
	return k.Status == StatusRevoked
}

// Revoke revokes the API key
func (k *APIKey) Revoke() {
	k.Status = StatusRevoked
	k.UpdatedAt = time.Now().UTC()
}

// UpdateLastUsed updates the last used timestamp
func (k *APIKey) UpdateLastUsed() {
	now := time.Now().UTC()
	k.LastUsedAt = &now
	k.UpdatedAt = now
}

// SetExpiration sets the expiration time for the API key
func (k *APIKey) SetExpiration(expiresAt time.Time) error {
	if expiresAt.Before(time.Now().UTC()) {
		return fmt.Errorf("expiration time must be in the future")
	}
	k.ExpiresAt = &expiresAt
	k.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateRateLimit updates the rate limit for the API key
func (k *APIKey) UpdateRateLimit(rateLimit int) error {
	if rateLimit <= 0 {
		return fmt.Errorf("rate limit must be greater than 0")
	}
	k.RateLimitPerHour = rateLimit
	k.UpdatedAt = time.Now().UTC()
	return nil
}

// HasPermission checks if the API key has a specific permission based on the associated user's role
func (k *APIKey) HasPermission(permission string) bool {
	if !k.IsActive() {
		return false
	}
	return k.Role.HasPermission(permission)
}

// ExtractPrefix extracts the prefix from a full API key for database lookups
func ExtractPrefix(fullKey string) (string, error) {
	expectedPrefixLength := len(KeyPrefix) + KeyPrefixLookupLength
	if len(fullKey) < expectedPrefixLength {
		return "", fmt.Errorf("invalid API key format: too short")
	}
	if fullKey[:len(KeyPrefix)] != KeyPrefix {
		return "", fmt.Errorf("invalid API key format: missing prefix")
	}
	// Return the prefix part (e.g., "cmpz_xxxxxxxxxxxx" - 12 chars after prefix)
	return fullKey[:expectedPrefixLength], nil
}
