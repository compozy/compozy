package fileutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TarGzipDirectory returns a deterministic gzip-compressed tar stream.
// Names in excludeTopLevel omit matching entries directly below root.
func TarGzipDirectory(root string, excludeTopLevel map[string]struct{}) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	gzipWriter.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("fileutil: resolve archive path %q: %w", path, err)
		}
		if relative == "." {
			return nil
		}
		topLevel := strings.Split(filepath.ToSlash(relative), "/")[0]
		if _, excluded := excludeTopLevel[topLevel]; excluded {
			if entry.IsDir() && relative == topLevel {
				return filepath.SkipDir
			}
			return nil
		}
		return writeTarGzipEntry(tarWriter, path, relative, entry)
	})
	tarCloseErr := tarWriter.Close()
	gzipCloseErr := gzipWriter.Close()
	if err := errors.Join(walkErr, tarCloseErr, gzipCloseErr); err != nil {
		return nil, fmt.Errorf("fileutil: archive directory %q: %w", root, err)
	}
	return buffer.Bytes(), nil
}

func writeTarGzipEntry(writer *tar.Writer, path string, relative string, entry fs.DirEntry) (err error) {
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

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("fileutil: open archive entry %q: %w", path, err)
	}
	defer func() {
		err = errors.Join(err, closeTarGzipFile(file, path))
	}()
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("fileutil: write archive entry %q: %w", path, err)
	}
	return nil
}

func closeTarGzipFile(file *os.File, path string) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("fileutil: close archive entry %q: %w", path, err)
	}
	return nil
}
