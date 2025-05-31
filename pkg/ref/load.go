package ref

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// fileCache provides a simple LRU-style cache for loaded documents
type fileCache struct {
	mu      sync.RWMutex
	cache   map[string]Document
	access  map[string]int64
	maxSize int
}

var globalFileCache = &fileCache{
	cache:   make(map[string]Document),
	access:  make(map[string]int64),
	maxSize: 128,
}

func (fc *fileCache) get(key string) (Document, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	doc, exists := fc.cache[key]
	if exists {
		fc.access[key] = time.Now().UnixNano()
	}
	return doc, exists
}

func (fc *fileCache) set(key string, doc Document) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Simple eviction if we're at capacity
	if len(fc.cache) >= fc.maxSize {
		// Find oldest accessed item
		var oldestKey string
		var oldestTime = time.Now().UnixNano()
		for k, accessTime := range fc.access {
			if accessTime < oldestTime {
				oldestTime = accessTime
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(fc.cache, oldestKey)
			delete(fc.access, oldestKey)
		}
	}

	fc.cache[key] = doc
	fc.access[key] = time.Now().UnixNano()
}

// httpClient is a shared HTTP client with a timeout and redirect policy for loading remote documents.
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
	if doc, exists := globalFileCache.get(fullPath); exists {
		return doc, nil
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

	result := &simpleDocument{data: doc}
	// Cache the result
	globalFileCache.set(fullPath, result)

	return result, nil
}

// loadFromURL loads a YAML document from a URL.
func loadFromURL(ctx context.Context, urlStr string) (Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", urlStr)
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Compozy-Ref-Resolver/1.0")

	resp, err := httpClient.Do(req)
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
	return &simpleDocument{data: doc}, nil
}
