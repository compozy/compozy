package ref

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"fmt"

	"github.com/dgraph-io/ristretto"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// ristrettoConfig holds configuration values for the global Ristretto cache,
// populated from environment variables.
type ristrettoConfig struct {
	NumCounters int64 `envconfig:"NUM_COUNTERS" default:"10000000"`    // Default 10M
	MaxCost     int64 `envconfig:"MAX_COST"     default:"1073741824"` // Default 1GB (1 << 30)
	BufferItems int64 `envconfig:"BUFFER_ITEMS" default:"64"`
	Metrics     bool  `envconfig:"METRICS"      default:"true"`
}

// Global cache instance
var globalCache *ristretto.Cache

func init() {
	var rCfg ristrettoConfig
	// Use a prefix for environment variables, e.g., COMPOZY_REF_NUM_COUNTERS
	err := envconfig.Process("compozy_ref", &rCfg)
	if err != nil {
		// Log the error and use hardcoded defaults, as panicking might be too aggressive for a library.
		fmt.Fprintf(os.Stderr, "Warning: Error processing Ristretto cache config from env: %v. Using default values.\n", err)
		// Set defaults manually if envconfig fails (though 'default' struct tags should handle cases where vars aren't set)
		// This manual setting is more of a fallback if Process itself fails unexpectedly.
		rCfg.NumCounters = 10000000
		rCfg.MaxCost = 1 << 30
		rCfg.BufferItems = 64
		rCfg.Metrics = true
	}

	// Validate parsed/defaulted values to ensure they are sensible
	if rCfg.NumCounters <= 0 {
		fmt.Fprintf(os.Stderr, "Warning: Invalid Ristretto NumCounters (%d), using default 10M.\n", rCfg.NumCounters)
		rCfg.NumCounters = 10000000
	}
	if rCfg.MaxCost <= 0 {
		fmt.Fprintf(os.Stderr, "Warning: Invalid Ristretto MaxCost (%d), using default 1GB.\n", rCfg.MaxCost)
		rCfg.MaxCost = 1 << 30
	}
	if rCfg.BufferItems <= 0 {
		fmt.Fprintf(os.Stderr, "Warning: Invalid Ristretto BufferItems (%d), using default 64.\n", rCfg.BufferItems)
		rCfg.BufferItems = 64
	}

	globalCache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: rCfg.NumCounters,
		MaxCost:     rCfg.MaxCost,
		BufferItems: rCfg.BufferItems,
		Metrics:     rCfg.Metrics,
	})
	if err != nil {
		// Panic if cache creation itself fails, as this is a critical component.
		panic(fmt.Sprintf("Failed to create global Ristretto cache: %v", err))
	}
}

// Function to get the global cache
func GetGlobalCache() *ristretto.Cache {
	return globalCache
}

// Function to reset cache for testing (if needed by tests)
func ResetRistrettoCacheForTesting() {
	globalCache.Clear()
	// Or re-initialize if simpler and acceptable for tests
	// init()
}

// httpClient is a shared HTTP client with a timeout and redirect policy for loading remote documents.
// This can be overridden for testing by setting customHTTPClient.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= MaxURLRedirects {
			return errors.Errorf("too many redirects (%d)", len(via))
		}
		// Validate each redirect URL
		if err := validateURL(req.URL.String()); err != nil {
			return errors.Wrap(err, "invalid redirect URL")
		}
		return nil
	},
}

// customHTTPClient allows injection of a custom HTTP client for testing
var customHTTPClient *http.Client

// getHTTPClient returns the HTTP client to use for remote requests
func getHTTPClient() *http.Client {
	if customHTTPClient != nil {
		return customHTTPClient
	}
	return httpClient
}

// SetHTTPClientForTesting allows tests to inject a custom HTTP client
func SetHTTPClientForTesting(client *http.Client) {
	customHTTPClient = client
}

// ResetHTTPClientForTesting resets the HTTP client to the default
func ResetHTTPClientForTesting() {
	customHTTPClient = nil
}

// loadDocument loads a YAML document from a file or URL.
// The cwd parameter is used to resolve relative file paths and must be the directory of the current file.
func loadDocument(ctx context.Context, filePath, cwd string) (Document, error) {
	if cwd == "" {
		return nil, errors.New("current working directory cannot be empty")
	}
	if filePath == "" {
		return nil, errors.New("file path cannot be empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check if it's a URL
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		if err := validateURL(filePath); err != nil {
			return nil, errors.Wrapf(err, "invalid URL: %s", filePath)
		}
		return loadFromURL(ctx, filePath)
	}

	// Validate file path for security
	if err := validateFilePath(filePath, cwd); err != nil {
		return nil, err
	}

	var fullPath string
	if filepath.IsAbs(filePath) {
		fullPath = filePath
	} else {
		fullPath = filepath.Clean(filepath.Join(cwd, filePath))
	}

	// Check cache first
	if cached, ok := globalCache.Get(fullPath); ok {
		return &simpleDocument{data: cached}, nil
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat file %s", fullPath)
	}
	if info.Size() > MaxFileSize {
		return nil, errors.Errorf("file %s is too large (%d bytes, max: %d bytes)", fullPath, info.Size(), MaxFileSize)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Errorf("path %s is not a regular file", fullPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %s", fullPath)
	}

	// Validate data size
	if err := validateDataSize(data); err != nil {
		return nil, err
	}

	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML in %s", fullPath)
	}

	// Cache the parsed document data
	// Assuming cost is 1 for simplicity for now. TTL set to 1 hour.
	globalCache.SetWithTTL(fullPath, doc, 1, 1*time.Hour)

	return &simpleDocument{data: doc}, nil
}

// loadFromURL loads a YAML document from a URL.
func loadFromURL(ctx context.Context, urlStr string) (Document, error) {
	// Check cache first (URLs are also cached in the same LRU)
	if cached, ok := globalCache.Get(urlStr); ok {
		return &simpleDocument{data: cached}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", urlStr)
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Compozy-Ref-Resolver/1.0")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", urlStr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("HTTP %d when fetching %s", resp.StatusCode, urlStr)
	}

	// Limit response body size
	limitedReader := io.LimitReader(resp.Body, MaxResolvedDataSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response from %s", urlStr)
	}

	// Check if we hit the size limit
	if len(data) == MaxResolvedDataSize {
		return nil, errors.Errorf("response from %s exceeds maximum size of %d bytes", urlStr, MaxResolvedDataSize)
	}

	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML from %s", urlStr)
	}

	// Cache remote document
	// Assuming cost is 1 for simplicity for now. TTL set to 1 hour.
	globalCache.SetWithTTL(urlStr, doc, 1, 1*time.Hour)

	return &simpleDocument{data: doc}, nil
}
