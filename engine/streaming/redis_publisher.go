package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/redis/go-redis/v9"
)

// RedisPublisher publishes execution events using Redis for fan-out and replay.
type RedisPublisher struct {
	client        redis.UniversalClient
	channelPrefix string
	logPrefix     string
	seqPrefix     string
	maxEntries    int64
	ttl           time.Duration
}

// RedisOptions controls Redis publisher behavior.
type RedisOptions struct {
	ChannelPrefix string
	LogPrefix     string
	SeqPrefix     string
	MaxEntries    int64
	TTL           time.Duration
}

const (
	defaultChannelPrefix = "stream:events:"
	defaultLogPrefix     = "stream:events:log:"
	defaultSeqPrefix     = "stream:events:seq:"
	defaultMaxEntries    = 500
	defaultTTL           = 24 * time.Hour
)

// NewRedisPublisher constructs a Redis-backed event publisher.
func NewRedisPublisher(client redis.UniversalClient, opts *RedisOptions) (*RedisPublisher, error) {
	if client == nil {
		return nil, errors.New("streaming: redis client is required")
	}
	cfg := applyRedisDefaults(opts)
	if cfg.maxEntries <= 0 {
		return nil, fmt.Errorf("streaming: max entries must be > 0 (got %d)", cfg.maxEntries)
	}
	return &RedisPublisher{
		client:        client,
		channelPrefix: cfg.channelPrefix,
		logPrefix:     cfg.logPrefix,
		seqPrefix:     cfg.seqPrefix,
		maxEntries:    cfg.maxEntries,
		ttl:           cfg.ttl,
	}, nil
}

// Publish appends the event to Redis storage and broadcasts it to subscribers.
func (p *RedisPublisher) Publish(ctx context.Context, execID core.ID, event Event) (Envelope, error) {
	if p == nil {
		return Envelope{}, errors.New("streaming: publisher is nil")
	}
	if execID.IsZero() {
		return Envelope{}, errors.New("streaming: exec id is required")
	}
	id, err := p.nextID(ctx, execID)
	if err != nil {
		return Envelope{}, err
	}
	envelope, err := NewEnvelope(id, execID, event, time.Now())
	if err != nil {
		return Envelope{}, err
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return Envelope{}, fmt.Errorf("streaming: marshal envelope: %w", err)
	}
	if err := p.persist(ctx, execID, payload); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

// Replay returns stored events with id greater than afterID in ascending order.
func (p *RedisPublisher) Replay(ctx context.Context, execID core.ID, afterID int64, limit int) ([]Envelope, error) {
	if p == nil {
		return nil, errors.New("streaming: publisher is nil")
	}
	if limit <= 0 || int64(limit) > p.maxEntries {
		limit = int(p.maxEntries)
	}
	values, err := p.client.LRange(ctx, p.logKey(execID), 0, p.maxEntries-1).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("streaming: fetch backlog: %w", err)
	}
	capN := len(values)
	if capN > limit {
		capN = limit
	}
	result := make([]Envelope, 0, capN)
	for i := len(values) - 1; i >= 0; i-- {
		var envelope Envelope
		if err := json.Unmarshal([]byte(values[i]), &envelope); err != nil {
			continue
		}
		if envelope.ID <= afterID {
			continue
		}
		result = append(result, envelope)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

// Channel returns the pub/sub channel for the execution id.
func (p *RedisPublisher) Channel(execID core.ID) string {
	return p.channelPrefix + execID.String()
}

func (p *RedisPublisher) persist(ctx context.Context, execID core.ID, payload []byte) error {
	logKey := p.logKey(execID)
	channel := p.Channel(execID)
	pipe := p.client.TxPipeline()
	pipe.LPush(ctx, logKey, payload)
	pipe.LTrim(ctx, logKey, 0, p.maxEntries-1)
	if p.ttl > 0 {
		pipe.Expire(ctx, logKey, p.ttl)
		pipe.Expire(ctx, p.seqKey(execID), p.ttl)
	}
	pipe.Publish(ctx, channel, payload)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("streaming: persist event: %w", err)
	}
	return nil
}

func (p *RedisPublisher) nextID(ctx context.Context, execID core.ID) (int64, error) {
	id, err := p.client.Incr(ctx, p.seqKey(execID)).Result()
	if err != nil {
		return 0, fmt.Errorf("streaming: increment seq: %w", err)
	}
	return id, nil
}

func (p *RedisPublisher) logKey(execID core.ID) string {
	return p.logPrefix + execID.String()
}

func (p *RedisPublisher) seqKey(execID core.ID) string {
	return p.seqPrefix + execID.String()
}

type redisConfig struct {
	channelPrefix string
	logPrefix     string
	seqPrefix     string
	maxEntries    int64
	ttl           time.Duration
}

func applyRedisDefaults(opts *RedisOptions) redisConfig {
	if opts == nil {
		return redisConfig{
			channelPrefix: defaultChannelPrefix,
			logPrefix:     defaultLogPrefix,
			seqPrefix:     defaultSeqPrefix,
			maxEntries:    defaultMaxEntries,
			ttl:           defaultTTL,
		}
	}
	cfg := redisConfig{
		channelPrefix: chooseOrDefault(opts.ChannelPrefix, defaultChannelPrefix),
		logPrefix:     chooseOrDefault(opts.LogPrefix, defaultLogPrefix),
		seqPrefix:     chooseOrDefault(opts.SeqPrefix, defaultSeqPrefix),
		maxEntries:    opts.MaxEntries,
		ttl:           opts.TTL,
	}
	if cfg.maxEntries <= 0 {
		cfg.maxEntries = defaultMaxEntries
	}
	if cfg.ttl == 0 {
		cfg.ttl = defaultTTL
	}
	return cfg
}

func chooseOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
