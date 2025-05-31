package ref

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Mock RoundTripper for Testing Remote HTTP Loading
// -----------------------------------------------------------------------------

// HandlerFunc matches http.RoundTripper but is easier to compose
type HandlerFunc func(req *http.Request) (*http.Response, error)

// MockRoundTripper implements http.RoundTripper for testing
type MockRoundTripper struct {
	handlers map[string]HandlerFunc
	callLog  []string // Track which URLs were called
}

// NewMockRT creates a new MockRoundTripper
func NewMockRT() *MockRoundTripper {
	return &MockRoundTripper{
		handlers: make(map[string]HandlerFunc),
		callLog:  make([]string, 0),
	}
}

// RoundTrip implements the http.RoundTripper interface
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	m.callLog = append(m.callLog, url)

	h, ok := m.handlers[url]
	if !ok {
		return nil, fmt.Errorf("unexpected request to %s", url)
	}
	return h(req)
}

// Stub adds a handler that returns the given status and body
func (m *MockRoundTripper) Stub(url string, status int, body string, hdr http.Header) {
	if hdr == nil {
		hdr = http.Header{}
	}
	m.handlers[url] = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     hdr,
		}, nil
	}
}

// Redirect adds a handler that returns a redirect to the destination URL
func (m *MockRoundTripper) Redirect(src, dst string) {
	h := http.Header{}
	h.Set("Location", dst)
	m.Stub(src, http.StatusFound, "", h)
}

// CallCount returns the number of times a URL was called
func (m *MockRoundTripper) CallCount(url string) int {
	count := 0
	for _, called := range m.callLog {
		if called == url {
			count++
		}
	}
	return count
}

// -----------------------------------------------------------------------------
// Remote Document Loading Tests
// -----------------------------------------------------------------------------

