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

// DirectorySource reads one per-kind document from an absolute file URL.
type DirectorySource struct {
	kind             Kind
	path             string
	maxResponseBytes int64
}

var _ Source = (*DirectorySource)(nil)

// NewDirectorySource creates a bounded local-checkout catalog source.
func NewDirectorySource(kind Kind, baseURL string) (*DirectorySource, error) {
	filename, err := kindFilename(kind)
	if err != nil {
		return nil, err
	}
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
		kind:             kind,
		path:             filepath.Join(directory, filename),
		maxResponseBytes: defaultMaxResponseBytes,
	}, nil
}

func (s *DirectorySource) Kind() Kind {
	if s == nil {
		return ""
	}
	return s.kind
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
		return nil, fmt.Errorf("marketplace catalog: read %q feed canceled: %w", s.kind, err)
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: open %q feed: %w", s.kind, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("marketplace catalog: close %q feed: %w", s.kind, closeErr))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: stat %q feed: %w", s.kind, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("marketplace catalog: %q feed must be a regular file", s.kind)
	}
	if info.Size() > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(file, s.maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: read %q feed: %w", s.kind, err)
	}
	if int64(len(body)) > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("marketplace catalog: read %q feed canceled: %w", s.kind, err)
	}
	document, err = DecodeDocument(s.kind, body)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: validate %q feed: %w", s.kind, err)
	}
	return document, nil
}
