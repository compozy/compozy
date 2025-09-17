package mcpproxy

import (
	"context"
	"fmt"
	"time"
)

// DefaultConfig returns a default configuration for the MCP proxy server
func DefaultConfig() *Config {
	return &Config{
		Port:            "6001",
		Host:            "127.0.0.1", // Bind to localhost by default for security
		ShutdownTimeout: 10 * time.Second,
	}
}

// New creates a new MCP proxy server with default configuration and in-memory storage
func New(ctx context.Context) (*Server, error) {
	return NewWithConfig(ctx, DefaultConfig())
}

// NewWithConfig creates a new MCP proxy server with custom configuration and in-memory storage
func NewWithConfig(ctx context.Context, config *Config) (*Server, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	storage := NewMemoryStorage()
	clientManager := NewMCPClientManager(ctx, storage, nil)
	return NewServer(config, storage, clientManager), nil
}

// NewWithRedis creates a new MCP proxy server with Redis storage
func NewWithRedis(ctx context.Context, config *Config, redisConfig *RedisConfig) (*Server, error) {
	storage, err := NewRedisStorage(ctx, redisConfig)
	if err != nil {
		return nil, err
	}
	clientManager := NewMCPClientManager(ctx, storage, nil)
	return NewServer(config, storage, clientManager), nil
}

// Run starts the MCP proxy server and blocks until shutdown with in-memory storage
func Run(ctx context.Context, config *Config) error {
	server, err := NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	return server.Start(ctx)
}
