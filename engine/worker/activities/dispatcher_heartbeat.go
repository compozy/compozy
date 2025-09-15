package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	"github.com/compozy/compozy/pkg/logger"
)

// DispatcherHeartbeatLabel is the activity label for dispatcher heartbeat
const DispatcherHeartbeatLabel = "DispatcherHeartbeat"

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
	key := fmt.Sprintf("dispatcher:heartbeat:%s", input.DispatcherID)
	ttl := 5 * time.Minute
	if input.TTL > 0 {
		ttl = input.TTL
	}
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
	// Default stale threshold is 2 minutes
	staleThreshold := input.StaleThreshold
	if staleThreshold == 0 {
		staleThreshold = 2 * time.Minute
	}
	// Iterate dispatcher heartbeat keys using streaming iterator to avoid buffering
	pattern := "dispatcher:heartbeat:*"
	it, err := contracts.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to start key iteration: %w", err)
	}
	var (
		dispatchers   []DispatcherInfo
		now           = time.Now()
		totalScanned  int64
		staleFound    int64
		scanStartTime = time.Now()
	)
	// Retry parameters for transient Next() errors
	const maxRetries = 3
	const retryDelay = 50 * time.Millisecond
	for {
		batch, done, ierr := it.Next(ctx)
		if ierr != nil {
			// Simple bounded retry with backoff
			var retried bool
			for attempt := 1; attempt <= maxRetries; attempt++ {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(retryDelay * time.Duration(attempt)):
				}
				batch, done, ierr = it.Next(ctx)
				if ierr == nil {
					retried = true
					break
				}
			}
			if !retried && ierr != nil {
				return nil, fmt.Errorf("failed during key iteration: %w", ierr)
			}
		}
		totalScanned += int64(len(batch))
		infos, stale := buildDispatchersFromKeys(ctx, log, contracts, batch, input.ProjectName, staleThreshold, now)
		staleFound += stale
		dispatchers = append(dispatchers, infos...)
		if done {
			break
		}
	}
	// Record metrics and logs
	duration := time.Since(scanStartTime)
	interceptor.RecordDispatcherScan(ctx, totalScanned, staleFound, duration)
	log.Debug("Dispatcher scan complete",
		"keys_scanned", totalScanned,
		"stale_found", staleFound,
		"duration", duration,
		"project_filter", input.ProjectName)
	return &ListActiveDispatchersOutput{Dispatchers: dispatchers}, nil
}

// buildDispatchersFromKeys loads heartbeat data and returns dispatcher infos and stale count.
func buildDispatchersFromKeys(
	ctx context.Context,
	log logger.Logger,
	contracts CacheContracts,
	keys []string,
	projectFilter string,
	staleThreshold time.Duration,
	now time.Time,
) ([]DispatcherInfo, int64) {
	var out []DispatcherInfo
	var staleFound int64
	for _, key := range keys {
		jsonData, gerr := contracts.Get(ctx, key)
		if gerr != nil {
			log.Warn("Failed to get heartbeat data", "key", key, "error", gerr)
			continue
		}
		var data DispatcherHeartbeatData
		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
			log.Warn("Failed to unmarshal heartbeat data", "key", key, "error", err)
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
	key := fmt.Sprintf("dispatcher:heartbeat:%s", dispatcherID)
	_, err := kv.Del(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to remove heartbeat: %w", err)
	}
	return nil
}
