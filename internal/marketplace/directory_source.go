package marketplace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// DirectorySource reads the extension catalog from an absolute file URL.
type DirectorySource struct {
	path             string
	maxResponseBytes int64
}

var _ FeedSource = (*DirectorySource)(nil)

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

// Fetch reads and validates the local document without mutating projection state.
func (s *DirectorySource) Fetch(ctx context.Context) (*Document, error) {
	if s == nil {
		return nil, errors.New("marketplace catalog: directory source is required")
	}
	body, err := s.read(ctx, s.path)
	if body == nil {
		return nil, err
	}
	document, decodeErr := DecodeDocument(body)
	return document, errors.Join(err, decodeErr)
}

// FetchPresets reads only the v3 preset catalog from the configured directory.
func (s *DirectorySource) FetchPresets(ctx context.Context) (*PresetDocument, error) {
	if s == nil {
		return nil, errors.New("marketplace catalog: directory source is required")
	}
	body, err := s.read(ctx, filepath.Join(filepath.Dir(s.path), "marketplaces.json"))
	if body == nil {
		return nil, err
	}
	document, decodeErr := DecodePresets(body)
	return document, errors.Join(err, decodeErr)
}

func (s *DirectorySource) read(ctx context.Context, path string) (_ []byte, err error) {
	if ctx == nil {
		return nil, errors.New("marketplace catalog: fetch context is required")
	}
	if s == nil || strings.TrimSpace(s.path) == "" || s.maxResponseBytes <= 0 {
		return nil, errors.New("marketplace catalog: directory source is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("marketplace catalog: read feed canceled: %w", err)
	}
	file, err := fileutil.OpenRegularFile(path)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: open feed: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("marketplace catalog: close feed: %w", closeErr))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: stat feed: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("marketplace catalog: feed must be a regular file")
	}
	if info.Size() > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(file, s.maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: read feed: %w", err)
	}
	if int64(len(body)) > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("marketplace catalog: read feed canceled: %w", err)
	}
	return body, nil
}
