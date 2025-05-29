package ref

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// httpClient is a shared HTTP client with a timeout for loading remote documents.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// loadDocument loads a YAML document from a file or URL.
// The cwd parameter is used to resolve relative file paths and must be the directory of the current file.
func loadDocument(ctx context.Context, filePath, cwd string) (Document, error) {
	if cwd == "" {
		return nil, errors.New("current working directory cannot be empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var fullPath string
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return loadFromURL(ctx, filePath)
	}
	if filepath.IsAbs(filePath) {
		fullPath = filePath
	} else {
		fullPath = filepath.Clean(filepath.Join(cwd, filePath))
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat file %s", fullPath)
	}
	if info.Size() > MaxFileSize {
		return nil, errors.Errorf("file %s is too large (%d bytes)", fullPath, info.Size())
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %s", fullPath)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML in %s", fullPath)
	}
	return &simpleDocument{data: doc}, nil
}

// loadFromURL loads a YAML document from a URL.
func loadFromURL(ctx context.Context, urlStr string) (Document, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.Errorf("invalid URL %s", urlStr)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), http.NoBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", urlStr)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", urlStr)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("HTTP %d when fetching %s", resp.StatusCode, urlStr)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response from %s", urlStr)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML from %s", urlStr)
	}
	return &simpleDocument{data: doc}, nil
}
