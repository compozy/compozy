package marketplace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DirectorySource reads the extension catalog from an absolute file URL.
type DirectorySource struct {
	path             string
	maxResponseBytes int64
}

var _ Source = (*DirectorySource)(nil)

// NewDirectorySource creates a bounded local-checkout catalog source.
func NewDirectorySource(baseURL string) (*DirectorySource, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != protocolFile || parsed.Host != "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("marketplace catalog: base URL must be an absolute file URL")
	}
	directory := filepath.FromSlash(parsed.Path)
	if !filepath.IsAbs(directory) {
		return nil, errors.New("marketplace catalog: base URL must be an absolute file URL")
	}
	return &DirectorySource{
		path:             filepath.Join(directory, "v3", "extensions.json"),
		maxResponseBytes: defaultMaxResponseBytes,
	}, nil
}

// refreshOnAccess keeps checkout reads independent from durable HTTP cache freshness.
func (s *DirectorySource) refreshOnAccess() bool {
	return s != nil
}

// Fetch reads and validates the local document without mutating projection state.
func (s *DirectorySource) Fetch(ctx context.Context) (document *Document, err error) {
	if ctx == nil {
		return nil, errors.New("marketplace catalog: fetch context is required")
	}
	if s == nil || strings.TrimSpace(s.path) == "" || s.maxResponseBytes <= 0 {
		return nil, errors.New("marketplace catalog: directory source is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("marketplace catalog: read extension feed canceled: %w", err)
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: open extension feed: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("marketplace catalog: close extension feed: %w", closeErr))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: stat extension feed: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("marketplace catalog: extension feed must be a regular file")
	}
	if info.Size() > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(file, s.maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: read extension feed: %w", err)
	}
	if int64(len(body)) > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("marketplace catalog: read extension feed canceled: %w", err)
	}
	document, err = DecodeDocument(KindExtension, body)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: validate extension feed: %w", err)
	}
	return document, nil
}
