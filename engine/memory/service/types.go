package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/workflow"
)

// ValidatePayloadType validates that the payload is one of the supported types.
// Supported types: string, map[string]any, []map[string]any, []any
func ValidatePayloadType(payload any) error {
	if payload == nil {
		return fmt.Errorf("payload cannot be nil")
	}
	switch payload.(type) {
	case string, map[string]any, []map[string]any, []any:
		return nil
	default:
		return fmt.Errorf("unsupported payload type: %T, supported types: string, map[string]any, []map[string]any, []any",
			payload)
	}
}

// MemoryOperationsService provides centralized memory operations
type MemoryOperationsService interface {
	// Core operations
	Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error)
	ReadPaginated(ctx context.Context, req *ReadPaginatedRequest) (*ReadPaginatedResponse, error)
	Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error)
	Append(ctx context.Context, req *AppendRequest) (*AppendResponse, error)
	Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error)

	// Advanced operations
	Flush(ctx context.Context, req *FlushRequest) (*FlushResponse, error)
	Clear(ctx context.Context, req *ClearRequest) (*ClearResponse, error)
	Health(ctx context.Context, req *HealthRequest) (*HealthResponse, error)
	Stats(ctx context.Context, req *StatsRequest) (*StatsResponse, error)
}

// BaseRequest contains common fields for all requests
type BaseRequest struct {
	MemoryRef string `json:"memory_ref"`
	Key       string `json:"key"`
}

// ReadRequest represents a memory read operation
type ReadRequest struct {
	BaseRequest
}

// ReadResponse contains the result of a read operation
type ReadResponse struct {
	Messages []llm.Message `json:"messages"`
	Count    int           `json:"count"`
	Key      string        `json:"key"`
}

