package pluginsource

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
)

type DirectorySource struct {
	ref  string
	path string
}

func NewDirectorySource(ref string) (*DirectorySource, error) {
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(normalized, "file:") {
		return nil, ErrInvalidRef
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, err
	}
	return &DirectorySource{ref: normalized, path: filepath.FromSlash(parsed.Path)}, nil
}

func (s *DirectorySource) Fetch(ctx context.Context) (Document, error) {
	document, err := ReadDirectory(ctx, s.path)
	if err != nil {
		return Document{}, err
	}
	document.SourceRef = s.ref
	document.ResolvedRef = s.ref + "@" + document.DigestSHA256
	return document, nil
}

func (s *DirectorySource) OpenSnapshot(ctx context.Context, document Document) (*Snapshot, error) {
	if document.SourceRef != s.ref {
		return nil, errors.New("pluginsource: document origin does not match its source")
	}
	captured, err := s.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	if captured.DigestSHA256 != document.DigestSHA256 || captured.Path != document.Path {
		return nil, errors.New("pluginsource: folder marketplace document changed before package capture")
	}
	return &Snapshot{Root: s.path, ResolvedRef: captured.ResolvedRef, release: func() error { return nil }}, nil
}
