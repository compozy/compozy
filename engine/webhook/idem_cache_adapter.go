package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
)

// cacheRedisClientAdapter adapts cache.RedisInterface to the minimal RedisClient
// used by the idempotency service. This preserves consumer dependency on the
// cache package rather than go-redis types.
type cacheRedisClientAdapter struct{ c cache.RedisInterface }

func (a *cacheRedisClientAdapter) SetNX(
	ctx context.Context,
	key string,
	value any,
	expiration time.Duration,
) (bool, error) {
	return a.c.SetNX(ctx, key, value, expiration).Result()
}

// NewServiceFromCache builds an idempotency Service from cache contracts.
// Pass any cache.RedisInterface-compatible client (e.g., Redis adapter).
func NewServiceFromCache(client cache.RedisInterface) (Service, error) {
	if client == nil {
		return nil, fmt.Errorf("webhook.NewServiceFromCache: redis client is nil")
	}
	return NewRedisService(&cacheRedisClientAdapter{c: client}), nil
}
