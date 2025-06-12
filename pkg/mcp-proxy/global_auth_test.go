package mcpproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyHandlers_CombineAuthTokens(t *testing.T) {
	initLogger()

	t.Run("No global tokens, client tokens only", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: nil,
		}

		clientTokens := []string{"client-token-1", "client-token-2"}
		result := proxyHandlers.combineAuthTokens(clientTokens)

		assert.Equal(t, clientTokens, result)
	})

	t.Run("Global tokens only, no client tokens", func(t *testing.T) {
		globalTokens := []string{"global-token-1", "global-token-2"}
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: globalTokens,
		}

		result := proxyHandlers.combineAuthTokens(nil)

		assert.Equal(t, globalTokens, result)
	})

	t.Run("Both global and client tokens, no duplicates", func(t *testing.T) {
		globalTokens := []string{"global-token-1", "global-token-2"}
		clientTokens := []string{"client-token-1", "client-token-2"}
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: globalTokens,
		}

		result := proxyHandlers.combineAuthTokens(clientTokens)

		expected := []string{"global-token-1", "global-token-2", "client-token-1", "client-token-2"}
		assert.Equal(t, expected, result)
	})

	t.Run("Both global and client tokens with duplicates", func(t *testing.T) {
		globalTokens := []string{"shared-token", "global-token"}
		clientTokens := []string{"shared-token", "client-token"}
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: globalTokens,
		}

		result := proxyHandlers.combineAuthTokens(clientTokens)

		// Should include each token only once, with global tokens having priority
		expected := []string{"shared-token", "global-token", "client-token"}
		assert.Equal(t, expected, result)
	})

	t.Run("Empty string tokens are filtered out", func(t *testing.T) {
		globalTokens := []string{"global-token", "", "global-token-2"}
		clientTokens := []string{"", "client-token", ""}
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: globalTokens,
		}

		result := proxyHandlers.combineAuthTokens(clientTokens)

		expected := []string{"global-token", "global-token-2", "client-token"}
		assert.Equal(t, expected, result)
	})

	t.Run("All empty lists", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: []string{},
		}

		result := proxyHandlers.combineAuthTokens([]string{})

		assert.Empty(t, result)
	})

	t.Run("Both nil lists", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: nil,
		}

		result := proxyHandlers.combineAuthTokens(nil)

		assert.Nil(t, result)
	})
}

func TestGlobalAuthTokensIntegration(t *testing.T) {
	initLogger()

	t.Run("Server configuration with global auth tokens", func(t *testing.T) {
		globalTokens := []string{"global-token-1", "global-token-2"}

		config := &Config{
			BaseURL:          "http://localhost:8080",
			GlobalAuthTokens: globalTokens,
		}

		storage := NewMemoryStorage()
		clientManager := NewMockClientManager()

		// Create proxy handlers with global tokens
		proxyHandlers := NewProxyHandlers(storage, clientManager, config.BaseURL, config.GlobalAuthTokens)

		assert.Equal(t, globalTokens, proxyHandlers.globalAuthTokens)
	})

	t.Run("Global auth tokens are inherited by client middleware", func(t *testing.T) {
		globalTokens := []string{"global-auth-123"}
		clientTokens := []string{"client-auth-456"}

		storage := NewMemoryStorage()
		clientManager := NewMockClientManager()

		proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:8080", globalTokens)

		// Test the combination logic
		combined := proxyHandlers.combineAuthTokens(clientTokens)

		expected := []string{"global-auth-123", "client-auth-456"}
		assert.Equal(t, expected, combined)
	})

	t.Run("Global tokens work when client has no tokens", func(t *testing.T) {
		globalTokens := []string{"global-only-token"}

		storage := NewMemoryStorage()
		clientManager := NewMockClientManager()

		proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:8080", globalTokens)

		// Test with empty client tokens
		combined := proxyHandlers.combineAuthTokens([]string{})

		assert.Equal(t, globalTokens, combined)

		// Test with nil client tokens
		combined = proxyHandlers.combineAuthTokens(nil)

		assert.Equal(t, globalTokens, combined)
	})

	t.Run("No global tokens means only client tokens are used", func(t *testing.T) {
		clientTokens := []string{"client-only-token"}

		storage := NewMemoryStorage()
		clientManager := NewMockClientManager()

		// No global tokens
		proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:8080", nil)

		combined := proxyHandlers.combineAuthTokens(clientTokens)

		assert.Equal(t, clientTokens, combined)
	})
}
