package uc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

// Pre-computed bcrypt hash of an arbitrary string (cost=10) for timing-equalized comparisons.
// Any valid bcrypt hash works; value choice is irrelevant as long as it's valid.
var dummyBcryptHash = []byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOa5hnhtNGRjukDWO2xzg3sjQTL1dDQ2u")

const (
	// defaultAPIKeyLastUsedMaxConcurrency limits concurrent background operations to prevent resource exhaustion.
	defaultAPIKeyLastUsedMaxConcurrency = 10
	// defaultAPIKeyLastUsedTimeout bounds the execution time for background last-used updates.
	defaultAPIKeyLastUsedTimeout = 2 * time.Second
)

// backgroundLimiter coordinates non-blocking acquisition for asynchronous tasks based on configurable limits.
type backgroundLimiter struct {
	mu    sync.RWMutex
	sem   chan struct{}
	limit int
}

func newBackgroundLimiter(limit int) *backgroundLimiter {
	bl := &backgroundLimiter{}
	bl.ensure(limit)
	return bl
}

func (l *backgroundLimiter) ensure(limit int) chan struct{} {
	if limit < 0 {
		limit = 0
	}
	l.mu.RLock()
	if limit == l.limit {
		ch := l.sem
		l.mu.RUnlock()
		return ch
	}
	l.mu.RUnlock()
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit == l.limit {
		return l.sem
	}
	if limit == 0 {
		l.sem = nil
		l.limit = 0
		return nil
	}
	l.sem = make(chan struct{}, limit)
	l.limit = limit
	return l.sem
}

func (l *backgroundLimiter) tryAcquire(limit int) (chan struct{}, bool) {
	if limit <= 0 {
		l.ensure(limit)
		return nil, false
	}
	ch := l.ensure(limit)
	if ch == nil {
		return nil, false
	}
	select {
	case ch <- struct{}{}:
		return ch, true
	default:
		return nil, false
	}
}

func (l *backgroundLimiter) release(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
	}
}

// Background limiter to prevent unbounded goroutine creation.
var apiKeyLastUsedLimiter = newBackgroundLimiter(defaultAPIKeyLastUsedMaxConcurrency)

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
	hash := sha256.Sum256([]byte(uc.plaintext))
	apiKey, err := uc.repo.GetAPIKeyByFingerprint(ctx, hash[:])
	if err != nil {
		// NOTE: Perform dummy bcrypt comparison to equalize timing with the success path and prevent timing attacks.
		//nolint:errcheck // CompareHashAndPassword failure is expected for invalid keys.
		_ = bcrypt.CompareHashAndPassword(
			dummyBcryptHash,
			[]byte(uc.plaintext),
		)
		if errors.Is(err, ErrAPIKeyNotFound) {
			log.Debug("Invalid API key", "error", err)
			return nil, ErrInvalidCredentials
		}
		log.Error("Failed to get API key by fingerprint", "error", err)
		return nil, fmt.Errorf("internal error validating API key: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword(apiKey.Hash, []byte(uc.plaintext)); err != nil {
		log.Debug("Invalid API key", "error", err)
		return nil, ErrInvalidCredentials
	}
	user, err := uc.repo.GetUserByID(ctx, apiKey.UserID)
	if err != nil {
		log.Error("Failed to get user for valid API key", "error", err, "user_id", apiKey.UserID)
		return nil, fmt.Errorf("failed to get user for API key: %w", err)
	}
	uc.scheduleLastUsedUpdate(ctx, apiKey)
	return user, nil
}

func (uc *ValidateAPIKey) scheduleLastUsedUpdate(ctx context.Context, apiKey *model.APIKey) {
	cfg := config.FromContext(ctx)
	maxConcurrency := defaultAPIKeyLastUsedMaxConcurrency
	updateTimeout := defaultAPIKeyLastUsedTimeout
	if cfg != nil {
		if v := cfg.Server.Auth.APIKeyLastUsedMaxConcurrency; v < 0 {
			maxConcurrency = 0
		} else if v > 0 {
			maxConcurrency = v
		}
		if timeout := cfg.Server.Auth.APIKeyLastUsedTimeout; timeout > 0 {
			updateTimeout = timeout
		}
	}
	log := logger.FromContext(ctx)
	if maxConcurrency == 0 {
		log.Debug("Skipping API key last used update because asynchronous updates are disabled", "key_id", apiKey.ID)
		return
	}
	releaseCh, acquired := apiKeyLastUsedLimiter.tryAcquire(maxConcurrency)
	if !acquired {
		log.Debug("Skipping API key last used update due to high load", "key_id", apiKey.ID)
		return
	}
	go func(ch chan struct{}, timeout time.Duration) {
		defer apiKeyLastUsedLimiter.release(ch)

		baseCtx := context.WithoutCancel(ctx)
		bgCtx := baseCtx
		var cancel context.CancelFunc
		if timeout > 0 {
			bgCtx, cancel = context.WithTimeout(baseCtx, timeout)
			defer cancel()
		}
		if updateErr := uc.repo.UpdateAPIKeyLastUsed(bgCtx, apiKey.ID); updateErr != nil {
			bgLog := logger.FromContext(bgCtx)
			bgLog.Warn("Failed to update API key last used", "error", updateErr, "key_id", apiKey.ID)
		}
	}(releaseCh, updateTimeout)
}
