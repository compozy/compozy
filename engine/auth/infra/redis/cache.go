package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// CachedRepository wraps a repository with Redis caching for performance
type CachedRepository struct {
	repo   uc.Repository
	client Interface
	ttl    time.Duration
}

// Interface defines the minimal Redis interface needed for caching
type Interface interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Ping(ctx context.Context) *redis.StatusCmd
}

// NewCachedRepository creates a new cached repository
func NewCachedRepository(repo uc.Repository, client Interface, ttl time.Duration) uc.Repository {
	if ttl == 0 {
		ttl = 30 * time.Second // default TTL as per tech spec
	}
	return &CachedRepository{
		repo:   repo,
		client: client,
		ttl:    ttl,
	}
}

// cacheKey generates a cache key for API key hashes
func (c *CachedRepository) cacheKey(hash []byte) string {
	return fmt.Sprintf("auth:apikey:hash:%x", hash)
}

// GetAPIKeyByHash retrieves an API key by hash with Redis caching
func (c *CachedRepository) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
	log := logger.FromContext(ctx)
	cacheKey := c.cacheKey(hash)

	// Try cache first
	cached := c.client.Get(ctx, cacheKey)
	if cached.Err() == nil {
		var key model.APIKey
		unmarshalErr := json.Unmarshal([]byte(cached.Val()), &key)
		if unmarshalErr == nil {
			log.Debug("API key cache hit", "cache_key", cacheKey)
			return &key, nil
		}
		log.Debug("failed to unmarshal cached API key", "error", unmarshalErr)
	}

	// Cache miss - fetch from database
	log.Debug("API key cache miss", "cache_key", cacheKey)
	key, err := c.repo.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	// Cache the result
	keyJSON, err := json.Marshal(key)
	if err != nil {
		log.Warn("failed to marshal API key for cache", "error", err)
		return key, nil // return result even if caching fails
	}

	if err := c.client.Set(ctx, cacheKey, keyJSON, c.ttl).Err(); err != nil {
		log.Warn("failed to cache API key", "error", err)
	} else {
		log.Debug("cached API key", "cache_key", cacheKey, "ttl", c.ttl)
	}

	return key, nil
}

// invalidateAPIKeyCache invalidates both hash-based and ID-based cache entries for an API key
func (c *CachedRepository) invalidateAPIKeyCache(ctx context.Context, keyID core.ID) error {
	log := logger.FromContext(ctx)

	// First, invalidate the ID-based cache entry
	idCacheKey := fmt.Sprintf("auth:apikey:id:%s", keyID)
	if err := c.client.Del(ctx, idCacheKey).Err(); err != nil {
		log.Warn("failed to delete ID-based cache entry", "cache_key", idCacheKey, "error", err)
		return fmt.Errorf("failed to delete ID-based cache entry: %w", err)
	}

	// Get the API key to find its fingerprint for hash-based cache invalidation
	key, err := c.repo.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		log.Warn("failed to get API key for hash-based cache invalidation", "key_id", keyID, "error", err)
		// Don't return error here - ID cache was invalidated successfully
		log.Debug("API key ID cache invalidated", "key_id", keyID, "id_cache_key", idCacheKey)
		return nil
	}

	// Invalidate the hash-based cache entry
	hashCacheKey := c.cacheKey(key.Fingerprint)
	if err := c.client.Del(ctx, hashCacheKey).Err(); err != nil {
		log.Warn("failed to delete hash-based cache entry", "cache_key", hashCacheKey, "error", err)
		return fmt.Errorf("failed to delete hash-based cache entry: %w", err)
	}

	log.Debug("API key cache invalidated", "key_id", keyID, "id_cache_key", idCacheKey, "hash_cache_key", hashCacheKey)
	return nil
}

// Delegate all other methods to the wrapped repository

func (c *CachedRepository) CreateUser(ctx context.Context, user *model.User) error {
	return c.repo.CreateUser(ctx, user)
}

func (c *CachedRepository) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	return c.repo.GetUserByID(ctx, id)
}

func (c *CachedRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return c.repo.GetUserByEmail(ctx, email)
}

func (c *CachedRepository) ListUsers(ctx context.Context) ([]*model.User, error) {
	return c.repo.ListUsers(ctx)
}

func (c *CachedRepository) UpdateUser(ctx context.Context, user *model.User) error {
	return c.repo.UpdateUser(ctx, user)
}

func (c *CachedRepository) DeleteUser(ctx context.Context, id core.ID) error {
	return c.repo.DeleteUser(ctx, id)
}

func (c *CachedRepository) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	return c.repo.CreateAPIKey(ctx, key)
}

func (c *CachedRepository) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	log := logger.FromContext(ctx)
	cacheKey := fmt.Sprintf("auth:apikey:id:%s", id)

	// Try cache first
	cached := c.client.Get(ctx, cacheKey)
	if cached.Err() == nil {
		var key model.APIKey
		if err := json.Unmarshal([]byte(cached.Val()), &key); err == nil {
			log.Debug("API key cache hit", "cache_key", cacheKey)
			return &key, nil
		}
	}

	// Cache miss - fetch from database
	key, err := c.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	keyJSON, err := json.Marshal(key)
	if err != nil {
		log.Error("Failed to marshal API key for cache", "error", err)
		return key, nil // Return the key anyway, just skip caching
	}
	c.client.Set(ctx, cacheKey, keyJSON, c.ttl)

	return key, nil
}

func (c *CachedRepository) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	return c.repo.ListAPIKeysByUserID(ctx, userID)
}

func (c *CachedRepository) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	// Invalidate cache first, but don't fail the operation if cache invalidation fails
	if err := c.invalidateAPIKeyCache(ctx, id); err != nil {
		logger.FromContext(ctx).Warn("cache invalidation failed during update", "error", err)
	}
	return c.repo.UpdateAPIKeyLastUsed(ctx, id)
}

func (c *CachedRepository) DeleteAPIKey(ctx context.Context, id core.ID) error {
	// Invalidate cache first, but don't fail the operation if cache invalidation fails
	if err := c.invalidateAPIKeyCache(ctx, id); err != nil {
		logger.FromContext(ctx).Warn("cache invalidation failed during delete", "error", err)
	}
	return c.repo.DeleteAPIKey(ctx, id)
}
