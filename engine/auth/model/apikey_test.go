package model

import (
	"database/sql"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestAPIKey_Model(t *testing.T) {
	t.Run("Should create API key with all fields", func(t *testing.T) {
		keyID := core.MustNewID()
		userID := core.MustNewID()
		now := time.Now()
		hash := []byte("hashed-key-content")

		apiKey := &APIKey{
			ID:        keyID,
			UserID:    userID,
			Hash:      hash,
			Prefix:    "cpzy_",
			CreatedAt: now,
			LastUsed:  sql.NullTime{Time: now, Valid: true},
		}

		assert.Equal(t, keyID, apiKey.ID)
		assert.Equal(t, userID, apiKey.UserID)
		assert.Equal(t, hash, apiKey.Hash)
		assert.Equal(t, "cpzy_", apiKey.Prefix)
		assert.Equal(t, now, apiKey.CreatedAt)
		assert.True(t, apiKey.LastUsed.Valid)
		assert.Equal(t, now, apiKey.LastUsed.Time)
	})

	t.Run("Should create API key without last used", func(t *testing.T) {
		keyID := core.MustNewID()
		userID := core.MustNewID()
		now := time.Now()
		hash := []byte("hashed-key-content")

		apiKey := &APIKey{
			ID:        keyID,
			UserID:    userID,
			Hash:      hash,
			Prefix:    "cpzy_",
			CreatedAt: now,
			LastUsed:  sql.NullTime{Valid: false},
		}

		assert.Equal(t, keyID, apiKey.ID)
		assert.Equal(t, userID, apiKey.UserID)
		assert.Equal(t, hash, apiKey.Hash)
		assert.Equal(t, "cpzy_", apiKey.Prefix)
		assert.Equal(t, now, apiKey.CreatedAt)
		assert.False(t, apiKey.LastUsed.Valid)
	})
}
