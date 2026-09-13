package fileutil

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrTarGzipCompressedLimit reports that the compressed stream exceeded its byte budget.
	ErrTarGzipCompressedLimit = errors.New("fileutil: tar gzip compressed size limit exceeded")
	// ErrTarSizeLimit reports that the raw tar stream exceeded its byte budget.
	ErrTarSizeLimit = errors.New("fileutil: tar size limit exceeded")
	// ErrTarFileCountLimit reports that the archive exceeded its entry budget.
	ErrTarFileCountLimit = errors.New("fileutil: tar file count limit exceeded")
)

// TarGzipLimits bounds the archive while it is produced. Non-positive values disable a limit.
type TarGzipLimits struct {
	MaxCompressedSize   int64
	MaxUncompressedSize int64
	MaxFileCount        int
}

// TarGzipStats reports bytes and entries accepted before completion or failure.
type TarGzipStats struct {
	CompressedSize   int64
	UncompressedSize int64
	FileCount        int
}

type TarLimits struct {
	MaxBytes     int64
	MaxFileCount int
}

type TarStats struct {
	Bytes     int64
	FileCount int
}

// WriteTarGzipDirectory writes a deterministic gzip-compressed tar stream.
// Names in excludeTopLevel omit matching entries directly below root.
func WriteTarGzipDirectory(
	ctx context.Context,
	destination io.Writer,
	root string,
	excludeTopLevel map[string]struct{},
	limits TarGzipLimits,
) (stats TarGzipStats, err error) {
	if ctx == nil {
		return stats, errors.New("fileutil: tar gzip context is required")
	}
	if destination == nil {
		return stats, errors.New("fileutil: tar gzip destination is required")
	}

	compressed := newArchiveLimitWriter(destination, limits.MaxCompressedSize, ErrTarGzipCompressedLimit)
	gzipWriter := gzip.NewWriter(compressed)
	gzipWriter.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.OS = 255
	tarStats, tarErr := WriteTarDirectory(ctx, gzipWriter, root, excludeTopLevel, TarLimits{
		MaxBytes: limits.MaxUncompressedSize, MaxFileCount: limits.MaxFileCount,
	})
	gzipCloseErr := gzipWriter.Close()
	stats.CompressedSize = compressed.written
	stats.UncompressedSize, stats.FileCount = tarStats.Bytes, tarStats.FileCount
	return stats, errors.Join(tarErr, gzipCloseErr)
}

// WriteTarDirectory writes the canonical directory tar without a compression envelope.
func WriteTarDirectory(
	ctx context.Context,
	destination io.Writer,
	root string,
	excludeTopLevel map[string]struct{},
	limits TarLimits,
) (stats TarStats, err error) {
	if ctx == nil || destination == nil {
		return stats, errors.New("fileutil: tar context and destination are required")
	}
	bounded := newArchiveLimitWriter(destination, limits.MaxBytes, ErrTarSizeLimit)
	tarWriter := tar.NewWriter(bounded)

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("fileutil: resolve archive path %q: %w", path, err)
		}
		if relative == "." {
			return nil
		}
		topLevel, _, _ := strings.Cut(filepath.ToSlash(relative), "/")
		if _, excluded := excludeTopLevel[topLevel]; excluded {
			if entry.IsDir() && relative == topLevel {
				return filepath.SkipDir
			}
			return nil
		}
		if limits.MaxFileCount > 0 && stats.FileCount >= limits.MaxFileCount {
			return fmt.Errorf("%w: limit=%d", ErrTarFileCountLimit, limits.MaxFileCount)
		}
		stats.FileCount++
		return writeTarEntry(ctx, tarWriter, path, relative, entry)
	})
	tarCloseErr := tarWriter.Close()
	stats.Bytes = bounded.written
	if err := errors.Join(walkErr, tarCloseErr); err != nil {
		return stats, fmt.Errorf("fileutil: archive directory %q: %w", root, err)
	}
	return stats, nil
}

func writeTarEntry(
	ctx context.Context,
	writer *tar.Writer,
	path string,
	relative string,
	entry fs.DirEntry,
) (err error) {
	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("fileutil: inspect archive entry %q: %w", path, err)
	}
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("fileutil: read archive symlink %q: %w", path, err)
		}
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("fileutil: create archive header %q: %w", path, err)
	}
	header.Name = filepath.ToSlash(relative)
	header.ModTime = time.Unix(0, 0).UTC()
	header.AccessTime = time.Time{}
	header.ChangeTime = time.Time{}
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	if entry.IsDir() {
		header.Name += "/"
	}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("fileutil: write archive header %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	file, err := OpenRegularFile(path)
	if err != nil {
		return fmt.Errorf("fileutil: open archive entry %q: %w", path, err)
	}
	defer func() {
		err = errors.Join(err, closeArchiveFile(file, path))
	}()
	if _, err := io.Copy(writer, &contextReader{ctx: ctx, reader: file}); err != nil {
		return fmt.Errorf("fileutil: write archive entry %q: %w", path, err)
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

type archiveLimitWriter struct {
	destination io.Writer
	limit       int64
	written     int64
	limitErr    error
}

func newArchiveLimitWriter(destination io.Writer, limit int64, limitErr error) *archiveLimitWriter {
	return &archiveLimitWriter{destination: destination, limit: limit, limitErr: limitErr}
}

func (w *archiveLimitWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	accepted := data
	overflow := false
	if w.limit > 0 {
		remaining := w.limit - w.written
		if remaining <= 0 {
			return 0, w.limitErr
		}
		if int64(len(accepted)) > remaining {
			accepted = accepted[:remaining]
			overflow = true
		}
	}

	written, err := w.destination.Write(accepted)
	w.written += int64(written)
	if err != nil {
		return written, err
	}
	if written != len(accepted) {
		return written, io.ErrShortWrite
	}
	if overflow {
		return written, w.limitErr
	}
	return written, nil
}

func closeArchiveFile(file *os.File, path string) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("fileutil: close archive entry %q: %w", path, err)
	}
	return nil
}
