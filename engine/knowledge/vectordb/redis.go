package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/compozy/compozy/engine/core"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client    *redis.Client
	setKey    string
	dimension int
	maxTopK   int
}

const (
	redisDefaultMaxTopK     = 1000
	redisTextAttrKey        = "text"
	redisMetadataAttrKey    = "_metadata"
	redisMetadataPrefix     = "meta_"
	redisDefaultVectorKey   = "knowledge_vectors"
	redisFilterEqualsFormat = `%s == "%s"`
)

func newRedisStore(ctx context.Context, cfg *Config) (Store, error) {
	if cfg == nil {
		return nil, errors.New("vector_db config is required")
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		cfg.DSN = resolveRedisDSN(ctx, cfg)
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("redis vector_db %q: connection DSN is required", cfg.ID)
	}
	options, err := parseRedisOptions(cfg)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis vector_db %q: ping failed: %w", cfg.ID, err)
	}
	store := &redisStore{
		client:    client,
		setKey:    determineRedisKey(cfg),
		dimension: cfg.Dimension,
		maxTopK:   chooseRedisMaxTopK(cfg.MaxTopK),
	}
	return store, nil
}

func parseRedisOptions(cfg *Config) (*redis.Options, error) {
	dsn := strings.TrimSpace(cfg.DSN)
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("redis vector_db %q: invalid dsn: %w", cfg.ID, err)
	}
	opt.Protocol = 3
	opt.UnstableResp3 = true
	if opt.Username == "" {
		if user, ok := cfg.Auth["username"]; ok && strings.TrimSpace(user) != "" {
			opt.Username = strings.TrimSpace(user)
		}
	}
	if opt.Password == "" {
		if pass, ok := cfg.Auth["password"]; ok {
			opt.Password = pass
		}
	}
	return opt, nil
}

func resolveRedisDSN(ctx context.Context, cfg *Config) string {
	log := logger.FromContext(ctx)
	globalCfg := appconfig.FromContext(ctx)
	if globalCfg == nil {
		log.Warn("redis vector_db: global config missing for DSN fallback", "vector_db_id", cfg.ID)
		return ""
	}
	if trimmed := strings.TrimSpace(globalCfg.Redis.URL); trimmed != "" {
		log.Debug("redis vector_db: using global redis url fallback", "vector_db_id", cfg.ID)
		return trimmed
	}
	dsn := buildRedisDSNFromConfig(&globalCfg.Redis)
	if strings.TrimSpace(dsn) == "" {
		log.Warn("redis vector_db: global redis config incomplete for DSN fallback", "vector_db_id", cfg.ID)
		return ""
	}
	log.Debug("redis vector_db: built DSN from global redis settings", "vector_db_id", cfg.ID)
	return dsn
}

func buildRedisDSNFromConfig(cfg *appconfig.RedisConfig) string {
	if cfg == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(cfg.URL); trimmed != "" {
		return trimmed
	}
	host := fallbackString(cfg.Host, "localhost")
	port := fallbackString(cfg.Port, "6379")
	scheme := "redis"
	if cfg.TLSEnabled {
		scheme = "rediss"
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	u := &url.URL{
		Scheme: scheme,
		Host:   addr,
		Path:   fmt.Sprintf("/%d", cfg.DB),
	}
	if pwd := strings.TrimSpace(cfg.Password); pwd != "" {
		u.User = url.UserPassword("", pwd)
	}
	return u.String()
}

func determineRedisKey(cfg *Config) string {
	candidates := []string{
		cfg.Collection,
		cfg.Namespace,
		cfg.Index,
		cfg.Table,
		cfg.ID,
	}
	for _, candidate := range candidates {
		if key := sanitizeRedisKey(candidate); key != "" {
			return key
		}
	}
	return redisDefaultVectorKey
}

func sanitizeRedisKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	builder := strings.Builder{}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(unicode.ToLower(r))
		case r == ':', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	key := strings.Trim(builder.String(), "_:-")
	if key == "" {
		return ""
	}
	return key
}

func chooseRedisMaxTopK(maxTopK int) int {
	if maxTopK <= 0 {
		return redisDefaultMaxTopK
	}
	return maxTopK
}

func (r *redisStore) Upsert(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for _, record := range records {
		if len(record.Embedding) != r.dimension {
			return fmt.Errorf("redis: record %q dimension mismatch", record.ID)
		}
		vector := &redis.VectorValues{Val: float32ToFloat64(record.Embedding)}
		pipe.VAdd(ctx, r.setKey, record.ID, vector)
		attrs := buildRedisAttributes(record)
		pipe.VSetAttr(ctx, r.setKey, record.ID, attrs)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis: upsert pipeline: %w", err)
	}
	return nil
}

func (r *redisStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]Match, error) {
	if len(query) != r.dimension {
		return nil, fmt.Errorf("redis: query dimension mismatch")
	}
	count := r.searchCount(opts.TopK)
	args := buildVSimArgs(count, opts.Filters)
	results, err := r.client.VSimWithArgsWithScores(
		ctx,
		r.setKey,
		&redis.VectorValues{Val: float32ToFloat64(query)},
		args,
	).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis: similarity search: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}
	payloads, err := r.loadAttributePayloads(ctx, results)
	if err != nil {
		return nil, err
	}
	return buildMatchesFromPayloads(results, payloads, opts.MinScore)
}

