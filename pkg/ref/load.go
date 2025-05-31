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

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Default cache sizes
const (
	DefaultDocCacheSize  = 256
	DefaultPathCacheSize = 512
)

// Cache configuration
type CacheConfig struct {
	DocCacheSize    int
	PathCacheSize   int
	EnablePathCache bool
}

// Global cache instances
var (
	resolvedDocs     *lru.Cache[string, any]
	resolvedDocsOnce sync.Once

	pathCache     *lru.Cache[string, string] // path -> compiled gjson path
	pathCacheOnce sync.Once

	cacheConfig               *CacheConfig
	configOnce                sync.Once
	configSetProgrammatically bool // Flag to prevent env var override
)

// getCacheConfig returns the cache configuration, reading from environment variables if available
func getCacheConfig() *CacheConfig {
	// If config is already set programmatically, return it
	if cacheConfig != nil && configSetProgrammatically {
		return cacheConfig
	}

	configOnce.Do(func() {
		// Don't override programmatically set config
		if cacheConfig != nil && configSetProgrammatically {
			return
		}

		docSize := DefaultDocCacheSize
		pathSize := DefaultPathCacheSize
		enablePathCache := true

		// Read from environment variables
		if envDocSize := os.Getenv("COMPOZY_REF_CACHE_SIZE"); envDocSize != "" {
			if size, err := strconv.Atoi(envDocSize); err == nil && size > 0 {
				docSize = size
			}
		}

		if envPathSize := os.Getenv("COMPOZY_REF_PATH_CACHE_SIZE"); envPathSize != "" {
			if size, err := strconv.Atoi(envPathSize); err == nil && size > 0 {
				pathSize = size
			}
		}

		if envDisablePathCache := os.Getenv("COMPOZY_REF_DISABLE_PATH_CACHE"); envDisablePathCache != "" {
			enablePathCache = envDisablePathCache != "true" && envDisablePathCache != "1"
		}

		cacheConfig = &CacheConfig{
			DocCacheSize:    docSize,
			PathCacheSize:   pathSize,
			EnablePathCache: enablePathCache,
		}
	})
	return cacheConfig
}

// SetCacheConfig allows programmatic configuration of cache settings.
// This must be called before any cache operations or it will have no effect.
func SetCacheConfig(config *CacheConfig) {
	// Reset the once guards first to allow reconfiguration
	resolvedDocsOnce = sync.Once{}
	pathCacheOnce = sync.Once{}
	configOnce = sync.Once{}

	// Clear existing caches
	resolvedDocs = nil
	pathCache = nil
	cacheConfig = nil
	configSetProgrammatically = false

	// Set the new config
	if config == nil {
		cacheConfig = &CacheConfig{
			DocCacheSize:    DefaultDocCacheSize,
			PathCacheSize:   DefaultPathCacheSize,
			EnablePathCache: true,
		}
		configSetProgrammatically = false // Use env vars for nil config
	} else {
		cacheConfig = &CacheConfig{
			DocCacheSize:    config.DocCacheSize,
			PathCacheSize:   config.PathCacheSize,
			EnablePathCache: config.EnablePathCache,
		}
		if cacheConfig.DocCacheSize <= 0 {
			cacheConfig.DocCacheSize = DefaultDocCacheSize
		}
		if cacheConfig.PathCacheSize <= 0 {
			cacheConfig.PathCacheSize = DefaultPathCacheSize
		}
		configSetProgrammatically = true // Mark as programmatic
	}
}

// ResetCachesForTesting resets all caches and configuration for testing purposes
func ResetCachesForTesting() {
	resolvedDocsOnce = sync.Once{}
	pathCacheOnce = sync.Once{}
	configOnce = sync.Once{}
	resolvedDocs = nil
	pathCache = nil
	cacheConfig = nil
	configSetProgrammatically = false
}

// getResolvedDocsCache returns the global LRU cache, initializing it if necessary
func getResolvedDocsCache() *lru.Cache[string, any] {
	resolvedDocsOnce.Do(func() {
		config := getCacheConfig()
		var err error
		resolvedDocs, err = lru.New[string, any](config.DocCacheSize)
		if err != nil {
			panic("failed to create LRU cache: " + err.Error())
		}
	})
	return resolvedDocs
}

// getPathCache returns the global path cache, initializing it if necessary
func getPathCache() *lru.Cache[string, string] {
	if !getCacheConfig().EnablePathCache {
		return nil
	}

	pathCacheOnce.Do(func() {
		config := getCacheConfig()
		var err error
		pathCache, err = lru.New[string, string](config.PathCacheSize)
		if err != nil {
			// Path cache is optional, don't panic
			pathCache = nil
		}
	})
	return pathCache
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
	if cached, ok := getResolvedDocsCache().Get(fullPath); ok {
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
	getResolvedDocsCache().Add(fullPath, doc)

	return &simpleDocument{data: doc}, nil
}

// loadFromURL loads a YAML document from a URL.
func loadFromURL(ctx context.Context, urlStr string) (Document, error) {
	// Check cache first (URLs are also cached in the same LRU)
	if cached, ok := getResolvedDocsCache().Get(urlStr); ok {
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
	getResolvedDocsCache().Add(urlStr, doc)

	return &simpleDocument{data: doc}, nil
}
