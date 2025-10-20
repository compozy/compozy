package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// DispatcherHeartbeatLabel is the activity label for dispatcher heartbeat
const DispatcherHeartbeatLabel = "DispatcherHeartbeat"
const dispatcherHeartbeatKeyPrefix = "dispatcher:heartbeat"

// DispatcherHeartbeatInput contains the input for the heartbeat activity
type DispatcherHeartbeatInput struct {
	DispatcherID string        `json:"dispatcher_id"`
	ProjectName  string        `json:"project_name"`
	ServerID     string        `json:"server_id"`
	TTL          time.Duration `json:"ttl,omitempty"` // Optional: custom TTL, fallback to default if not provided
}

// DispatcherHeartbeatData represents the heartbeat data stored in Redis
type DispatcherHeartbeatData struct {
	DispatcherID  string    `json:"dispatcher_id"`
	ProjectName   string    `json:"project_name"`
	ServerID      string    `json:"server_id"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// DispatcherHeartbeat records a heartbeat for the dispatcher workflow
// Uses cache contracts (KV) rather than direct Redis types.
func DispatcherHeartbeat(ctx context.Context, kv cache.KV, input *DispatcherHeartbeatInput) error {
	log := logger.FromContext(ctx)
	// Validate input
	if input.DispatcherID == "" {
		return fmt.Errorf("dispatcher ID cannot be empty")
	}
	if input.ProjectName == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	log.Debug("Recording dispatcher heartbeat",
		"dispatcher_id", input.DispatcherID,
		"project", input.ProjectName,
		"server_id", input.ServerID)
	if kv == nil {
		return fmt.Errorf("cache KV not configured")
	}
	// Prepare heartbeat data
	data := DispatcherHeartbeatData{
		DispatcherID:  input.DispatcherID,
		ProjectName:   input.ProjectName,
		ServerID:      input.ServerID,
		LastHeartbeat: time.Now(),
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat data: %w", err)
	}
	// Store in Redis with configurable TTL (default 5 minutes)
	// TTL should be significantly longer than heartbeat interval to handle network delays
	key := fmt.Sprintf("%s:%s", dispatcherHeartbeatKeyPrefix, input.DispatcherID)
	ttl := resolveHeartbeatTTL(ctx, input.TTL)
	err = kv.Set(ctx, key, string(jsonData), ttl)
	if err != nil {
		return fmt.Errorf("failed to store heartbeat in Redis: %w", err)
	}
	// Record heartbeat metrics
	interceptor.RecordDispatcherHeartbeat(ctx, input.DispatcherID)
	monitoring.UpdateDispatcherHeartbeat(ctx, input.DispatcherID)
	log.Debug("Dispatcher heartbeat recorded successfully",
		"dispatcher_id", input.DispatcherID,
		"ttl", ttl)
	return nil
}

// ListActiveDispatchersLabel is the activity label for listing active dispatchers
const ListActiveDispatchersLabel = "ListActiveDispatchers"

// ListActiveDispatchersInput contains the input for listing active dispatchers
type ListActiveDispatchersInput struct {
	ProjectName    string        `json:"project_name,omitempty"` // Optional: filter by project
	StaleThreshold time.Duration `json:"stale_threshold"`        // How old before considered stale
}

// ListActiveDispatchersOutput contains the list of active dispatchers
type ListActiveDispatchersOutput struct {
	Dispatchers []DispatcherInfo `json:"dispatchers"`
}

// DispatcherInfo contains information about a dispatcher
type DispatcherInfo struct {
	DispatcherID  string    `json:"dispatcher_id"`
	ProjectName   string    `json:"project_name"`
	ServerID      string    `json:"server_id"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	IsStale       bool      `json:"is_stale"`
	StaleDuration string    `json:"stale_duration,omitempty"`
}

// CacheContracts aggregates the cache interfaces needed by list operation.
// It intentionally relies only on package-level contracts.
type CacheContracts interface {
	cache.KV
	cache.KeysProvider
}

// ListActiveDispatchers returns a list of all active dispatcher workflows
func ListActiveDispatchers(
	ctx context.Context,
	contracts CacheContracts,
	input *ListActiveDispatchersInput,
) (*ListActiveDispatchersOutput, error) {
	log := logger.FromContext(ctx)
	log.Debug("Listing active dispatchers", "project_filter", input.ProjectName)
	if contracts == nil {
		return nil, fmt.Errorf("cache contracts not configured")
	}
	staleThreshold := resolveStaleThreshold(ctx, input.StaleThreshold)
	iterator, err := contracts.Keys(ctx, dispatcherHeartbeatKeyPrefix+":*")
	if err != nil {
		return nil, fmt.Errorf("failed to start key iteration: %w", err)
	}
	scanner := newDispatcherScanner(ctx, contracts, input.ProjectName, staleThreshold, log)
	dispatchers, err := scanner.collect(iterator)
	if err != nil {
		return nil, err
	}
	scanner.recordMetrics(ctx)
	return &ListActiveDispatchersOutput{Dispatchers: dispatchers}, nil
}

// dispatcherScanner walks dispatcher heartbeat keys while tracking metrics.
type dispatcherScanner struct {
	ctx            context.Context
	contracts      CacheContracts
	projectFilter  string
	staleThreshold time.Duration
	now            time.Time
	log            logger.Logger
	totalScanned   int64
	staleFound     int64
	scanStart      time.Time
}

type keyIterator interface {
	Next(context.Context) ([]string, bool, error)
}

