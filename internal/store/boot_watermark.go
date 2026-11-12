package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	bootWatermarkFormat = 1
	// bootWatermarkSettleWindow keeps a file modified this recently out of a watermark: a later
	// write inside the filesystem's timestamp granularity would be indistinguishable from it.
	bootWatermarkSettleWindow = 2 * time.Second
)

// FileStamp is the cheap identity a boot watermark compares instead of reopening a file.
type FileStamp struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime_ns"`
}

// StatFileStamp stamps one regular file. A missing file reports false without an error.
func StatFileStamp(path string) (FileStamp, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return FileStamp{}, false, nil
	}
	if err != nil {
		return FileStamp{}, false, fmt.Errorf("store: stat %q for boot watermark: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return FileStamp{}, false, fmt.Errorf("store: boot watermark target %q is not a regular file", path)
	}
	return FileStamp{Name: info.Name(), Size: info.Size(), ModTime: info.ModTime().UnixNano()}, true, nil
}

// Settled reports whether the file was last modified long enough before now for its stamp to
// distinguish any later write.
func (s FileStamp) Settled(now time.Time) bool {
	return now.Sub(time.Unix(0, s.ModTime)) >= bootWatermarkSettleWindow
}

// SessionFilesStamp identifies the durable files one session's boot checks read: its metadata
// document and every member of its events database family. The shared-memory index is transient
// and an empty write-ahead log holds no frames, so neither takes part.
type SessionFilesStamp struct {
	Meta     FileStamp   `json:"meta"`
	Database []FileStamp `json:"database"`
}

// StatSessionFiles stamps a session directory. It reports false when the directory has no
// readable metadata document, which always means "inspect this session in full".
func StatSessionFiles(sessionDir string) (SessionFilesStamp, bool, error) {
	meta, found, err := StatFileStamp(SessionMetaFile(sessionDir))
	if err != nil || !found {
		return SessionFilesStamp{}, false, err
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return SessionFilesStamp{}, false, fmt.Errorf("store: list %q for boot watermark: %w", sessionDir, err)
	}
	stamp := SessionFilesStamp{Meta: meta, Database: make([]FileStamp, 0, 2)}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, SessionDatabaseName) || name == SessionDatabaseName+"-shm" {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return SessionFilesStamp{}, false, fmt.Errorf("store: stat %q for boot watermark: %w", name, err)
		}
		if name == SessionDatabaseName+"-wal" && info.Size() == 0 {
			continue
		}
		if !info.Mode().IsRegular() {
			return SessionFilesStamp{}, false, fmt.Errorf("store: session database member %q is not a regular file", name)
		}
		stamp.Database = append(stamp.Database, FileStamp{
			Name: name, Size: info.Size(), ModTime: info.ModTime().UnixNano(),
		})
	}
	return stamp, true, nil
}

// Equal reports whether both stamps describe byte-identical session files.
func (s SessionFilesStamp) Equal(other SessionFilesStamp) bool {
	if s.Meta != other.Meta || len(s.Database) != len(other.Database) {
		return false
	}
	for index := range s.Database {
		if s.Database[index] != other.Database[index] {
			return false
		}
	}
	return true
}

// Settled reports whether every stamped file is old enough to be recorded in a watermark.
func (s SessionFilesStamp) Settled(now time.Time) bool {
	if !s.Meta.Settled(now) {
		return false
	}
	for _, member := range s.Database {
		if !member.Settled(now) {
			return false
		}
	}
	return true
}

type bootWatermarkDocument[T any] struct {
	Format  int          `json:"format"`
	Profile string       `json:"profile"`
	Entries map[string]T `json:"entries"`
}

// ReadBootWatermarks loads a boot watermark index. The index is a disposable proof cache: a
// missing, unreadable, foreign-format, or foreign-profile document yields no entries, which makes
// every session take its full boot path.
func ReadBootWatermarks[T any](path string, profile string) map[string]T {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]T{}
	}
	var document bootWatermarkDocument[T]
	if err := json.Unmarshal(data, &document); err != nil {
		return map[string]T{}
	}
	if document.Format != bootWatermarkFormat || document.Profile != profile || document.Entries == nil {
		return map[string]T{}
	}
	return document.Entries
}

// WriteBootWatermarks atomically replaces a boot watermark index.
func WriteBootWatermarks[T any](path string, profile string, entries map[string]T) error {
	payload, err := json.Marshal(bootWatermarkDocument[T]{
		Format: bootWatermarkFormat, Profile: profile, Entries: entries,
	})
	if err != nil {
		return fmt.Errorf("store: encode boot watermarks %q: %w", path, err)
	}
	if err := fileutil.AtomicWriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("store: write boot watermarks %q: %w", path, err)
	}
	return nil
}

// MigrationStreamHead identifies the embedded head a binary verifies databases against. Two
// binaries with the same head accept exactly the same recorded migration history.
func MigrationStreamHead(stream MigrationStream) (string, error) {
	directory, err := loadMigrationDirectory(stream)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d:%d:%s",
		stream.Name, directory.maxVersion, directory.migrationCount, directory.sumDigest), nil
}

// BootWatermarkBinary identifies the running executable. Proofs recorded by another build are
// never honored, because the checks that build ran at boot may differ from this one's.
func BootWatermarkBinary() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("store: resolve executable for boot watermark: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("store: stat executable for boot watermark: %w", err)
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano()), nil
}
