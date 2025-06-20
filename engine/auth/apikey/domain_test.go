package apikey

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAPIKeyStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status APIKeyStatus
		want   bool
	}{
		{
			name:   "Should validate active status",
			status: StatusActive,
			want:   true,
		},
		{
			name:   "Should validate revoked status",
			status: StatusRevoked,
			want:   true,
		},
		{
			name:   "Should validate expired status",
			status: StatusExpired,
			want:   true,
		},
		{
			name:   "Should reject invalid status",
			status: APIKeyStatus("invalid"),
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

func TestGenerate(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should generate API key with secure random key", func(t *testing.T) {
		plainKey, apiKey, err := Generate(userID, orgID, "Test Key")
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.NotEmpty(t, plainKey)
		// Check plaintext key format
		assert.True(t, strings.HasPrefix(plainKey, KeyPrefix))
		assert.Greater(t, len(plainKey), len(KeyPrefix)+40) // Base64 encoded 32 bytes
		// Check API key fields
		assert.NotEmpty(t, apiKey.ID)
		assert.Equal(t, userID, apiKey.UserID)
		assert.Equal(t, orgID, apiKey.OrgID)
		assert.Equal(t, "Test Key", apiKey.Name)
		assert.NotEmpty(t, apiKey.KeyHash)
		assert.NotEmpty(t, apiKey.KeyPrefix)
		assert.True(t, strings.HasPrefix(apiKey.KeyPrefix, KeyPrefix))
		assert.Equal(t, len(KeyPrefix)+8, len(apiKey.KeyPrefix))
		assert.Equal(t, StatusActive, apiKey.Status)
		// Verify hash can be checked
		err = bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(plainKey))
		assert.NoError(t, err)
	})
	t.Run("Should generate unique keys each time", func(t *testing.T) {
		key1, apiKey1, err1 := Generate(userID, orgID, "Key 1")
		key2, apiKey2, err2 := Generate(userID, orgID, "Key 2")
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, key1, key2)
		assert.NotEqual(t, apiKey1.KeyHash, apiKey2.KeyHash)
		assert.NotEqual(t, apiKey1.KeyPrefix, apiKey2.KeyPrefix)
		assert.NotEqual(t, apiKey1.ID, apiKey2.ID)
	})
	t.Run("Should reject empty user ID", func(t *testing.T) {
		key, apiKey, err := Generate("", orgID, "Test Key")
		assert.Error(t, err)
		assert.Empty(t, key)
		assert.Nil(t, apiKey)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})
	t.Run("Should reject empty org ID", func(t *testing.T) {
		key, apiKey, err := Generate(userID, "", "Test Key")
		assert.Error(t, err)
		assert.Empty(t, key)
		assert.Nil(t, apiKey)
		assert.Contains(t, err.Error(), "organization ID cannot be empty")
	})
	t.Run("Should reject invalid name", func(t *testing.T) {
		key, apiKey, err := Generate(userID, orgID, "")
		assert.Error(t, err)
		assert.Empty(t, key)
		assert.Nil(t, apiKey)
		assert.Contains(t, err.Error(), "API key name cannot be empty")
	})
}

func TestNewAPIKey(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should create new API key with valid details", func(t *testing.T) {
		key, err := NewAPIKey(userID, orgID, "Production API Key")
		require.NoError(t, err)
		require.NotNil(t, key)
		assert.NotEmpty(t, key.ID)
		assert.Equal(t, userID, key.UserID)
		assert.Equal(t, orgID, key.OrgID)
		assert.Equal(t, "Production API Key", key.Name)
		assert.Equal(t, 3600, key.RateLimitPerHour)
		assert.Equal(t, StatusActive, key.Status)
		assert.Nil(t, key.ExpiresAt)
		assert.Nil(t, key.LastUsedAt)
		assert.False(t, key.CreatedAt.IsZero())
		assert.False(t, key.UpdatedAt.IsZero())
	})
	t.Run("Should reject empty user ID", func(t *testing.T) {
		key, err := NewAPIKey("", orgID, "Test Key")
		assert.Error(t, err)
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})
	t.Run("Should reject empty org ID", func(t *testing.T) {
		key, err := NewAPIKey(userID, "", "Test Key")
		assert.Error(t, err)
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "organization ID cannot be empty")
	})
	t.Run("Should reject invalid name", func(t *testing.T) {
		key, err := NewAPIKey(userID, orgID, "")
		assert.Error(t, err)
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
}

func TestValidateAPIKeyName(t *testing.T) {
	tests := []struct {
		name    string
		keyName string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Should accept valid name",
			keyName: "Production API Key",
			wantErr: false,
		},
		{
			name:    "Should accept minimum length name",
			keyName: "abc",
			wantErr: false,
		},
		{
			name:    "Should reject empty name",
			keyName: "",
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name:    "Should reject short name",
			keyName: "ab",
			wantErr: true,
			errMsg:  "at least 3 characters",
		},
		{
			name:    "Should reject very long name",
			keyName: string(make([]byte, 256)),
			wantErr: true,
			errMsg:  "at most 255 characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKeyName(tt.keyName)
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

func TestAPIKey_Validate(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should validate valid API key", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           StatusActive,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		err := key.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should reject empty ID", func(t *testing.T) {
		key := &APIKey{
			ID:               "",
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key ID cannot be empty")
	})
	t.Run("Should reject empty key hash", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key hash cannot be empty")
	})
	t.Run("Should reject empty key prefix", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key prefix cannot be empty")
	})
	t.Run("Should reject zero rate limit", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 0,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit must be greater than 0")
	})
	t.Run("Should reject empty user ID", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           "",
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})
	t.Run("Should reject empty org ID", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            "",
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID cannot be empty")
	})
	t.Run("Should reject invalid name", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "ab", // Too short
			RateLimitPerHour: 3600,
			Status:           StatusActive,
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 3 characters")
	})
	t.Run("Should reject invalid status", func(t *testing.T) {
		key := &APIKey{
			ID:               core.MustNewID(),
			UserID:           userID,
			OrgID:            orgID,
			KeyHash:          "hashed_key",
			KeyPrefix:        "cmpz_abcd",
			Name:             "Test Key",
			RateLimitPerHour: 3600,
			Status:           APIKeyStatus("invalid"),
		}
		err := key.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key status")
	})
}

