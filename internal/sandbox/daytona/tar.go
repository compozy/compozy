package daytona

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

var errUnsafeTarPath = errors.New("sandbox/daytona: unsafe tar path")

type tarStats struct {
	Files int
	Bytes int64
}

type archiveEntry struct {
	path string
	rel  string
	info fs.FileInfo
	link string
}

func writeTar(
	ctx context.Context,
	root string,
	dst io.Writer,
	excludePatterns []string,
) (stats tarStats, err error) {
	root = filepath.Clean(root)
	entries, err := collectArchiveEntries(ctx, root, excludePatterns)
	if err != nil {
		return tarStats{}, err
	}
	writer := tar.NewWriter(dst)
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("sandbox/daytona: close tar writer: %w", closeErr))
		}
	}()
	for _, entry := range entries {
		header, err := tar.FileInfoHeader(entry.info, entry.link)
		if err != nil {
			return tarStats{}, fmt.Errorf("sandbox/daytona: build tar header for %q: %w", entry.path, err)
		}
		header.Name = entry.rel
		if err := writer.WriteHeader(header); err != nil {
			return tarStats{}, fmt.Errorf("sandbox/daytona: write tar header for %q: %w", entry.rel, err)
		}
		if entry.info.Mode().IsRegular() {
			written, err := copyArchiveFile(writer, entry)
			if err != nil {
				return tarStats{}, err
			}
			stats.Files++
			stats.Bytes += written
		}
	}
	return stats, nil
}

func buildTarArchive(ctx context.Context, root string, excludePatterns []string) (*os.File, tarStats, error) {
	file, err := os.CreateTemp("", "compozy-daytona-sync-*.tar")
	if err != nil {
		return nil, tarStats{}, fmt.Errorf("sandbox/daytona: create tar archive temp file: %w", err)
	}
	stats, writeErr := writeTar(ctx, root, file, excludePatterns)
	if writeErr != nil {
		closeErr := file.Close()
		removeErr := os.Remove(file.Name())
		return nil, tarStats{}, errors.Join(writeErr, closeErr, removeErr)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		closeErr := file.Close()
		removeErr := os.Remove(file.Name())
		return nil, tarStats{}, errors.Join(
			fmt.Errorf("sandbox/daytona: rewind tar archive temp file: %w", err),
			closeErr,
			removeErr,
		)
	}
	return file, stats, nil
}

func collectArchiveEntries(ctx context.Context, root string, excludePatterns []string) ([]archiveEntry, error) {
	var entries []archiveEntry
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if filePath == root {
			return nil
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return fmt.Errorf("sandbox/daytona: make tar relative path: %w", err)
		}
		rel = filepath.ToSlash(rel)
		if shouldExcludeArchivePath(rel, excludePatterns) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("sandbox/daytona: stat %q for tar: %w", filePath, err)
		}
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("sandbox/daytona: read symlink %q: %w", filePath, err)
			}
		}
		entries = append(entries, archiveEntry{path: filePath, rel: rel, info: info, link: link})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func copyArchiveFile(writer io.Writer, entry archiveEntry) (int64, error) {
	file, err := os.Open(entry.path)
	if err != nil {
		return 0, fmt.Errorf("sandbox/daytona: open %q for tar: %w", entry.path, err)
	}
	written, copyErr := io.Copy(writer, file)
	closeErr := file.Close()
	if copyErr != nil {
		return 0, fmt.Errorf("sandbox/daytona: write tar file %q: %w", entry.rel, copyErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("sandbox/daytona: close tar source %q: %w", entry.path, closeErr)
	}
	return written, nil
}

func safeArchiveName(name string) (string, error) {
	cleaned := path.Clean(strings.TrimSpace(name))
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("%w: empty path", errUnsafeTarPath)
	}
	if path.IsAbs(cleaned) {
		return "", fmt.Errorf("%w: absolute path %q", errUnsafeTarPath, name)
	}
	if slices.Contains(strings.Split(cleaned, "/"), "..") {
		return "", fmt.Errorf("%w: traversal path %q", errUnsafeTarPath, name)
	}
	return cleaned, nil
}

func isArchiveRootMarker(name string) bool {
	cleaned := path.Clean(strings.TrimSpace(name))
	return cleaned == "." || cleaned == ""
}

func shouldExcludeArchivePath(rel string, excludePatterns []string) bool {
	for part := range strings.SplitSeq(rel, "/") {
		switch part {
		case "node_modules", "dist", "build", "target", ".cache", ".next", ".turbo":
			return true
		}
	}
	for _, pattern := range excludePatterns {
		if archivePatternMatches(pattern, rel) {
			return true
		}
	}
	return false
}

func archivePatternMatches(pattern string, rel string) bool {
	pattern = strings.TrimSpace(filepath.ToSlash(pattern))
	if pattern == "" {
		return false
	}
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	if rel == "" {
		return false
	}
	trimmed := strings.TrimSuffix(pattern, "/")
	if trimmed != "" && (rel == trimmed || strings.HasPrefix(rel, trimmed+"/")) {
		return true
	}
	if matched, err := path.Match(pattern, rel); err == nil && matched {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if matched, err := path.Match(pattern, path.Base(rel)); err == nil && matched {
			return true
		}
	}
	return false
}

func modePerm(mode fs.FileMode, fallback fs.FileMode) fs.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return fallback
	}
	return perm
}