func (r *redisStore) Delete(ctx context.Context, filter Filter) error {
	targets := make(map[string]struct{}, len(filter.IDs))
	for _, id := range filter.IDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			targets[trimmed] = struct{}{}
		}
	}
	if len(filter.Metadata) > 0 {
		ids, err := r.lookupIDsByMetadata(ctx, filter.Metadata)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				targets[trimmed] = struct{}{}
			}
		}
	}
	if len(targets) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for id := range targets {
		pipe.VRem(ctx, r.setKey, id)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis: delete vectors: %w", err)
	}
	return nil
}

func (r *redisStore) lookupIDsByMetadata(ctx context.Context, metadata map[string]string) ([]string, error) {
	filter := buildRedisFilter(metadata)
	if filter == "" {
		return nil, nil
	}
	total, err := r.client.VCard(ctx, r.setKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis: vcard: %w", err)
	}
	if total == 0 {
		return nil, nil
	}
	args := &redis.VSimArgs{
		Count:  total,
		Filter: filter,
	}
	zero := make([]float64, r.dimension)
	names, err := r.client.VSimWithArgs(
		ctx,
		r.setKey,
		&redis.VectorValues{Val: zero},
		args,
	).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis: metadata filter query: %w", err)
	}
	return names, nil
}

func (r *redisStore) Close(context.Context) error {
	return r.client.Close()
}

func (r *redisStore) searchCount(topK int) int {
	count := topK
	if count <= 0 {
		count = defaultTopK
	}
	if r.maxTopK > 0 && count > r.maxTopK {
		count = r.maxTopK
	}
	return count
}

func buildVSimArgs(count int, filters map[string]string) *redis.VSimArgs {
	args := &redis.VSimArgs{
		Count: int64(count),
	}
	if filter := buildRedisFilter(filters); filter != "" {
		args.Filter = filter
	}
	return args
}

func (r *redisStore) loadAttributePayloads(
	ctx context.Context,
	results []redis.VectorScore,
) ([]string, error) {
	pipe := r.client.Pipeline()
	attrCmds := make([]*redis.StringCmd, len(results))
	for i := range results {
		attrCmds[i] = pipe.VGetAttr(ctx, r.setKey, results[i].Name)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("redis: fetch attributes: %w", err)
	}
	payloads := make([]string, len(results))
	for i := range attrCmds {
		raw, err := attrCmds[i].Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				payloads[i] = ""
				continue
			}
			return nil, fmt.Errorf("redis: read attributes for %q: %w", results[i].Name, err)
		}
		payloads[i] = raw
	}
	return payloads, nil
}

func buildMatchesFromPayloads(
	results []redis.VectorScore,
	payloads []string,
	minScore float64,
) ([]Match, error) {
	matches := make([]Match, 0, len(results))
	for i, item := range results {
		if minScore > 0 && item.Score < minScore {
			continue
		}
		if i >= len(payloads) || payloads[i] == "" {
			continue
		}
		match, err := buildMatchFromAttributes(item.Name, item.Score, payloads[i])
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matches, nil
}

func float32ToFloat64(values []float32) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		out[i] = float64(values[i])
	}
	return out
}

func buildRedisAttributes(record Record) map[string]any {
	attrs := make(map[string]any, len(record.Metadata)+2)
	attrs[redisTextAttrKey] = record.Text
	meta := core.CloneMap(record.Metadata)
	if meta == nil {
		meta = make(map[string]any)
	}
	attrs[redisMetadataAttrKey] = meta
	for key, value := range record.Metadata {
		attrKey := metadataAttributeKey(key)
		attrs[attrKey] = fmt.Sprint(value)
	}
	return attrs
}

func metadataAttributeKey(key string) string {
	return redisMetadataPrefix + sanitizeAttributeKey(key)
}

func sanitizeAttributeKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "unknown"
	}
	builder := strings.Builder{}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(unicode.ToLower(r))
		default:
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "unknown"
	}
	return result
}

func buildRedisFilter(filters map[string]string) string {
	if len(filters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := filters[key]
		attr := "." + metadataAttributeKey(key)
		parts = append(parts, fmt.Sprintf(redisFilterEqualsFormat, attr, escapeFilterValue(value)))
	}
	return strings.Join(parts, " && ")
}

func escapeFilterValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}

func buildMatchFromAttributes(id string, score float64, attrJSON string) (Match, error) {
	text, metadata, err := parseAttributeJSON(attrJSON)
	if err != nil {
		return Match{}, fmt.Errorf("redis: parse attributes for %q: %w", id, err)
	}
	return Match{
		ID:       id,
		Score:    score,
		Text:     text,
		Metadata: metadata,
	}, nil
}

func parseAttributeJSON(payload string) (string, map[string]any, error) {
	if strings.TrimSpace(payload) == "" {
		return "", make(map[string]any), nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return "", nil, err
	}
	text := ""
	if value, ok := decoded[redisTextAttrKey].(string); ok {
		text = value
	}
	meta := make(map[string]any)
	if raw, ok := decoded[redisMetadataAttrKey]; ok && raw != nil {
		switch typed := raw.(type) {
		case map[string]any:
			meta = typed
		default:
			bytes, err := json.Marshal(typed)
			if err == nil {
				tmp := make(map[string]any)
				if unmarshalErr := json.Unmarshal(bytes, &tmp); unmarshalErr == nil {
					meta = tmp
				}
			}
		}
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	return text, meta, nil
}