func TestAPIKey_IsActive(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should be active when status is active and not expired", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		assert.True(t, key.IsActive())
	})
	t.Run("Should not be active when status is revoked", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		key.Status = StatusRevoked
		assert.False(t, key.IsActive())
	})
	t.Run("Should not be active when expired", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		expiredTime := time.Now().Add(-time.Hour)
		key.ExpiresAt = &expiredTime
		assert.False(t, key.IsActive())
	})
	t.Run("Should be active when expiration is in the future", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		futureTime := time.Now().Add(time.Hour)
		key.ExpiresAt = &futureTime
		assert.True(t, key.IsActive())
	})
}

func TestAPIKey_IsExpired(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should not be expired when no expiration set", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		assert.False(t, key.IsExpired())
	})
	t.Run("Should be expired when expiration is in the past", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		expiredTime := time.Now().Add(-time.Hour)
		key.ExpiresAt = &expiredTime
		assert.True(t, key.IsExpired())
	})
	t.Run("Should not be expired when expiration is in the future", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		futureTime := time.Now().Add(time.Hour)
		key.ExpiresAt = &futureTime
		assert.False(t, key.IsExpired())
	})
}

func TestAPIKey_Revoke(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should revoke API key", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		originalUpdatedAt := key.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time difference
		key.Revoke()
		assert.Equal(t, StatusRevoked, key.Status)
		assert.True(t, key.IsRevoked())
		assert.True(t, key.UpdatedAt.After(originalUpdatedAt))
	})
}

func TestAPIKey_UpdateLastUsed(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should update last used timestamp", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		assert.Nil(t, key.LastUsedAt)
		originalUpdatedAt := key.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time difference
		key.UpdateLastUsed()
		assert.NotNil(t, key.LastUsedAt)
		assert.True(t, key.LastUsedAt.After(key.CreatedAt))
		assert.True(t, key.UpdatedAt.After(originalUpdatedAt))
		assert.Equal(t, key.UpdatedAt, *key.LastUsedAt)
	})
}

func TestAPIKey_SetExpiration(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should set future expiration", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		futureTime := time.Now().Add(24 * time.Hour)
		err := key.SetExpiration(futureTime)
		assert.NoError(t, err)
		assert.NotNil(t, key.ExpiresAt)
		assert.Equal(t, futureTime.Unix(), key.ExpiresAt.Unix())
	})
	t.Run("Should reject past expiration", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		pastTime := time.Now().Add(-time.Hour)
		err := key.SetExpiration(pastTime)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be in the future")
		assert.Nil(t, key.ExpiresAt)
	})
	t.Run("Should update status to expired if already expired", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		// Set a future time first
		futureTime := time.Now().Add(time.Millisecond)
		key.SetExpiration(futureTime)
		// Wait for it to expire
		time.Sleep(2 * time.Millisecond)
		// Check if the key is expired
		assert.True(t, key.IsExpired())
		// The status doesn't automatically update; it would be updated by the service layer
		assert.False(t, key.IsActive())
	})
}

func TestAPIKey_UpdateRateLimit(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should update rate limit", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		originalUpdatedAt := key.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time difference
		err := key.UpdateRateLimit(7200)
		assert.NoError(t, err)
		assert.Equal(t, 7200, key.RateLimitPerHour)
		assert.True(t, key.UpdatedAt.After(originalUpdatedAt))
	})
	t.Run("Should reject zero rate limit", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		err := key.UpdateRateLimit(0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be greater than 0")
		assert.Equal(t, 3600, key.RateLimitPerHour) // Should not change
	})
	t.Run("Should reject negative rate limit", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		err := key.UpdateRateLimit(-100)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be greater than 0")
		assert.Equal(t, 3600, key.RateLimitPerHour) // Should not change
	})
}

func TestAPIKey_HasPermission(t *testing.T) {
	userID := core.MustNewID()
	orgID := core.MustNewID()
	t.Run("Should check permission for active key with role", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		key.Role = user.RoleOrgCustomer
		assert.True(t, key.HasPermission(user.PermWorkflowRead))
		assert.False(t, key.HasPermission(user.PermUserWrite))
	})
	t.Run("Should deny permission for revoked key", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		key.Role = user.RoleOrgAdmin
		key.Revoke()
		assert.False(t, key.HasPermission(user.PermWorkflowRead))
		assert.False(t, key.HasPermission(user.PermUserWrite))
	})
	t.Run("Should deny permission for expired key", func(t *testing.T) {
		key, _ := NewAPIKey(userID, orgID, "Test Key")
		key.KeyHash = "hash"
		key.KeyPrefix = "prefix"
		key.Role = user.RoleOrgAdmin
		expiredTime := time.Now().Add(-time.Hour)
		key.ExpiresAt = &expiredTime
		assert.False(t, key.HasPermission(user.PermWorkflowRead))
		assert.False(t, key.HasPermission(user.PermUserWrite))
	})
}
