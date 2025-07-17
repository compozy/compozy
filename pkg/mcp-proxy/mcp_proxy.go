package mcpproxy

import (
	"context"
	"fmt"
	"time"
)

// DefaultConfig returns a default configuration for the MCP proxy server
func DefaultConfig() *Config {
	return &Config{
		Port:             "8081",
		Host:             "127.0.0.1", // Bind to localhost by default for security
		ShutdownTimeout:  10 * time.Second,
		AdminTokens:      []string{"CHANGE_ME_ADMIN_TOKEN"}, // Require admin token by default
		AdminAllowIPs:    []string{"127.0.0.1", "::1"},      // Restrict to localhost by default
		GlobalAuthTokens: []string{},                        // No global auth tokens by default
	}
}

// New creates a new MCP proxy server with default configuration and in-memory storage
func New() (*Server, error) {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new MCP proxy server with custom configuration and in-memory storage
func NewWithConfig(config *Config) (*Server, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	storage := NewMemoryStorage()
	clientManager := NewMCPClientManager(storage, nil)
	return NewServer(config, storage, clientManager), nil
}

// NewWithRedis creates a new MCP proxy server with Redis storage
func NewWithRedis(config *Config, redisConfig *RedisConfig) (*Server, error) {
	storage, err := NewRedisStorage(redisConfig)
	if err != nil {
		return nil, err
	}
	clientManager := NewMCPClientManager(storage, nil)
	return NewServer(config, storage, clientManager), nil
}

// Run starts the MCP proxy server and blocks until shutdown with in-memory storage
func Run(ctx context.Context, config *Config) error {
	server, err := NewWithConfig(config)
	if err != nil {
		return err
	}
	return server.Start(ctx)
}
