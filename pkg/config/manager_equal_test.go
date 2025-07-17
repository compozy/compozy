package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigEqual(t *testing.T) {
	t.Run("Should return true for identical configurations", func(t *testing.T) {
		config1 := &Config{
			Server: ServerConfig{
				Host:        "localhost",
				Port:        5001,
				CORSEnabled: true,
				Timeout:     30 * time.Second,
			},
			Database: DatabaseConfig{
				Host:   "db.example.com",
				Port:   "5432",
				User:   "testuser",
				DBName: "testdb",
			},
		}

		config2 := &Config{
			Server: ServerConfig{
				Host:        "localhost",
				Port:        5001,
				CORSEnabled: true,
				Timeout:     30 * time.Second,
			},
			Database: DatabaseConfig{
				Host:   "db.example.com",
				Port:   "5432",
				User:   "testuser",
				DBName: "testdb",
			},
		}

		assert.True(t, configEqual(config1, config2))
	})

	t.Run("Should return false for different configurations", func(t *testing.T) {
		config1 := &Config{
			Server: ServerConfig{
				Host: "localhost",
				Port: 5001,
			},
		}

		config2 := &Config{
			Server: ServerConfig{
				Host: "different.host.com",
				Port: 5001,
			},
		}

		assert.False(t, configEqual(config1, config2))
	})

	t.Run("Should handle nil configurations", func(t *testing.T) {
		config := &Config{}

		assert.True(t, configEqual(nil, nil))
		assert.False(t, configEqual(config, nil))
		assert.False(t, configEqual(nil, config))
	})

	t.Run("Should detect database configuration differences", func(t *testing.T) {
		config1 := &Config{
			Database: DatabaseConfig{
				Host: "db1.example.com",
				Port: "5432",
			},
		}

		config2 := &Config{
			Database: DatabaseConfig{
				Host: "db2.example.com",
				Port: "5432",
			},
		}

		assert.False(t, configEqual(config1, config2))
	})

	t.Run("Should detect OpenAI configuration differences", func(t *testing.T) {
		config1 := &Config{
			OpenAI: OpenAIConfig{
				APIKey:       "key1",
				DefaultModel: "gpt-4",
			},
		}

		config2 := &Config{
			OpenAI: OpenAIConfig{
				APIKey:       "key2",
				DefaultModel: "gpt-4",
			},
		}

		assert.False(t, configEqual(config1, config2))
	})
}
