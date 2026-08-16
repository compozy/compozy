package support

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func readTail(ctx context.Context, path string, maxBytes int64) (data []byte, truncated bool, err error) {
	if err := buildContextError(ctx); err != nil {
		return nil, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			closeErr = fmt.Errorf("support: close log tail file: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()
	if err := buildContextError(ctx); err != nil {
		return nil, false, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("support: stat log tail: %w", err)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		if _, err := file.Seek(-maxBytes, io.SeekEnd); err != nil {
			return nil, false, fmt.Errorf("support: seek log tail: %w", err)
		}
		truncated = true
	}
	if err := buildContextError(ctx); err != nil {
		return nil, false, err
	}
	data, err = readFileWithContext(ctx, file)
	if err != nil {
		return nil, false, fmt.Errorf("support: read log tail: %w", err)
	}
	return data, truncated, nil
}

func readFileWithContext(ctx context.Context, reader io.Reader) ([]byte, error) {
	var data []byte
	buffer := make([]byte, 32<<10)
	for {
		if err := buildContextError(ctx); err != nil {
			return nil, err
		}
		read, readErr := reader.Read(buffer)
		if read > 0 {
			data = append(data, buffer[:read]...)
		}
		if readErr == io.EOF {
			if err := buildContextError(ctx); err != nil {
				return nil, err
			}
			return data, nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}

type HomeTreeEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
	Mode string `json:"mode"`
}

func collectHomeTree(ctx context.Context, root string, limit int, excludedDirs ...string) ([]HomeTreeEntry, error) {
	if err := buildContextError(ctx); err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("support: home directory is required")
	}
	var entries []HomeTreeEntry
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if contextErr := buildContextError(ctx); contextErr != nil {
			return contextErr
		}
		if err != nil {
			return err
		}
		if len(entries) >= limit {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path != root {
			for _, excludedDir := range excludedDirs {
				if samePath(path, excludedDir) {
					return filepath.SkipDir
				}
			}
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		entries = append(entries, HomeTreeEntry{
			Path: filepath.ToSlash(rel),
			Kind: fileKind(info),
			Size: info.Size(),
			Mode: info.Mode().String(),
		})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("support: collect home tree: %w", walkErr)
	}
	sort.SliceStable(entries, func(i int, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func samePath(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func fileKind(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return "dir"
	case mode.IsRegular():
		return "file"
	case mode&os.ModeSymlink != 0:
		return "symlink"
	default:
		return "other"
	}
}
