package pluginsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/registry"
)

var ErrSourceOutsideCheckout = errors.New("source_outside_checkout")

// CapturePackage commits a confined package's canonical bytes before its checkout can be released.
func CapturePackage(ctx context.Context, checkout, relative string, cache *PackageCache) (digest string, err error) {
	capacity, err := cache.validate(ctx)
	if err != nil {
		return "", err
	}
	packagePath, err := confinedPackagePath(checkout, relative)
	if err != nil {
		return "", err
	}
	directory, err := fileutil.OpenDirectory(packagePath)
	if err != nil {
		if errors.Is(err, fileutil.ErrSymlink) {
			return "", fmt.Errorf("%w: %w", ErrSourceOutsideCheckout, err)
		}
		return "", fmt.Errorf("%w: open package directory: %w", ErrSourceUnreachable, err)
	}
	if err := directory.Close(); err != nil {
		return "", err
	}
	archive, err := os.CreateTemp("", "compozy-marketplace-package-*.tar")
	if err != nil {
		return "", fmt.Errorf("pluginsource: create package spool: %w", err)
	}
	defer func() { err = errors.Join(err, archive.Close(), os.Remove(archive.Name())) }()
	hash := sha256.New()
	_, err = fileutil.WriteTarDirectory(ctx, io.MultiWriter(archive, hash), packagePath,
		map[string]struct{}{".git": {}}, fileutil.TarLimits{
			MaxBytes: min(capacity, registry.DefaultMaxArchiveSize), MaxFileCount: registry.DefaultMaxFileCount,
		})
	if err != nil {
		return "", err
	}
	digest = hex.EncodeToString(hash.Sum(nil))
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("pluginsource: rewind package spool: %w", err)
	}
	if err := cache.Put(ctx, digest, archive); err != nil {
		return "", err
	}
	return digest, nil
}

func confinedPackagePath(checkout, relative string) (string, error) {
	if strings.TrimSpace(checkout) == "" || strings.ContainsAny(relative, "\\\x00") {
		return "", ErrSourceOutsideCheckout
	}
	if relative == "" {
		relative = "."
	}
	path := filepath.Clean(filepath.FromSlash(relative))
	if !filepath.IsLocal(path) {
		return "", ErrSourceOutsideCheckout
	}
	return filepath.Join(checkout, path), nil
}
