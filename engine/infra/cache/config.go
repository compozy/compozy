package cache

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	URL      string `json:"url,omitempty"                      yaml:"url,omitempty"                      mapstructure:"url"`
	Host     string `json:"host,omitempty"                     yaml:"host,omitempty"                     mapstructure:"host"`
	Port     string `json:"port,omitempty"                     yaml:"port,omitempty"                     mapstructure:"port"`
	Password string `json:"password,omitempty"                 yaml:"password,omitempty"                 mapstructure:"password"`
	DB       int    `json:"db,omitempty"                       yaml:"db,omitempty"                       mapstructure:"db"`
	PoolSize int    `json:"pool_size,omitempty"                yaml:"pool_size,omitempty"                mapstructure:"pool_size"`
	// TLS Configuration
	TLSEnabled bool        `json:"tls_enabled,omitempty"              yaml:"tls_enabled,omitempty"              mapstructure:"tls_enabled"`
	TLSConfig  *tls.Config `json:"-"                                  yaml:"-"                                  mapstructure:"-"` // Not serializable
	// Timeout Configuration
	DialTimeout  time.Duration `json:"dial_timeout,omitempty"             yaml:"dial_timeout,omitempty"             mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout,omitempty"             yaml:"read_timeout,omitempty"             mapstructure:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout,omitempty"            yaml:"write_timeout,omitempty"            mapstructure:"write_timeout"`
	// Pool Configuration
	MaxRetries      int           `json:"max_retries,omitempty"              yaml:"max_retries,omitempty"              mapstructure:"max_retries"`
	MinRetryBackoff time.Duration `json:"min_retry_backoff,omitempty"        yaml:"min_retry_backoff,omitempty"        mapstructure:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `json:"max_retry_backoff,omitempty"        yaml:"max_retry_backoff,omitempty"        mapstructure:"max_retry_backoff"`
	MaxIdleConns    int           `json:"max_idle_conns,omitempty"           yaml:"max_idle_conns,omitempty"           mapstructure:"max_idle_conns"`
	MinIdleConns    int           `json:"min_idle_conns,omitempty"           yaml:"min_idle_conns,omitempty"           mapstructure:"min_idle_conns"`
	// Health Check
	PoolTimeout time.Duration `json:"pool_timeout,omitempty"             yaml:"pool_timeout,omitempty"             mapstructure:"pool_timeout"`
	PingTimeout time.Duration `json:"ping_timeout,omitempty"             yaml:"ping_timeout,omitempty"             mapstructure:"ping_timeout"`
	// Notification Configuration
	NotificationBufferSize int `json:"notification_buffer_size,omitempty" yaml:"notification_buffer_size,omitempty" mapstructure:"notification_buffer_size"`
}

// Validate checks if the cache configuration has valid values
func (c *Config) Validate() error {
	if err := c.validateBasicConfig(); err != nil {
		return err
	}
	if err := c.validatePoolConfig(); err != nil {
		return err
	}
	if err := c.validateTimeoutConfig(); err != nil {
		return err
	}
	if err := c.validateRetryConfig(); err != nil {
		return err
	}
	return nil
}

// validateBasicConfig validates basic configuration fields
func (c *Config) validateBasicConfig() error {
	if c.DB < 0 {
		return fmt.Errorf("cache DB index cannot be negative: got %d", c.DB)
	}
	if c.URL == "" && strings.TrimSpace(c.Host) == "" {
		return errors.New("cache host cannot be empty when URL is not provided")
	}
	if c.Port != "" {
		if port, err := strconv.Atoi(c.Port); err != nil {
			return fmt.Errorf("invalid cache port: %s", c.Port)
		} else if port <= 0 || port > 65535 {
			return fmt.Errorf("cache port must be between 1 and 65535: got %d", port)
		}
	}
	return nil
}

// validatePoolConfig validates pool-related configuration
func (c *Config) validatePoolConfig() error {
	if c.PoolSize < 0 {
		return fmt.Errorf("cache pool size cannot be negative: got %d", c.PoolSize)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("cache max retries cannot be negative: got %d", c.MaxRetries)
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("cache max idle connections cannot be negative: got %d", c.MaxIdleConns)
	}
	if c.MinIdleConns < 0 {
		return fmt.Errorf("cache min idle connections cannot be negative: got %d", c.MinIdleConns)
	}
	if c.MinIdleConns > c.MaxIdleConns && c.MaxIdleConns > 0 {
		return fmt.Errorf(
			"cache min idle connections (%d) cannot be greater than max idle connections (%d)",
			c.MinIdleConns,
			c.MaxIdleConns,
		)
	}
	if c.NotificationBufferSize < 0 {
		return fmt.Errorf("cache notification buffer size cannot be negative: got %d", c.NotificationBufferSize)
	}
	return nil
}

// validateTimeoutConfig validates timeout-related configuration
func (c *Config) validateTimeoutConfig() error {
	if c.DialTimeout < 0 {
		return fmt.Errorf("cache dial timeout cannot be negative: got %v", c.DialTimeout)
	}
	if c.ReadTimeout < 0 {
		return fmt.Errorf("cache read timeout cannot be negative: got %v", c.ReadTimeout)
	}
	if c.WriteTimeout < 0 {
		return fmt.Errorf("cache write timeout cannot be negative: got %v", c.WriteTimeout)
	}
	if c.PoolTimeout < 0 {
		return fmt.Errorf("cache pool timeout cannot be negative: got %v", c.PoolTimeout)
	}
	if c.PingTimeout < 0 {
		return fmt.Errorf("cache ping timeout cannot be negative: got %v", c.PingTimeout)
	}
	return nil
}

// validateRetryConfig validates retry backoff configuration
func (c *Config) validateRetryConfig() error {
	if c.MinRetryBackoff < 0 {
		return fmt.Errorf("cache min retry backoff cannot be negative: got %v", c.MinRetryBackoff)
	}
	if c.MaxRetryBackoff < 0 {
		return fmt.Errorf("cache max retry backoff cannot be negative: got %v", c.MaxRetryBackoff)
	}
	if c.MinRetryBackoff > c.MaxRetryBackoff && c.MaxRetryBackoff > 0 {
		return fmt.Errorf(
			"cache min retry backoff (%v) cannot be greater than max retry backoff (%v)",
			c.MinRetryBackoff,
			c.MaxRetryBackoff,
		)
	}
	return nil
}
