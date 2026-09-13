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
	archive, err := capturePackage(ctx, checkout, relative, capacity, "")
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, archive.Close()) }()
	if err := cache.Put(ctx, archive.digest, archive.file); err != nil {
		return "", err
	}
	return archive.digest, nil
}

// capturedPackage owns private staging bytes until verification permits cache publication.
type capturedPackage struct {
	file   *os.File
	digest string
}

func (p *capturedPackage) Close() error {
	return errors.Join(p.file.Close(), os.Remove(p.file.Name()))
}

func capturePackage(
	ctx context.Context,
	checkout, relative string,
	capacity int64,
	tempDir string,
) (_ *capturedPackage, err error) {
	packagePath, err := confinedPackagePath(checkout, relative)
	if err != nil {
		return nil, err
	}
	directory, err := fileutil.OpenDirectory(packagePath)
	if err != nil {
		if errors.Is(err, fileutil.ErrSymlink) {
			return nil, fmt.Errorf("%w: %w", ErrSourceOutsideCheckout, err)
		}
		return nil, fmt.Errorf("%w: open package directory: %w", ErrSourceUnreachable, err)
	}
	if err := directory.Close(); err != nil {
		return nil, err
	}
	archive, err := os.CreateTemp(tempDir, "compozy-marketplace-package-*.tar")
	if err != nil {
		return nil, fmt.Errorf("pluginsource: create package spool: %w", err)
	}
	packageBytes := &capturedPackage{file: archive}
	defer func() {
		if err != nil {
			err = errors.Join(err, packageBytes.Close())
		}
	}()
	hash := sha256.New()
	_, err = fileutil.WriteTarDirectory(ctx, io.MultiWriter(archive, hash), packagePath,
		map[string]struct{}{".git": {}}, fileutil.TarLimits{
			MaxBytes: min(capacity, registry.DefaultMaxArchiveSize), MaxFileCount: registry.DefaultMaxFileCount,
		})
	if err != nil {
		return nil, err
	}
	packageBytes.digest = hex.EncodeToString(hash.Sum(nil))
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("pluginsource: rewind package spool: %w", err)
	}
	return packageBytes, nil
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
