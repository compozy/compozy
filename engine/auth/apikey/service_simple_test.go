package apikey_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/audit"
)

func TestAPIKeyService_SimpleGenerateAndHash(t *testing.T) {
	t.Run("Should generate API key with correct format", func(t *testing.T) {
		// Arrange
		config := apikey.DefaultServiceConfig()
		svc := apikey.NewService(config, nil, nil, nil, audit.NewService())
		// Act
		key, err := svc.GenerateAPIKey()
		// Assert
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(key, "cmpz_"))
		assert.Equal(t, 37, len(key)) // "cmpz_" (5) + 32 hex chars
	})
	t.Run("Should hash and verify API key", func(t *testing.T) {
		// Arrange
		config := apikey.DefaultServiceConfig()
		svc := apikey.NewService(config, nil, nil, nil, audit.NewService())
		key := "cmpz_test_key"
		// Act
		hash, err := svc.HashAPIKey(key)
		require.NoError(t, err)
		// Verify
		valid := svc.VerifyAPIKey(key, hash)
		// Assert
		assert.True(t, valid)
		// Test wrong key
		assert.False(t, svc.VerifyAPIKey("cmpz_wrong_key", hash))
	})
	t.Run("Should extract key prefix correctly", func(t *testing.T) {
		// Arrange
		config := apikey.DefaultServiceConfig()
		svc := apikey.NewService(config, nil, nil, nil, audit.NewService())
		// Test valid key
		key := "cmpz_1234567890abcdef1234567890abcdef"
		prefix := svc.ExtractKeyPrefix(key)
		assert.Equal(t, "1234567890ab", prefix)
		// Test invalid key
		assert.Empty(t, svc.ExtractKeyPrefix("invalid_key"))
		// Test short key
		assert.Equal(t, "short", svc.ExtractKeyPrefix("cmpz_short"))
	})
}
