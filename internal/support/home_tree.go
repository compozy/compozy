package support

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func readTail(path string, maxBytes int64) (data []byte, truncated bool, err error) {
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
	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("support: stat log tail: %w", err)
	}
	if maxBytes <= 0 || info.Size() <= maxBytes {
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, false, fmt.Errorf("support: read log tail: %w", err)
		}
		return data, false, nil
	}
	if _, err := file.Seek(-maxBytes, io.SeekEnd); err != nil {
		return nil, false, fmt.Errorf("support: seek log tail: %w", err)
	}
	data, err = io.ReadAll(file)
	if err != nil {
		return nil, false, fmt.Errorf("support: read bounded log tail: %w", err)
	}
	return data, true, nil
}

type HomeTreeEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
	Mode string `json:"mode"`
}

func collectHomeTree(root string, supportDir string, limit int) ([]HomeTreeEntry, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("support: home directory is required")
	}
	var entries []HomeTreeEntry
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if len(entries) >= limit {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if samePath(path, supportDir) && path != root {
			return filepath.SkipDir
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
