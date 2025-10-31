package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	rds "github.com/redis/go-redis/v9"
)

// CachedRepository decorates an auth repository with Redis-backed caching.
// Security: never caches sensitive hash bytes. For validation lookups
// (GetAPIKeyByFingerprint), only a fingerprint->ID mapping is cached; the full
// record is always fetched from the underlying repository by ID.
type CachedRepository struct {
	repo   uc.Repository
	client *rds.Client
	ttl    time.Duration
}

const defaultAuthCacheTTL = 30 * time.Second

// NewCachedRepository returns a Redis-backed caching decorator.
func NewCachedRepository(repo uc.Repository, client *rds.Client, ttl time.Duration) uc.Repository {
	if ttl <= 0 {
		ttl = defaultAuthCacheTTL
	}
	return &CachedRepository{repo: repo, client: client, ttl: ttl}
}

func (c *CachedRepository) idKey(id core.ID) string { return "auth:apikey:id:" + id.String() }
func (c *CachedRepository) fpKey(fp []byte) string  { return "auth:apikey:fp:" + hex.EncodeToString(fp) }
func (c *CachedRepository) sanitize(k *model.APIKey) *model.APIKey {
	if k == nil {
		return nil
	}
	cp := *k
	cp.Hash = nil        // never cache sensitive hash bytes
	cp.Fingerprint = nil // avoid storing fingerprint in ID cache
	return &cp
}

// --- Cache-aware API Key methods ---

func (c *CachedRepository) GetAPIKeyByFingerprint(ctx context.Context, fingerprint []byte) (*model.APIKey, error) {
	log := logger.FromContext(ctx)
	if c.client != nil {
		if s, err := c.client.Get(ctx, c.fpKey(fingerprint)).Result(); err == nil && s != "" {
			id, perr := core.ParseID(s)
			if perr == nil {
				key, gerr := c.repo.GetAPIKeyByID(ctx, id)
				if gerr == nil {
					return key, nil
				}
				if derr := c.client.Del(ctx, c.fpKey(fingerprint)).Err(); derr != nil {
					log.Warn("redis: del fp cache failed", "error", derr)
				}
				log.Debug("repo.GetAPIKeyByID after fp mapping failed; evicted mapping", "error", gerr)
			} else {
				if derr := c.client.Del(ctx, c.fpKey(fingerprint)).Err(); derr != nil {
					log.Warn("redis: del fp cache failed", "error", derr)
				}
				log.Debug("redis: invalid fp->id mapping; evicted", "value", s, "error", perr)
			}
		}
	}
	key, err := c.repo.GetAPIKeyByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, err
	}
	if c.client != nil {
		if err := c.client.Set(ctx, c.fpKey(fingerprint), key.ID.String(), c.ttl).Err(); err != nil {
			log.Warn("redis: set fp->id mapping failed", "error", err)
		}
	}
	return key, nil
}

func (c *CachedRepository) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	log := logger.FromContext(ctx)
	if c.client != nil {
		if s, err := c.client.Get(ctx, c.idKey(id)).Result(); err == nil && s != "" {
			var masked model.APIKey
			uerr := json.Unmarshal([]byte(s), &masked)
			if uerr == nil {
				return &masked, nil
			}
			if derr := c.client.Del(ctx, c.idKey(id)).Err(); derr != nil {
				log.Warn("redis: del id cache failed", "error", derr)
			}
			log.Debug("redis: unmarshal cached api key failed; evicted", "error", uerr)
		}
	}
	key, err := c.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.client != nil {
		if b, jerr := json.Marshal(c.sanitize(key)); jerr == nil {
			if err := c.client.Set(ctx, c.idKey(id), b, c.ttl).Err(); err != nil {
				log.Warn("redis: set id cache failed", "error", err)
			}
		}
	}
	return key, nil
}

func (c *CachedRepository) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	if err := c.repo.UpdateAPIKeyLastUsed(ctx, id); err != nil {
		return err
	}
	if c.client != nil {
		if err := c.client.Del(ctx, c.idKey(id)).Err(); err != nil {
			logger.FromContext(ctx).Warn("redis: del id cache failed", "error", err)
		}
	}
	return nil
}

func (c *CachedRepository) DeleteAPIKey(ctx context.Context, id core.ID) error {
	var k *model.APIKey
	if c.client != nil {
		// best effort: capture fingerprint before deletion for cache eviction
		if key, err := c.repo.GetAPIKeyByID(ctx, id); err == nil {
			k = key
		}
	}
	if err := c.repo.DeleteAPIKey(ctx, id); err != nil {
		return err
	}
	if c.client != nil {
		if err := c.client.Del(ctx, c.idKey(id)).Err(); err != nil {
			logger.FromContext(ctx).Warn("redis: del id cache failed", "error", err)
		}
	}
	if c.client != nil && k != nil && len(k.Fingerprint) > 0 {
		if err := c.client.Del(ctx, c.fpKey(k.Fingerprint)).Err(); err != nil {
			logger.FromContext(ctx).Warn("redis: del fp cache failed", "error", err)
		}
	}
	return nil
}

// --- Delegated methods (no caching) ---

func (c *CachedRepository) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	return c.repo.CreateAPIKey(ctx, key)
}
func (c *CachedRepository) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	return c.repo.ListAPIKeysByUserID(ctx, userID)
}

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
func (c *CachedRepository) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	return c.repo.CreateInitialAdminIfNone(ctx, user)
}
