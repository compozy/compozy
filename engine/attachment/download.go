package attachment

import (
	"context"
	"fmt"
	"os"
)

// DownloadURLToTemp streams the remote resource identified by urlStr into a temporary
// file while honoring configured download policies. Callers must invoke Cleanup on the
// returned Resolved handle when finished.
func DownloadURLToTemp(ctx context.Context, urlStr string, maxSize int64) (Resolved, int64, error) {
	path, mime, err := httpDownloadToTemp(ctx, urlStr, maxSize)
	if err != nil {
		return nil, 0, err
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		_ = os.Remove(path)
		return nil, 0, fmt.Errorf("stat download %q: %w", path, statErr)
	}
	return &resolvedFile{path: path, mime: mime, temp: true}, info.Size(), nil
}
