package filesnap

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"time"
)

// Snapshot records the filesystem metadata used to detect staleness.
type Snapshot struct {
	ModTime  time.Time
	Size     int64
	mode     fs.FileMode
	change   changeToken
	digest   [sha256.Size]byte
	captured bool
	reusable bool
}

// FromPath captures current file metadata and a content fingerprint when OS change metadata is unavailable.
func FromPath(path string) (Snapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("filesnap: stat %q: %w", path, err)
	}

	return FromInfo(path, info), nil
}

// FromInfo captures supplied metadata, fingerprinting content only when the OS lacks a reliable change token.
func FromInfo(path string, info fs.FileInfo) Snapshot {
	snapshot := Snapshot{
		ModTime: info.ModTime(), Size: info.Size(), mode: info.Mode(),
		change: fileChangeToken(info), captured: true,
	}
	if snapshot.change.available {
		snapshot.reusable = true
		return snapshot
	}
	snapshot.digest, snapshot.reusable = fingerprintPath(path, info.Mode())
	return snapshot
}

// Equal requires matching captured changes or matching metadata for two synthetic snapshots.
func (s Snapshot) Equal(other Snapshot) bool {
	if s.Size != other.Size || !s.ModTime.Equal(other.ModTime) {
		return false
	}
	if !s.captured && !other.captured {
		return true
	}
	return s.captured && other.captured && s.reusable && other.reusable &&
		s.mode == other.mode && s.change == other.change && s.digest == other.digest
}

// fingerprintPath leaves unreadable or unsupported entries uncacheable without adding resolution errors.
func fingerprintPath(path string, mode fs.FileMode) ([sha256.Size]byte, bool) {
	switch {
	case mode.IsDir():
		entries, err := os.ReadDir(path)
		if err != nil {
			return [sha256.Size]byte{}, false
		}
		var content []byte
		for _, entry := range entries {
			content = append(content, entry.Name()...)
			content = append(content, 0)
			content = binary.LittleEndian.AppendUint32(content, uint32(entry.Type()))
		}
		return sha256.Sum256(content), true
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return [sha256.Size]byte{}, false
		}
		return sha256.Sum256([]byte(target)), true
	case mode.IsRegular():
		file, err := os.Open(path)
		if err != nil {
			return [sha256.Size]byte{}, false
		}
		digest := sha256.New()
		_, readErr := io.Copy(digest, file)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return [sha256.Size]byte{}, false
		}
		var result [sha256.Size]byte
		copy(result[:], digest.Sum(nil))
		return result, true
	default:
		return [sha256.Size]byte{}, false
	}
}

// Equal reports whether both snapshot maps contain the same keys and metadata.
func Equal(left, right map[string]Snapshot) bool {
	if len(left) != len(right) {
		return false
	}

	for path, leftSnapshot := range left {
		rightSnapshot, ok := right[path]
		if !ok {
			return false
		}
		if !leftSnapshot.Equal(rightSnapshot) {
			return false
		}
	}

	return true
}

// Clone returns an independent copy of the supplied snapshot map.
func Clone(src map[string]Snapshot) map[string]Snapshot {
	if len(src) == 0 {
		return map[string]Snapshot{}
	}

	cloned := make(map[string]Snapshot, len(src))
	maps.Copy(cloned, src)
	return cloned
}