// ReadPaginatedRequest represents a memory read operation with pagination
type ReadPaginatedRequest struct {
	BaseRequest
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// ReadPaginatedResponse contains the result of a paginated read operation
type ReadPaginatedResponse struct {
	Messages   []llm.Message `json:"messages"`
	Count      int           `json:"count"`
	TotalCount int           `json:"total_count"`
	Offset     int           `json:"offset"`
	Limit      int           `json:"limit"`
	HasMore    bool          `json:"has_more"`
	Key        string        `json:"key"`
}

// WriteRequest represents a memory write operation
type WriteRequest struct {
	BaseRequest
	// Payload contains the data to write to memory.
	// Supported types: string, map[string]any, []map[string]any, []any
	// Use ValidatePayloadType() to validate before use.
	Payload       any             `json:"payload"`
	MergedInput   *core.Input     `json:"merged_input,omitempty"`
	WorkflowState *workflow.State `json:"workflow_state,omitempty"`
}

// WriteResponse contains the result of a write operation
type WriteResponse struct {
	Success bool   `json:"success"`
	Count   int    `json:"count"`
	Key     string `json:"key"`
}

// AppendRequest represents a memory append operation
type AppendRequest struct {
	BaseRequest
	// Payload contains the data to append to memory.
	// Supported types: string, map[string]any, []map[string]any, []any
	// Use ValidatePayloadType() to validate before use.
	Payload       any             `json:"payload"`
	MergedInput   *core.Input     `json:"merged_input,omitempty"`
	WorkflowState *workflow.State `json:"workflow_state,omitempty"`
}

// AppendResponse contains the result of an append operation
type AppendResponse struct {
	Success    bool   `json:"success"`
	Appended   int    `json:"appended"`
	TotalCount int    `json:"total_count"`
	Key        string `json:"key"`
}

// DeleteRequest represents a memory delete operation
type DeleteRequest struct {
	BaseRequest
}

// DeleteResponse contains the result of a delete operation
type DeleteResponse struct {
	Success bool   `json:"success"`
	Key     string `json:"key"`
}

// FlushRequest represents a memory flush operation
type FlushRequest struct {
	BaseRequest
	Config *FlushConfig `json:"config,omitempty"`
}

// FlushResponse contains the result of a flush operation
type FlushResponse struct {
	Success          bool   `json:"success"`
	Key              string `json:"key"`
	SummaryGenerated bool   `json:"summary_generated"`
	MessageCount     int    `json:"message_count"`
	TokenCount       int    `json:"token_count"`
	DryRun           bool   `json:"dry_run,omitempty"`
	WouldFlush       bool   `json:"would_flush,omitempty"`
	ActualStrategy   string `json:"actual_strategy"`
	Error            string `json:"error,omitempty"`
}

// ClearRequest represents a memory clear operation
type ClearRequest struct {
	BaseRequest
	Config *ClearConfig `json:"config,omitempty"`
}

// ClearResponse contains the result of a clear operation
type ClearResponse struct {
	Success         bool   `json:"success"`
	Key             string `json:"key"`
	MessagesCleared int    `json:"messages_cleared"`
	BackupCreated   bool   `json:"backup_created"`
}

// HealthRequest represents a memory health check operation
type HealthRequest struct {
	BaseRequest
	Config *HealthConfig `json:"config,omitempty"`
}

// HealthResponse contains the result of a health check
type HealthResponse struct {
	Healthy        bool   `json:"healthy"`
	Key            string `json:"key"`
	TokenCount     int    `json:"token_count"`
	MessageCount   int    `json:"message_count"`
	ActualStrategy string `json:"actual_strategy"`
	LastFlush      string `json:"last_flush,omitempty"`
	CurrentTokens  int    `json:"current_tokens,omitempty"`
}

// StatsRequest represents a memory stats operation
type StatsRequest struct {
	BaseRequest
	Config *StatsConfig `json:"config,omitempty"`
}

// StatsResponse contains the result of a stats operation
type StatsResponse struct {
	Key                 string `json:"key"`
	MessageCount        int    `json:"message_count"`
	TokenCount          int    `json:"token_count"`
	ActualStrategy      string `json:"actual_strategy"`
	LastFlush           string `json:"last_flush,omitempty"`
	AvgTokensPerMessage int    `json:"avg_tokens_per_message,omitempty"`
}

// Configuration types reused from task package
type FlushConfig struct {
	Strategy  string  `json:"strategy"`
	MaxKeys   int     `json:"max_keys"`
	DryRun    bool    `json:"dry_run"`
	Force     bool    `json:"force"`
	Threshold float64 `json:"threshold"`
}

type ClearConfig struct {
	Confirm bool `json:"confirm"`
	Backup  bool `json:"backup"`
}

type HealthConfig struct {
	IncludeStats      bool `json:"include_stats"`
	CheckConnectivity bool `json:"check_connectivity"`
}

type StatsConfig struct {
	IncludeContent bool   `json:"include_content"`
	GroupBy        string `json:"group_by"`
}

// Config holds configuration for the memory operations service
type Config struct {
	// ValidationLimits holds configurable validation limits
	ValidationLimits ValidationLimits
	// LockTTLs holds lock timeout configurations
	LockTTLs LockTTLs
}

// ValidationLimits holds configurable validation limits
// MaxMessageContentLength and MaxTotalContentSize can be configured via:
// - Environment variables: MAX_MESSAGE_CONTENT_LENGTH, MAX_TOTAL_CONTENT_SIZE
// - Command line flags: --max-message-content-length, --max-total-content-size
// - Project config YAML: max_message_content_length, max_total_content_size
type ValidationLimits struct {
	MaxMemoryRefLength      int
	MaxKeyLength            int
	MaxMessageContentLength int // Configurable via env vars/flags/config
	MaxMessagesPerRequest   int
	MaxTotalContentSize     int // Configurable via env vars/flags/config
}

// LockTTLs holds lock timeout configurations
type LockTTLs struct {
	Append time.Duration
	Clear  time.Duration
	Flush  time.Duration
}

// getIntFromEnv gets an integer value from environment variable with fallback to default
func getIntFromEnv(envKey string, defaultValue int) int {
	if envValue := os.Getenv(envKey); envValue != "" {
		if envInt, err := strconv.Atoi(envValue); err == nil && envInt > 0 {
			return envInt
		}
	}
	return defaultValue
}

// DefaultConfig returns default configuration with environment variable support
func DefaultConfig() *Config {
	return &Config{
		ValidationLimits: ValidationLimits{
			MaxMemoryRefLength:      100,
			MaxKeyLength:            255,
			MaxMessageContentLength: getIntFromEnv("MAX_MESSAGE_CONTENT_LENGTH", 10*1024), // 10KB default
			MaxMessagesPerRequest:   100,
			MaxTotalContentSize:     getIntFromEnv("MAX_TOTAL_CONTENT_SIZE", 100*1024), // 100KB default
		},
		LockTTLs: LockTTLs{
			Append: 30 * time.Second,
			Clear:  10 * time.Second,
			Flush:  5 * time.Minute,
		},
	}
}
