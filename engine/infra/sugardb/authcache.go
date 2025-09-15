package sugardb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdk "github.com/echovault/sugardb/sugardb"
)

// AuthCachedRepository decorates an auth repository with SugarDB-backed caching.
// Security posture: never caches hash bytes; only caches sanitized API key by ID
// and fingerprint->ID mapping used to speed up lookups.
type AuthCachedRepository struct {
	repo uc.Repository
	db   *sdk.SugarDB
	ttl  time.Duration
}

func NewAuthCachedRepository(repo uc.Repository, db *sdk.SugarDB, ttl time.Duration) uc.Repository {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &AuthCachedRepository{repo: repo, db: db, ttl: ttl}
}

func (c *AuthCachedRepository) idKey(id core.ID) string { return "auth:apikey:id:" + id.String() }
func (c *AuthCachedRepository) fpKey(fp []byte) string {
	return "auth:apikey:fp:" + hex.EncodeToString(fp)
}
func (c *AuthCachedRepository) sanitize(k *model.APIKey) *model.APIKey {
	cp := *k
	cp.Hash = nil
	return &cp
}

// --- helpers ---
func (c *AuthCachedRepository) get(_ context.Context, key string) (string, error) {
	vals, err := c.db.MGet(key)
	if err != nil {
		return "", err
	}
	if len(vals) == 0 || vals[0] == "" || vals[0] == "nil" || vals[0] == "(nil)" || vals[0] == "<nil>" {
		return "", fmt.Errorf("not found")
	}
	return vals[0], nil
}

func (c *AuthCachedRepository) set(ctx context.Context, key, value string) {
	opt := sdk.SETOptions{}
	if c.ttl > 0 {
		opt.ExpireOpt = sdk.SETPX
		opt.ExpireTime = int(c.ttl.Milliseconds())
	}
	if _, _, err := c.db.Set(key, value, opt); err != nil {
		logger.FromContext(ctx).Warn("sugardb: set cache failed", "error", err)
	}
}

func (c *AuthCachedRepository) del(ctx context.Context, keys ...string) {
	if _, err := c.db.Del(keys...); err != nil {
		logger.FromContext(ctx).Warn("sugardb: del cache failed", "error", err)
	}
}

// --- cache-aware api key methods ---

func (c *AuthCachedRepository) GetAPIKeyByHash(ctx context.Context, fingerprint []byte) (*model.APIKey, error) {
	log := logger.FromContext(ctx)
	if s, err := c.get(ctx, c.fpKey(fingerprint)); err == nil && s != "" {
		if id, perr := core.ParseID(s); perr == nil {
			if key, gerr := c.repo.GetAPIKeyByID(ctx, id); gerr == nil {
				return key, nil
			}
			log.Debug("sugardb: repo.GetAPIKeyByID after fp mapping failed")
		}
	}
	key, err := c.repo.GetAPIKeyByHash(ctx, fingerprint)
	if err != nil {
		return nil, err
	}
	c.set(ctx, c.fpKey(fingerprint), key.ID.String())
	return key, nil
}

func (c *AuthCachedRepository) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	if s, err := c.get(ctx, c.idKey(id)); err == nil && s != "" {
		var masked model.APIKey
		if json.Unmarshal([]byte(s), &masked) == nil {
			return &masked, nil
		}
	}
	key, err := c.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b, jerr := json.Marshal(c.sanitize(key)); jerr == nil {
		c.set(ctx, c.idKey(id), string(b))
	}
	return key, nil
}

func (c *AuthCachedRepository) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	if err := c.repo.UpdateAPIKeyLastUsed(ctx, id); err != nil {
		return err
	}
	c.del(ctx, c.idKey(id))
	return nil
}

func (c *AuthCachedRepository) DeleteAPIKey(ctx context.Context, id core.ID) error {
	if err := c.repo.DeleteAPIKey(ctx, id); err != nil {
		return err
	}
	c.del(ctx, c.idKey(id))
	if k, err := c.repo.GetAPIKeyByID(ctx, id); err == nil {
		c.del(ctx, c.fpKey(k.Fingerprint))
	}
	return nil
}

// --- delegated methods ---
func (c *AuthCachedRepository) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	return c.repo.CreateAPIKey(ctx, key)
}
func (c *AuthCachedRepository) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	return c.repo.ListAPIKeysByUserID(ctx, userID)
}

func (c *AuthCachedRepository) CreateUser(ctx context.Context, user *model.User) error {
	return c.repo.CreateUser(ctx, user)
}
func (c *AuthCachedRepository) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	return c.repo.GetUserByID(ctx, id)
}
func (c *AuthCachedRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return c.repo.GetUserByEmail(ctx, email)
}
func (c *AuthCachedRepository) ListUsers(ctx context.Context) ([]*model.User, error) {
	return c.repo.ListUsers(ctx)
}
func (c *AuthCachedRepository) UpdateUser(ctx context.Context, user *model.User) error {
	return c.repo.UpdateUser(ctx, user)
}
func (c *AuthCachedRepository) DeleteUser(ctx context.Context, id core.ID) error {
	return c.repo.DeleteUser(ctx, id)
}
func (c *AuthCachedRepository) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	return c.repo.CreateInitialAdminIfNone(ctx, user)
}
