package gitsrc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/registry"
)

const archiveTempPattern = "compozy-gitsrc-archive-*"

type archiveLimits struct {
	maxCompressedSize   int64
	maxUncompressedSize int64
	maxFileCount        int
	tempDir             string
}

func createRepositoryArchive(
	ctx context.Context,
	checkoutDir string,
	limits archiveLimits,
) (_ io.ReadCloser, size int64, err error) {
	file, err := os.CreateTemp(limits.tempDir, archiveTempPattern)
	if err != nil {
		return nil, 0, fmt.Errorf("gitsrc: create repository archive: %w", err)
	}
	archive := &ownedArchiveFile{File: file, path: file.Name()}
	defer func() {
		if err != nil {
			err = errors.Join(err, archive.Close())
		}
	}()

	stats, err := fileutil.WriteTarGzipDirectory(
		ctx,
		file,
		checkoutDir,
		map[string]struct{}{`.git`: {}},
		fileutil.TarGzipLimits{
			MaxCompressedSize:   limits.maxCompressedSize,
			MaxUncompressedSize: limits.maxUncompressedSize,
			MaxFileCount:        limits.maxFileCount,
		},
	)
	if err != nil {
		return nil, 0, classifyArchiveError(err, limits)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("gitsrc: rewind repository archive: %w", err)
	}
	return archive, stats.CompressedSize, nil
}

func classifyArchiveError(err error, limits archiveLimits) error {
	switch {
	case errors.Is(err, fileutil.ErrTarGzipCompressedLimit):
		return fmt.Errorf(
			"gitsrc: repository archive: %w: limit=%d",
			registry.ErrArchiveTooLargeCompressed,
			limits.maxCompressedSize,
		)
	case errors.Is(err, fileutil.ErrTarGzipUncompressedLimit):
		return fmt.Errorf("%w: limit=%d", ErrRepositoryTooLarge, limits.maxUncompressedSize)
	case errors.Is(err, fileutil.ErrTarGzipFileCountLimit):
		return fmt.Errorf("%w: limit=%d", ErrRepositoryTooManyFiles, limits.maxFileCount)
	default:
		return err
	}
}

type ownedArchiveFile struct {
	*os.File
	path string

	closeOnce sync.Once
	closeErr  error
}

func (f *ownedArchiveFile) Close() error {
	if f == nil {
		return nil
	}
	f.closeOnce.Do(func() {
		closeErr := f.File.Close()
		removeErr := os.Remove(f.path)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			removeErr = fmt.Errorf("gitsrc: remove repository archive %q: %w", f.path, removeErr)
		}
		f.closeErr = errors.Join(closeErr, removeErr)
	})
	return f.closeErr
}
