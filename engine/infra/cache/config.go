package cache

import (
	"crypto/tls"
	"time"
)

type Config struct {
	URL      string `json:"url,omitempty"               yaml:"url,omitempty"               mapstructure:"url"`
	Host     string `json:"host,omitempty"              yaml:"host,omitempty"              mapstructure:"host"`
	Port     string `json:"port,omitempty"              yaml:"port,omitempty"              mapstructure:"port"`
	Password string `json:"password,omitempty"          yaml:"password,omitempty"          mapstructure:"password"`
	DB       int    `json:"db,omitempty"                yaml:"db,omitempty"                mapstructure:"db"`
	PoolSize int    `json:"pool_size,omitempty"         yaml:"pool_size,omitempty"         mapstructure:"pool_size"`
	// TLS Configuration
	TLSEnabled bool        `json:"tls_enabled,omitempty"       yaml:"tls_enabled,omitempty"       mapstructure:"tls_enabled"`
	TLSConfig  *tls.Config `json:"-"                           yaml:"-"                           mapstructure:"-"` // Not serializable
	// Timeout Configuration
	DialTimeout  time.Duration `json:"dial_timeout,omitempty"      yaml:"dial_timeout,omitempty"      mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout,omitempty"      yaml:"read_timeout,omitempty"      mapstructure:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout,omitempty"     yaml:"write_timeout,omitempty"     mapstructure:"write_timeout"`
	// Pool Configuration
	MaxRetries      int           `json:"max_retries,omitempty"       yaml:"max_retries,omitempty"       mapstructure:"max_retries"`
	MinRetryBackoff time.Duration `json:"min_retry_backoff,omitempty" yaml:"min_retry_backoff,omitempty" mapstructure:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `json:"max_retry_backoff,omitempty" yaml:"max_retry_backoff,omitempty" mapstructure:"max_retry_backoff"`
	// Health Check
	PoolTimeout time.Duration `json:"pool_timeout,omitempty"      yaml:"pool_timeout,omitempty"      mapstructure:"pool_timeout"`
}
