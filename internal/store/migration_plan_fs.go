package store

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

type migrationPlanFS struct {
	files map[string][]byte
}

func newMigrationPlanFS() *migrationPlanFS {
	return &migrationPlanFS{files: make(map[string][]byte)}
}

func (m *migrationPlanFS) addFile(name string, contents []byte) error {
	if !fs.ValidPath(name) || name == "." {
		return fmt.Errorf("invalid migration plan path %q", name)
	}
	if _, exists := m.files[name]; exists {
		return fmt.Errorf("duplicate migration plan path %q", name)
	}
	m.files[name] = append([]byte(nil), contents...)
	return nil
}

func (m *migrationPlanFS) Open(name string) (fs.File, error) {
	contents, exists := m.files[name]
	if !exists {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	info := migrationPlanFileInfo{name: path.Base(name), size: int64(len(contents))}
	return &migrationPlanOpenFile{Reader: bytes.NewReader(contents), info: info}, nil
}

func (m *migrationPlanFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	entries := make(map[string]migrationPlanFileInfo)
	for filePath, contents := range m.files {
		if !strings.HasPrefix(filePath, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(filePath, prefix)
		parts := strings.SplitN(remainder, "/", 2)
		info := migrationPlanFileInfo{name: parts[0]}
		if len(parts) == 1 {
			info.size = int64(len(contents))
		} else {
			info.directory = true
		}
		entries[info.name] = info
	}
	if name != "." && len(entries) == 0 {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	result := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result, nil
}

type migrationPlanOpenFile struct {
	*bytes.Reader
	info migrationPlanFileInfo
}

func (f *migrationPlanOpenFile) Close() error {
	return nil
}

func (f *migrationPlanOpenFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

type migrationPlanFileInfo struct {
	name      string
	size      int64
	directory bool
}

func (i migrationPlanFileInfo) Name() string {
	return i.name
}

func (i migrationPlanFileInfo) Size() int64 {
	return i.size
}

func (i migrationPlanFileInfo) Mode() fs.FileMode {
	if i.directory {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

func (i migrationPlanFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (i migrationPlanFileInfo) IsDir() bool {
	return i.directory
}

func (i migrationPlanFileInfo) Sys() any {
	return nil
}

func (i migrationPlanFileInfo) Type() fs.FileMode {
	return i.Mode().Type()
}

func (i migrationPlanFileInfo) Info() (fs.FileInfo, error) {
	return i, nil
}
