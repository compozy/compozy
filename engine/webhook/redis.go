package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrDuplicate = errors.New("duplicate request")
var ErrKeyNotFound = errors.New("idempotency key not found")

type Service interface {
	CheckAndSet(ctx context.Context, key string, ttl time.Duration) error
}

type RedisClient interface {
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error)
}

type redisSvc struct {
	client RedisClient
}

func NewRedisService(client RedisClient) Service {
	return &redisSvc{client: client}
}

func (s *redisSvc) CheckAndSet(ctx context.Context, key string, ttl time.Duration) error {
	ok, err := s.client.SetNX(ctx, key, 1, ttl)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDuplicate
	}
	return nil
}

const HeaderIdempotencyKey = "X-Idempotency-Key"
const idempotencyPrefix = "idempotency:webhook:"

func DeriveKey(h http.Header, body []byte, jsonField string) (string, error) {
	if k := strings.TrimSpace(h.Get(HeaderIdempotencyKey)); k != "" {
		return k, nil
	}
	if jsonField == "" {
		return "", ErrKeyNotFound
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", fmt.Errorf("derive idempotency key: invalid json: %w", err)
	}
	v, ok := m[jsonField]
	if !ok {
		return "", ErrKeyNotFound
	}
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return "", ErrKeyNotFound
		}
		return t, nil
	default:
		return fmt.Sprint(t), nil
	}
}

func KeyWithNamespace(namespace, key string) string {
	if strings.TrimSpace(namespace) == "" {
		return idempotencyPrefix + key
	}
	return idempotencyPrefix + namespace + ":" + key
}