func TestLoadFromURL_HappyPath(t *testing.T) {
	const simpleYAML = "key: value\narray:\n  - item1\n  - item2\n"

	t.Run("Should load and parse YAML from URL", func(t *testing.T) {
		rt := NewMockRT()
		rt.Stub("https://cfg.local/core.yaml", http.StatusOK, simpleYAML, nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		doc, err := loadFromURL(ctx, "https://cfg.local/core.yaml")
		require.NoError(t, err)

		// Verify the document was parsed correctly
		value, err := doc.Get("key")
		require.NoError(t, err)
		assert.Equal(t, "value", value)

		arrayValue, err := doc.Get("array.0")
		require.NoError(t, err)
		assert.Equal(t, "item1", arrayValue)

		// Verify it was cached
		assert.Equal(t, 1, rt.CallCount("https://cfg.local/core.yaml"))
	})
}

func TestLoadFromURL_Caching(t *testing.T) {
	const yamlContent = "cached: true\n"

	t.Run("Should cache remote documents with TTL", func(t *testing.T) {
		// Reset cache to ensure clean state
		ResetRistrettoCacheForTesting()

		rt := NewMockRT()
		rt.Stub("https://cfg.local/cache-test.yaml", http.StatusOK, yamlContent, nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()

		// First call should hit the network
		doc1, err := loadFromURL(ctx, "https://cfg.local/cache-test.yaml")
		require.NoError(t, err)
		value1, err := doc1.Get("cached")
		require.NoError(t, err)
		assert.Equal(t, true, value1)

		// Second call should hit cache
		doc2, err := loadFromURL(ctx, "https://cfg.local/cache-test.yaml")
		require.NoError(t, err)
		value2, err := doc2.Get("cached")
		require.NoError(t, err)
		assert.Equal(t, true, value2)

		// Verify only one network call was made
		assert.Equal(t, 1, rt.CallCount("https://cfg.local/cache-test.yaml"))
	})
}

func TestLoadFromURL_RedirectLimits(t *testing.T) {
	t.Run("Should follow redirects up to MaxURLRedirects", func(t *testing.T) {
		rt := NewMockRT()

		// Create a redirect chain just under the limit
		for i := 1; i < MaxURLRedirects; i++ {
			src := fmt.Sprintf("https://cfg.local/redirect%d", i)
			dst := fmt.Sprintf("https://cfg.local/redirect%d", i+1)
			rt.Redirect(src, dst)
		}

		// Final destination returns content
		finalURL := fmt.Sprintf("https://cfg.local/redirect%d", MaxURLRedirects)
		rt.Stub(finalURL, http.StatusOK, "final: content\n", nil)

		SetHTTPClientForTesting(&http.Client{
			Transport:     rt,
			CheckRedirect: httpClient.CheckRedirect, // Use the same redirect policy
		})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		doc, err := loadFromURL(ctx, "https://cfg.local/redirect1")
		require.NoError(t, err)

		value, err := doc.Get("final")
		require.NoError(t, err)
		assert.Equal(t, "content", value)
	})

	t.Run("Should reject redirect chains exceeding MaxURLRedirects", func(t *testing.T) {
		rt := NewMockRT()

		// Create a redirect chain that exceeds the limit
		for i := 1; i <= MaxURLRedirects+2; i++ {
			src := fmt.Sprintf("https://cfg.local/loop%d", i)
			dst := fmt.Sprintf("https://cfg.local/loop%d", i+1)
			rt.Redirect(src, dst)
		}

		SetHTTPClientForTesting(&http.Client{
			Transport:     rt,
			CheckRedirect: httpClient.CheckRedirect, // Use the same redirect policy
		})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		_, err := loadFromURL(ctx, "https://cfg.local/loop1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too many redirects")
	})
}

func TestLoadFromURL_SizeLimits(t *testing.T) {
	t.Run("Should reject oversized responses", func(t *testing.T) {
		rt := NewMockRT()

		// Create a response larger than MaxResolvedDataSize
		oversized := strings.Repeat("A", MaxResolvedDataSize+1)
		rt.Stub("https://cfg.local/big.yaml", http.StatusOK, oversized, nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		_, err := loadFromURL(ctx, "https://cfg.local/big.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum size")
	})

	t.Run("Should accept responses at size limit", func(t *testing.T) {
		rt := NewMockRT()

		// Create a response just under the limit
		baseContent := "key: value\n"
		remainingBytes := MaxResolvedDataSize - len(baseContent) - 1 // Leave 1 byte under limit
		commentPattern := "# comment\n"
		numComments := remainingBytes / len(commentPattern)
		atLimit := baseContent + strings.Repeat(commentPattern, numComments)

		rt.Stub("https://cfg.local/at-limit.yaml", http.StatusOK, atLimit, nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		doc, err := loadFromURL(ctx, "https://cfg.local/at-limit.yaml")
		require.NoError(t, err)

		value, err := doc.Get("key")
		require.NoError(t, err)
		assert.Equal(t, "value", value)
	})
}

func TestLoadFromURL_ErrorHandling(t *testing.T) {
	t.Run("Should handle HTTP errors", func(t *testing.T) {
		rt := NewMockRT()
		rt.Stub("https://cfg.local/not-found.yaml", http.StatusNotFound, "Not Found", nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		_, err := loadFromURL(ctx, "https://cfg.local/not-found.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 404")
	})

	t.Run("Should handle invalid YAML", func(t *testing.T) {
		rt := NewMockRT()
		rt.Stub("https://cfg.local/invalid.yaml", http.StatusOK, "invalid: yaml: content: [", nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		_, err := loadFromURL(ctx, "https://cfg.local/invalid.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse YAML")
	})

	t.Run("Should handle network errors", func(t *testing.T) {
		rt := NewMockRT()
		rt.handlers["https://cfg.local/network-error.yaml"] = func(_ *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network connection failed")
		}

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ctx := context.Background()
		_, err := loadFromURL(ctx, "https://cfg.local/network-error.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch")
		assert.Contains(t, err.Error(), "network connection failed")
	})
}

func TestLoadFromURL_ContextTimeout(t *testing.T) {
	t.Run("Should respect context timeout", func(t *testing.T) {
		rt := NewMockRT()
		rt.handlers["https://cfg.local/slow.yaml"] = func(req *http.Request) (*http.Response, error) {
			// Check if the context is already canceled
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			default:
			}

			// Simulate a slow response that should be canceled
			time.Sleep(100 * time.Millisecond)

			// Check again after the sleep
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			default:
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("slow: response\n")),
			}, nil
		}

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		// Create a context that times out quickly
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := loadFromURL(ctx, "https://cfg.local/slow.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

// -----------------------------------------------------------------------------
// Integration Tests with Reference Resolution
// -----------------------------------------------------------------------------

func TestRemoteRefResolution(t *testing.T) {
	const remoteYAML = `
schemas:
  - id: remote_schema
    type: object
    properties:
      name:
        type: string
config:
  database:
    host: remote.example.com
    port: 5432
`

	t.Run("Should resolve references to remote documents", func(t *testing.T) {
		rt := NewMockRT()
		rt.Stub("https://cfg.example.com/remote.yaml", http.StatusOK, remoteYAML, nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ref := &Ref{
			Type: TypeFile,
			File: "https://cfg.example.com/remote.yaml",
			Path: "schemas.#(id==\"remote_schema\")",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		result, err := ref.Resolve(ctx, map[string]any{}, "/dummy/path", "/dummy/root")
		require.NoError(t, err)

		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "remote_schema", schema["id"])
		assert.Equal(t, "object", schema["type"])

		// Verify caching - should not make second HTTP call
		_, err = ref.Resolve(ctx, map[string]any{}, "/dummy/path", "/dummy/root")
		require.NoError(t, err)
		assert.Equal(t, 1, rt.CallCount("https://cfg.example.com/remote.yaml"))
	})

	t.Run("Should resolve nested references in remote documents", func(t *testing.T) {
		const nestedRemoteYAML = `
base_config:
  $ref: https://cfg.example.com/base.yaml::database
extra: value
`
		const baseYAML = `
database:
  host: nested.example.com
  port: 3306
  ssl: true
`

		rt := NewMockRT()
		rt.Stub("https://cfg.example.com/nested.yaml", http.StatusOK, nestedRemoteYAML, nil)
		rt.Stub("https://cfg.example.com/base.yaml", http.StatusOK, baseYAML, nil)

		SetHTTPClientForTesting(&http.Client{Transport: rt})
		defer ResetHTTPClientForTesting()

		ref := &Ref{
			Type: TypeFile,
			File: "https://cfg.example.com/nested.yaml",
			Path: "",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		result, err := ref.Resolve(ctx, map[string]any{}, "/dummy/path", "/dummy/root")
		require.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "value", resultMap["extra"])

		baseConfig, ok := resultMap["base_config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "nested.example.com", baseConfig["host"])
		assert.Equal(t, float64(3306), baseConfig["port"])
		assert.Equal(t, true, baseConfig["ssl"])

		// Verify both URLs were called
		assert.Equal(t, 1, rt.CallCount("https://cfg.example.com/nested.yaml"))
		assert.Equal(t, 1, rt.CallCount("https://cfg.example.com/base.yaml"))
	})
}
