package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	sdk "github.com/echovault/sugardb/sugardb"
)

// sugarConfigStore implements ConfigStore using embedded SugarDB for standalone mode.
type sugarConfigStore struct {
	db  *sdk.SugarDB
	ttl time.Duration
}

// NewSugarConfigStore creates a new SugarDB-backed config store.
func NewSugarConfigStore(db *sdk.SugarDB, ttl time.Duration) ConfigStore {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &sugarConfigStore{db: db, ttl: ttl}
}

// Save persists a task configuration with TTL.
func (s *sugarConfigStore) Save(ctx context.Context, taskExecID string, cfg *task.Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	opt := sdk.SETOptions{}
	if s.ttl > 0 {
		opt.ExpireOpt = sdk.SETPX
		opt.ExpireTime = int(s.ttl.Milliseconds())
	}
	if _, _, err := s.db.Set(ConfigKeyPrefix+taskExecID, string(data), opt); err != nil {
		return fmt.Errorf("sugardb set failed: %w", err)
	}
	logger.FromContext(ctx).With("task_exec_id", taskExecID, "ttl", s.ttl).Debug("Task config saved to SugarDB")
	return nil
}

// Get retrieves a configuration and extends its TTL best-effort.
func (s *sugarConfigStore) Get(ctx context.Context, taskExecID string) (*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	if taskExecID == "" {
		return nil, fmt.Errorf("taskExecID cannot be empty")
	}
	key := ConfigKeyPrefix + taskExecID
	// Prefer GETEX if available in the SDK to extend TTL atomically.
	var (
		val string
		err error
	)
	if s.ttl > 0 {
		// GETEX with PX semantics; SugarDB uses seconds for EX and milliseconds for PX.
		val, err = s.db.GetEx(key, sdk.PX, int(s.ttl.Milliseconds()))
	} else {
		val, err = s.db.Get(key)
	}
	if err != nil {
		return nil, fmt.Errorf("sugardb get failed: %w", err)
	}
	if val == "" {
		return nil, fmt.Errorf("config not found for taskExecID %s", taskExecID)
	}
	var cfg task.Config
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	// GETEX already extends TTL when s.ttl > 0
	logger.FromContext(ctx).
		With("task_exec_id", taskExecID, "ttl_extended", s.ttl).
		Debug("Task config retrieved from SugarDB with TTL extended")
	return &cfg, nil
}

// Delete removes a stored configuration.
func (s *sugarConfigStore) Delete(ctx context.Context, taskExecID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	n, err := s.db.Del(ConfigKeyPrefix + taskExecID)
	if err != nil {
		return fmt.Errorf("sugardb del failed: %w", err)
	}
	if n == 0 {
		logger.FromContext(ctx).With("task_exec_id", taskExecID).Debug("Task config not found for deletion (SugarDB)")
	} else {
		logger.FromContext(ctx).With("task_exec_id", taskExecID).Debug("Task config deleted from SugarDB")
	}
	return nil
}

// SaveMetadata stores arbitrary metadata with TTL.
func (s *sugarConfigStore) SaveMetadata(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	opt := sdk.SETOptions{}
	if s.ttl > 0 {
		opt.ExpireOpt = sdk.SETPX
		opt.ExpireTime = int(s.ttl.Milliseconds())
	}
	if _, _, err := s.db.Set(MetadataKeyPrefix+key, string(data), opt); err != nil {
		return fmt.Errorf("sugardb set metadata failed: %w", err)
	}
	logger.FromContext(ctx).With("metadata_key", key, "ttl", s.ttl).Debug("Metadata saved to SugarDB")
	return nil
}

// GetMetadata retrieves metadata and extends TTL.
func (s *sugarConfigStore) GetMetadata(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	k := MetadataKeyPrefix + key
	var (
		val string
		err error
	)
	if s.ttl > 0 {
		val, err = s.db.GetEx(k, sdk.PX, int(s.ttl.Milliseconds()))
	} else {
		val, err = s.db.Get(k)
	}
	if err != nil {
		return nil, fmt.Errorf("sugardb get metadata failed: %w", err)
	}
	if val == "" {
		return nil, fmt.Errorf("metadata not found for key %s", key)
	}
	// GETEX already extends TTL when s.ttl > 0
	logger.FromContext(ctx).
		With("metadata_key", key, "ttl_extended", s.ttl).
		Debug("Metadata retrieved from SugarDB with TTL extended")
	return []byte(val), nil
}