func newDispatcherScanner(
	ctx context.Context,
	contracts CacheContracts,
	projectFilter string,
	staleThreshold time.Duration,
	log logger.Logger,
) *dispatcherScanner {
	return &dispatcherScanner{
		ctx:            ctx,
		contracts:      contracts,
		projectFilter:  projectFilter,
		staleThreshold: staleThreshold,
		now:            time.Now(),
		log:            log,
		scanStart:      time.Now(),
	}
}

func (s *dispatcherScanner) collect(iterator keyIterator) ([]DispatcherInfo, error) {
	const (
		maxRetries = 3
		retryDelay = 50 * time.Millisecond
	)
	var dispatchers []DispatcherInfo
	for {
		batch, done, err := iterator.Next(s.ctx)
		if err != nil {
			batch, done, err = s.retryNext(iterator, maxRetries, retryDelay)
			if err != nil {
				return nil, fmt.Errorf("failed during key iteration: %w", err)
			}
		}
		s.totalScanned += int64(len(batch))
		infos, stale := buildDispatchersFromKeys(s.ctx, s.contracts, batch, s.projectFilter, s.staleThreshold, s.now)
		s.staleFound += stale
		dispatchers = append(dispatchers, infos...)
		if done {
			break
		}
	}
	return dispatchers, nil
}

// retryNext reattempts iterator advancement with bounded backoff.
func (s *dispatcherScanner) retryNext(
	iterator keyIterator,
	maxRetries int,
	retryDelay time.Duration,
) ([]string, bool, error) {
	var (
		batch []string
		done  bool
		err   error
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-s.ctx.Done():
			return nil, false, s.ctx.Err()
		case <-time.After(retryDelay * time.Duration(attempt)):
		}
		batch, done, err = iterator.Next(s.ctx)
		if err == nil {
			return batch, done, nil
		}
	}
	return nil, false, err
}

// recordMetrics captures scan statistics for logging and monitoring.
func (s *dispatcherScanner) recordMetrics(ctx context.Context) {
	duration := time.Since(s.scanStart)
	interceptor.RecordDispatcherScan(ctx, s.totalScanned, s.staleFound, duration)
	s.log.Debug("Dispatcher scan complete",
		"keys_scanned", s.totalScanned,
		"stale_found", s.staleFound,
		"duration", duration,
		"project_filter", s.projectFilter)
}

// buildDispatchersFromKeys loads heartbeat data and returns dispatcher infos and stale count.
func buildDispatchersFromKeys(
	ctx context.Context,
	contracts CacheContracts,
	keys []string,
	projectFilter string,
	staleThreshold time.Duration,
	now time.Time,
) ([]DispatcherInfo, int64) {
	log := logger.FromContext(ctx)
	var out []DispatcherInfo
	var staleFound int64
	if len(keys) == 0 {
		return out, 0
	}
	vals, gerr := contracts.MGet(ctx, keys...)
	if gerr != nil {
		log.Warn("Failed to MGET heartbeat data", "count", len(keys), "error", gerr)
		return out, 0
	}
	for i, jsonData := range vals {
		if jsonData == "" {
			continue
		}
		var data DispatcherHeartbeatData
		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
			log.Warn("Failed to unmarshal heartbeat data", "key", keys[i], "error", err)
			continue
		}
		if projectFilter != "" && data.ProjectName != projectFilter {
			continue
		}
		timeSinceHeartbeat := now.Sub(data.LastHeartbeat)
		isStale := timeSinceHeartbeat > staleThreshold
		if isStale {
			staleFound++
		}
		info := DispatcherInfo{
			DispatcherID:  data.DispatcherID,
			ProjectName:   data.ProjectName,
			ServerID:      data.ServerID,
			LastHeartbeat: data.LastHeartbeat,
			IsStale:       isStale,
		}
		if isStale {
			info.StaleDuration = timeSinceHeartbeat.Round(time.Second).String()
		}
		out = append(out, info)
	}
	return out, staleFound
}

// RemoveDispatcherHeartbeat removes a dispatcher's heartbeat using cache contracts
func RemoveDispatcherHeartbeat(ctx context.Context, kv cache.KV, dispatcherID string) error {
	log := logger.FromContext(ctx)
	log.Debug("Removing dispatcher heartbeat", "dispatcher_id", dispatcherID)
	if dispatcherID == "" {
		return fmt.Errorf("dispatcherID cannot be empty")
	}
	if kv == nil {
		return fmt.Errorf("cache KV not configured")
	}
	key := fmt.Sprintf("%s:%s", dispatcherHeartbeatKeyPrefix, dispatcherID)
	_, err := kv.Del(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to remove heartbeat: %w", err)
	}
	return nil
}

const defaultHeartbeatTTL = 5 * time.Minute
const defaultStaleThreshold = 2 * time.Minute

func resolveHeartbeatTTL(ctx context.Context, in time.Duration) time.Duration {
	if in > 0 {
		return in
	}
	if cfg := config.FromContext(ctx); cfg != nil {
		if cfg.Worker.Dispatcher.HeartbeatTTL > 0 {
			return cfg.Worker.Dispatcher.HeartbeatTTL
		}
		if cfg.Runtime.DispatcherHeartbeatTTL > 0 {
			return cfg.Runtime.DispatcherHeartbeatTTL
		}
	}
	return defaultHeartbeatTTL
}

func resolveStaleThreshold(ctx context.Context, in time.Duration) time.Duration {
	if in > 0 {
		return in
	}
	if cfg := config.FromContext(ctx); cfg != nil {
		if cfg.Worker.Dispatcher.StaleThreshold > 0 {
			return cfg.Worker.Dispatcher.StaleThreshold
		}
		if cfg.Runtime.DispatcherStaleThreshold > 0 {
			return cfg.Runtime.DispatcherStaleThreshold
		}
	}
	return defaultStaleThreshold
}
