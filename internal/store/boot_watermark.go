package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/version"
)

const (
	bootWatermarkFormat = 1
	// bootWatermarkSettleWindow keeps a file modified this recently out of a watermark: a later
	// write inside the filesystem's timestamp granularity would be indistinguishable from it.
	bootWatermarkSettleWindow = 2 * time.Second
	// bootWatermarkHeadBytes covers the SQLite database header (change counter, page count,
	// schema cookie, version-valid-for) and the whole write-ahead log header (checkpoint
	// sequence and salts), which SQLite rewrites whenever it commits to or restarts the file.
	bootWatermarkHeadBytes = 100
)

// FileStamp is the cheap evidence a boot watermark compares instead of reopening a file. Size and
// modification time alone do not prove unchanged content, so a stamp also carries the file's
// platform identity and, for a database family member, a digest of its leading bytes.
//
// What the evidence proves: the same inode on the same device was neither written, truncated,
// renamed, nor re-timestamped since the stamp (each of those moves the inode-change time, which
// no caller can set back), and an atomic replacement is a different inode. What stays unproven:
// a filesystem that does not maintain inode-change times, a raw write below the filesystem, or a
// rewrite landing in the very clock tick the stamp was taken in; the header digest still catches
// those whenever the replacing content committed through SQLite at a different change counter,
// page count, schema cookie, or log salt, but not a same-header change confined to data pages.
type FileStamp struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime_ns"`
	// Identity is the device, inode, and inode-change time. It is empty on a platform that
	// exposes none, and a stamp without it never proves anything.
	Identity string `json:"identity,omitempty"`
	// Head digests the leading bytes of a database family member.
	Head string `json:"head,omitempty"`
}

func newFileStamp(info os.FileInfo) FileStamp {
	identity, _ := fileStampIdentity(info)
	return FileStamp{
		Name: info.Name(), Size: info.Size(), ModTime: info.ModTime().UnixNano(), Identity: identity,
	}
}

// proves reports whether other is the same file with the same content evidence. A stamp whose
// file identity could not be established proves nothing, so its session takes the full checks.
func (s FileStamp) proves(other FileStamp) bool {
	return s.Identity != "" && s == other
}

// fileHeadDigest digests the first bootWatermarkHeadBytes of a file.
func fileHeadDigest(path string) (digest string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	head := make([]byte, bootWatermarkHeadBytes)
	read, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	sum := sha256.Sum256(head[:read])
	return hex.EncodeToString(sum[:]), nil
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
	return newFileStamp(info), true, nil
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
			return SessionFilesStamp{}, false, fmt.Errorf(
				"store: session database member %q is not a regular file",
				name,
			)
		}
		member := newFileStamp(info)
		member.Head, err = fileHeadDigest(filepath.Join(sessionDir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return SessionFilesStamp{}, false, fmt.Errorf("store: read %q for boot watermark: %w", name, err)
		}
		stamp.Database = append(stamp.Database, member)
	}
	return stamp, true, nil
}

// Equal reports whether both stamps carry the same evidence for every session file. It is false
// whenever any file's identity could not be established, which sends the session down the full
// path instead of trusting size and modification time.
func (s SessionFilesStamp) Equal(other SessionFilesStamp) bool {
	if !s.Meta.proves(other.Meta) || len(s.Database) != len(other.Database) {
		return false
	}
	for index := range s.Database {
		if !s.Database[index].proves(other.Database[index]) {
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

// BootWatermarkBinary identifies the running build. Proofs recorded by another build are never
// honored, because the checks that build ran at boot may differ from this one's. The identity is
// resolved once per process.
func BootWatermarkBinary() (string, error) {
	return bootWatermarkBinary()
}

var bootWatermarkBinary = sync.OnceValues(func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("store: resolve executable for boot watermark: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("store: stat executable for boot watermark: %w", err)
	}
	build := version.Current()
	return BootWatermarkBuildIdentity(build.Version, build.Commit, info.Size(), info.ModTime().UnixNano(),
		func() (string, error) { return executableDigest(path) })
})

// BootWatermarkBuildIdentity names one build. Executable size and modification time can collide
// across builds, so they are only ever combined with a build-specific identifier: the stamped
// release version and commit, or, for a build that carries no commit stamp, a digest of the
// executable's content. A build that can supply neither has no identity.
func BootWatermarkBuildIdentity(
	release string,
	commit string,
	size int64,
	modTime int64,
	contentDigest func() (string, error),
) (string, error) {
	commit = strings.TrimSpace(commit)
	if commit != "" && commit != "unknown" {
		return fmt.Sprintf("build:%s:%s:%d:%d", strings.TrimSpace(release), commit, size, modTime), nil
	}
	digest, err := contentDigest()
	if err != nil {
		return "", fmt.Errorf("store: digest executable for boot watermark: %w", err)
	}
	if strings.TrimSpace(digest) == "" {
		return "", errors.New("store: executable digest for boot watermark is empty")
	}
	return fmt.Sprintf("sha256:%s:%d:%d", digest, size, modTime), nil
}

func executableDigest(path string) (digest string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