// DeleteMetadata removes stored metadata.
func (s *sugarConfigStore) DeleteMetadata(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	n, err := s.db.Del(MetadataKeyPrefix + key)
	if err != nil {
		return fmt.Errorf("sugardb del metadata failed: %w", err)
	}
	if n == 0 {
		logger.FromContext(ctx).With("metadata_key", key).Debug("Metadata not found for deletion (SugarDB)")
	} else {
		logger.FromContext(ctx).With("metadata_key", key).Debug("Metadata deleted from SugarDB")
	}
	return nil
}

// Close closes the underlying DB if it implements Close; otherwise no-op.
func (s *sugarConfigStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if closer, ok := any(s.db).(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// HealthCheck performs a simple R/W cycle.
func (s *sugarConfigStore) HealthCheck(_ context.Context) error {
	testKey := "cfg:healthcheck"
	if _, _, err := s.db.Set(testKey, "ok", sdk.SETOptions{ExpireOpt: sdk.SETPX, ExpireTime: 500}); err != nil {
		return err
	}
	v, err := s.db.Get(testKey)
	if err != nil {
		return err
	}
	if v == "" {
		return fmt.Errorf("health key missing after set")
	}
	if _, err := s.db.Del(testKey); err != nil {
		return err
	}
	return nil
}

// GetKeys returns keys matching a pattern. SugarDB supports KEYS(pattern).
func (s *sugarConfigStore) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	// Key iteration is not guaranteed across SugarDB versions; return empty set and warn.
	logger.FromContext(ctx).Warn("GetKeys not implemented for SugarDB store; returning empty list", "pattern", pattern)
	return []string{}, nil
}

func (s *sugarConfigStore) GetAllConfigKeys(ctx context.Context) ([]string, error) {
	return s.GetKeys(ctx, ConfigKeyPrefix+"*")
}

func (s *sugarConfigStore) GetAllMetadataKeys(ctx context.Context) ([]string, error) {
	return s.GetKeys(ctx, MetadataKeyPrefix+"*")
}

// ExtendTTL updates the TTL for a given configuration key.
func (s *sugarConfigStore) ExtendTTL(ctx context.Context, taskExecID string, ttl time.Duration) error {
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	if ttl <= 0 {
		// Persist key (no expiration)
		ok, err := s.db.Persist(ConfigKeyPrefix + taskExecID)
		if err != nil {
			return fmt.Errorf("sugardb persist failed: %w", err)
		}
		if !ok {
			return fmt.Errorf("config not found for taskExecID %s", taskExecID)
		}
		return nil
	}
	ok, err := s.db.PExpire(ConfigKeyPrefix+taskExecID, int(ttl.Milliseconds()))
	if err != nil {
		return fmt.Errorf("sugardb pexpire failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("config not found for taskExecID %s", taskExecID)
	}
	logger.FromContext(ctx).With("task_exec_id", taskExecID, "new_ttl", ttl).Debug("Task config TTL extended (SugarDB)")
	return nil
}

// GetTTL returns the remaining TTL for a configuration key.
func (s *sugarConfigStore) GetTTL(_ context.Context, taskExecID string) (time.Duration, error) {
	if taskExecID == "" {
		return 0, fmt.Errorf("taskExecID cannot be empty")
	}
	ttl, err := s.db.PTTL(ConfigKeyPrefix + taskExecID)
	if err != nil {
		return 0, fmt.Errorf("sugardb pttl failed: %w", err)
	}
	// SugarDB PTTL returns milliseconds
	if ttl < 0 {
		return 0, nil
	}
	return time.Duration(ttl) * time.Millisecond, nil
}
