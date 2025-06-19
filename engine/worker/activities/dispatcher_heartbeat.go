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
	DispatcherID string `json:"dispatcher_id"`
	ProjectName  string `json:"project_name"`
	ServerID     string `json:"server_id"`
}

// DispatcherHeartbeatData represents the heartbeat data stored in Redis
type DispatcherHeartbeatData struct {
	DispatcherID  string    `json:"dispatcher_id"`
	ProjectName   string    `json:"project_name"`
	ServerID      string    `json:"server_id"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// DispatcherHeartbeat records a heartbeat for the dispatcher workflow
func DispatcherHeartbeat(ctx context.Context, cache *cache.Cache, input *DispatcherHeartbeatInput) error {
	log := logger.FromContext(ctx)
	log.Debug("Recording dispatcher heartbeat",
		"dispatcher_id", input.DispatcherID,
		"project", input.ProjectName,
		"server_id", input.ServerID)
	if cache == nil {
		return fmt.Errorf("redis cache not configured")
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
	// Store in Redis with 5 minute TTL
	key := fmt.Sprintf("dispatcher:heartbeat:%s", input.DispatcherID)
	ttl := 5 * time.Minute
	err = cache.Redis.Set(ctx, key, string(jsonData), ttl).Err()
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

// ListActiveDispatchers returns a list of all active dispatcher workflows
func ListActiveDispatchers(
	ctx context.Context,
	cache *cache.Cache,
	input *ListActiveDispatchersInput,
) (*ListActiveDispatchersOutput, error) {
	log := logger.FromContext(ctx)
	log.Debug("Listing active dispatchers", "project_filter", input.ProjectName)
	if cache == nil {
		return nil, fmt.Errorf("redis cache not configured")
	}
	// Default stale threshold is 2 minutes
	staleThreshold := input.StaleThreshold
	if staleThreshold == 0 {
		staleThreshold = 2 * time.Minute
	}
	// Get all dispatcher heartbeat keys using SCAN (production-safe alternative to KEYS)
	pattern := "dispatcher:heartbeat:*"
	var keys []string
	iter := cache.Redis.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan for dispatcher keys: %w", err)
	}
	var dispatchers []DispatcherInfo
	now := time.Now()
	for _, key := range keys {
		// Get heartbeat data
		jsonData, err := cache.Redis.Get(ctx, key).Result()
		if err != nil {
			log.Warn("Failed to get heartbeat data", "key", key, "error", err)
			continue
		}
		var data DispatcherHeartbeatData
		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
			log.Warn("Failed to unmarshal heartbeat data", "key", key, "error", err)
			continue
		}
		// Apply project filter if specified
		if input.ProjectName != "" && data.ProjectName != input.ProjectName {
			continue
		}
		// Check if stale
		timeSinceHeartbeat := now.Sub(data.LastHeartbeat)
		isStale := timeSinceHeartbeat > staleThreshold
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
		dispatchers = append(dispatchers, info)
	}
	log.Debug("Found dispatchers",
		"total", len(dispatchers),
		"project_filter", input.ProjectName)
	return &ListActiveDispatchersOutput{Dispatchers: dispatchers}, nil
}

// RemoveDispatcherHeartbeat removes a dispatcher's heartbeat from Redis
func RemoveDispatcherHeartbeat(ctx context.Context, cache *cache.Cache, dispatcherID string) error {
	log := logger.FromContext(ctx)
	log.Debug("Removing dispatcher heartbeat", "dispatcher_id", dispatcherID)
	if cache == nil {
		return fmt.Errorf("redis cache not configured")
	}
	key := fmt.Sprintf("dispatcher:heartbeat:%s", dispatcherID)
	err := cache.Redis.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to remove heartbeat: %w", err)
	}
	return nil
}
