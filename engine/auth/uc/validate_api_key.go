package uc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

// Pre-computed bcrypt hash of an arbitrary string (cost=10) for timing-equalized comparisons.
// Any valid bcrypt hash works; value choice is irrelevant as long as it's valid.
var dummyBcryptHash = []byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOa5hnhtNGRjukDWO2xzg3sjQTL1dDQ2u")

// Background task semaphore to prevent unbounded goroutine creation
// Limits concurrent background operations to prevent resource exhaustion
var backgroundTaskSem = make(chan struct{}, 10) // Allow max 10 concurrent background tasks

// ValidateAPIKey use case for validating an API key and returning the associated user
type ValidateAPIKey struct {
	repo      Repository
	plaintext string
}

// NewValidateAPIKey creates a new validate API key use case
func NewValidateAPIKey(repo Repository, plaintext string) *ValidateAPIKey {
	return &ValidateAPIKey{
		repo:      repo,
		plaintext: plaintext,
	}
}

// Execute validates an API key and returns the associated user
func (uc *ValidateAPIKey) Execute(ctx context.Context) (*model.User, error) {
	log := logger.FromContext(ctx)

	// Hash the plaintext key to find it in the database (fingerprint for O(1) lookup)
	hash := sha256.Sum256([]byte(uc.plaintext))

	apiKey, err := uc.repo.GetAPIKeyByHash(ctx, hash[:])
	if err != nil {
		//nolint:errcheck // Intentionally ignore error for timing equalization to prevent timing attacks
		_ = bcrypt.CompareHashAndPassword(
			dummyBcryptHash,
			[]byte(uc.plaintext),
		)
		if errors.Is(err, ErrAPIKeyNotFound) {
			log.Debug("API key not found", "error", err)
			return nil, fmt.Errorf("invalid API key")
		}
		log.Error("Failed to get API key by hash", "error", err)
		return nil, fmt.Errorf("internal error validating API key: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword(apiKey.Hash, []byte(uc.plaintext)); err != nil {
		log.Debug("API key hash verification failed", "error", err)
		return nil, fmt.Errorf("invalid API key")
	}

	// Get the associated user
	user, err := uc.repo.GetUserByID(ctx, apiKey.UserID)
	if err != nil {
		log.Error("Failed to get user for valid API key", "error", err, "user_id", apiKey.UserID)
		return nil, fmt.Errorf("failed to get user for API key: %w", err)
	}

	// Update last used timestamp (fire and forget with resource limiting)
	select {
	case backgroundTaskSem <- struct{}{}:
		// Acquired semaphore, proceed with background task
		go func() {
			defer func() { <-backgroundTaskSem }() // Release semaphore when done
			// Detach cancellation but preserve values; bound execution time
			bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			if updateErr := uc.repo.UpdateAPIKeyLastUsed(bgCtx, apiKey.ID); updateErr != nil {
				// Extract logger from background context (request-scoped values preserved)
				bgLog := logger.FromContext(bgCtx)
				bgLog.Warn("Failed to update API key last used", "error", updateErr, "key_id", apiKey.ID)
			}
		}()
	default:
		// Semaphore full, skip update to prevent resource exhaustion
		log.Debug("Skipping API key last used update due to high load", "key_id", apiKey.ID)
	}

	return user, nil
}
