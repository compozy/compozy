package filesnap

import (
	"fmt"
	"maps"
	"os"
	"time"
)

// Snapshot records the filesystem metadata used to detect staleness.
type Snapshot struct {
	ModTime time.Time
	Size    int64
}

// FromPath snapshots one filesystem path with os.Stat metadata.
func FromPath(path string) (Snapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("filesnap: stat %q: %w", path, err)
	}

	return Snapshot{
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, nil
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
		if leftSnapshot.Size != rightSnapshot.Size {
			return false
		}
		if !leftSnapshot.ModTime.Equal(rightSnapshot.ModTime) {
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
