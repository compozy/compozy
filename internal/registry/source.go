package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotSupported reports that a registry source does not implement an
// operation.
var ErrNotSupported = errors.New("registry: operation not supported")

// ErrPackageNotFound reports that no registry source resolved a package slug.
var ErrPackageNotFound = errors.New("registry: package not found")

// PackageNotFoundError carries the missing package slug.
type PackageNotFoundError struct {
	Slug string
}

func (e PackageNotFoundError) Error() string {
	slug := strings.TrimSpace(e.Slug)
	if slug == "" {
		return ErrPackageNotFound.Error()
	}
	return fmt.Sprintf("registry: package %q not found", slug)
}

func (e PackageNotFoundError) Is(target error) bool {
	return target == ErrPackageNotFound
}

// NewPackageNotFoundError returns the canonical package-not-found error.
func NewPackageNotFoundError(slug string) error {
	return PackageNotFoundError{Slug: slug}
}

// Source abstracts one registry backend.
type Source interface {
	Name() string
	Capabilities() SourceCaps
	Search(ctx context.Context, query string, opts SearchOpts) ([]Listing, error)
	Info(ctx context.Context, slug string) (*Detail, error)
	Download(ctx context.Context, slug string, opts DownloadOpts) (*DownloadResult, error)
	Close() error
}

// Downloader is the minimal download contract required by the installer.
type Downloader interface {
	Download(ctx context.Context, slug string, opts DownloadOpts) (*DownloadResult, error)
}
