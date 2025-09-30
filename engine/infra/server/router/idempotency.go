package router

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	apiIdempotencyPrefix    = "idempotency:api:execs"
	maxIdempotencyKeyBytes  = 50
	defaultIdempotencyTTL   = 24 * time.Hour
	maxIdempotencyBodyBytes = 1 << 20
)

type APIIdempotency interface {
	CheckAndSet(
		ctx context.Context,
		c *gin.Context,
		namespace string,
		body []byte,
		ttl time.Duration,
	) (bool, string, error)
}

type apiIdempotency struct {
	service webhook.Service
}

func NewAPIIdempotency(service webhook.Service) APIIdempotency {
	return &apiIdempotency{service: service}
}

func (a *apiIdempotency) CheckAndSet(
	ctx context.Context,
	c *gin.Context,
	namespace string,
	body []byte,
	ttl time.Duration,
) (bool, string, error) {
	if a == nil || a.service == nil {
		return false, "", fmt.Errorf("api idempotency service not configured")
	}
	if c == nil || c.Request == nil {
		return false, "", fmt.Errorf("gin context is required")
	}
	cfg := config.FromContext(ctx)
	maxBodyBytes := maxIdempotencyBodyBytes
	if cfg != nil && cfg.Webhooks.DefaultMaxBody > 0 && cfg.Webhooks.DefaultMaxBody < int64(maxBodyBytes) {
		maxBodyBytes = int(cfg.Webhooks.DefaultMaxBody)
	}
	key, err := deriveIdempotencyKey(c, body, maxBodyBytes)
	if err != nil {
		return false, "", err
	}
	effectiveTTL := ttl
	if effectiveTTL <= 0 {
		effectiveTTL = defaultIdempotencyTTL
	}
	finalKey := composeIdempotencyKey(namespace, key)
	err = a.service.CheckAndSet(ctx, finalKey, effectiveTTL)
	if err != nil {
		if errors.Is(err, webhook.ErrDuplicate) {
			log := logger.FromContext(ctx)
			log.Warn("duplicate execution request", "key", finalKey)
			return false, "duplicate", nil
		}
		return false, "", err
	}
	return true, "", nil
}

func deriveIdempotencyKey(c *gin.Context, body []byte, maxBodyBytes int) (string, error) {
	headerKey := strings.TrimSpace(c.GetHeader(webhook.HeaderIdempotencyKey))
	if headerKey != "" {
		if len(headerKey) > maxIdempotencyKeyBytes {
			return "", NewRequestError(http.StatusBadRequest, "idempotency key is too long", nil)
		}
		return headerKey, nil
	}
	if maxBodyBytes > 0 && len(body) > maxBodyBytes {
		return "", NewRequestError(
			http.StatusRequestEntityTooLarge,
			"request body too large for idempotency hashing",
			nil,
		)
	}
	normalizedBody, err := normalizeBody(body)
	if err != nil {
		return "", NewRequestError(http.StatusBadRequest, "invalid request body", err)
	}
	method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
	path := resolveRequestPath(c)
	query := ""
	if c.Request.URL != nil && c.Request.URL.RawQuery != "" {
		query = "?" + c.Request.URL.RawQuery
	}
	rawInput := strings.Join([]string{method, path + query, normalizedBody}, "\n")
	sum := sha256.Sum256([]byte(rawInput))
	return hex.EncodeToString(sum[:]), nil
}

func resolveRequestPath(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if c.Request.URL != nil {
		if path := c.Request.URL.EscapedPath(); path != "" {
			return path
		}
		if c.Request.URL.Path != "" {
			return c.Request.URL.Path
		}
	}
	if uri := c.Request.RequestURI; uri != "" {
		if idx := strings.Index(uri, "?"); idx >= 0 {
			return uri[:idx]
		}
		return uri
	}
	return resolvePathWithParams(c)
}

func resolvePathWithParams(c *gin.Context) string {
	pattern := c.FullPath()
	if pattern == "" {
		return ""
	}
	resolved := pattern
	for _, param := range c.Params {
		placeholder := ":" + param.Key
		if strings.Contains(resolved, placeholder) {
			resolved = strings.ReplaceAll(resolved, placeholder, param.Value)
		}
		wildcard := "*" + param.Key
		if strings.Contains(resolved, wildcard) {
			resolved = strings.ReplaceAll(resolved, wildcard, param.Value)
		}
	}
	return resolved
}

func normalizeBody(body []byte) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", nil
	}
	var payload any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return "", fmt.Errorf("normalize body: %w", err)
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("normalize body: %w", err)
	}
	return string(normalized), nil
}

func composeIdempotencyKey(namespace string, key string) string {
	cleanNamespace := strings.TrimSpace(namespace)
	cleanNamespace = strings.Trim(cleanNamespace, ":")
	if cleanNamespace == "" {
		return apiIdempotencyPrefix + ":" + key
	}
	return apiIdempotencyPrefix + ":" + cleanNamespace + ":" + key
}
