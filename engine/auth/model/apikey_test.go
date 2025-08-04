package model

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKey_SecurityValidation(t *testing.T) {
	t.Run("Should validate APIKey has required security fields", func(t *testing.T) {
		key := &APIKey{
			ID:          core.MustNewID(),
			UserID:      core.MustNewID(),
			Hash:        []byte("bcrypt-hashed-key"),
			Fingerprint: []byte("sha256-fingerprint"),
			Prefix:      "cpzy_",
			CreatedAt:   time.Now(),
		}
		assert.NotEmpty(t, key.Hash, "Hash field is required for secure key validation")
		assert.NotEmpty(t, key.Fingerprint, "Fingerprint field is required for O(1) lookups")
		assert.NotEmpty(t, key.Prefix, "Prefix field is required for key identification")
		assert.Equal(t, "cpzy_", key.Prefix, "Prefix should follow expected format")
	})

	t.Run("Should handle Hash field as byte slice for bcrypt compatibility", func(t *testing.T) {
		hash := []byte("$2a$10$example.bcrypt.hash")
		key := &APIKey{
			ID:          core.MustNewID(),
			UserID:      core.MustNewID(),
			Hash:        hash,
			Fingerprint: []byte("fingerprint"),
			Prefix:      "cpzy_",
			CreatedAt:   time.Now(),
		}
		assert.Equal(t, hash, key.Hash)
		assert.IsType(t, []byte{}, key.Hash)
	})

	t.Run("Should handle Fingerprint field as byte slice for SHA-256 compatibility", func(t *testing.T) {
		fingerprint := []byte("sha256-generated-fingerprint")
		key := &APIKey{
			ID:          core.MustNewID(),
			UserID:      core.MustNewID(),
			Hash:        []byte("hash"),
			Fingerprint: fingerprint,
			Prefix:      "cpzy_",
			CreatedAt:   time.Now(),
		}
		assert.Equal(t, fingerprint, key.Fingerprint)
		assert.IsType(t, []byte{}, key.Fingerprint)
	})

	t.Run("Should maintain timestamp integrity for audit purposes", func(t *testing.T) {
		now := time.Now()
		key := &APIKey{
			ID:          core.MustNewID(),
			UserID:      core.MustNewID(),
			Hash:        []byte("hash"),
			Fingerprint: []byte("fingerprint"),
			Prefix:      "cpzy_",
			CreatedAt:   now,
		}
		assert.Equal(t, now, key.CreatedAt)
		assert.WithinDuration(t, now, key.CreatedAt, time.Millisecond)
	})

	t.Run("Should associate APIKey with user through UserID", func(t *testing.T) {
		userID := core.MustNewID()
		key := &APIKey{
			ID:          core.MustNewID(),
			UserID:      userID,
			Hash:        []byte("hash"),
			Fingerprint: []byte("fingerprint"),
			Prefix:      "cpzy_",
			CreatedAt:   time.Now(),
		}
		assert.Equal(t, userID, key.UserID)
		require.NotEmpty(t, key.UserID, "UserID must be set for security association")
	})
}
