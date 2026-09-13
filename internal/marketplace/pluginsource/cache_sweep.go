package pluginsource

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

type cacheBlob struct {
	name     string
	size     int64
	modified time.Time
}

// Sweep preserves pinned blobs and evicts unreferenced blobs oldest-first to the configured budget.
func (c *PackageCache) Sweep(ctx context.Context, pinned map[string]struct{}) (err error) {
	limit, err := c.validate(ctx)
	if err != nil {
		return err
	}
	for digest := range pinned {
		if !validCacheDigest(digest) {
			return errors.New("marketplace package cache: invalid pinned digest")
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := fileutil.OpenDirectoryForMutation(c.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("marketplace package cache: open sweep root: %w", err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	blobs, total, err := cacheSweepCandidates(ctx, directory, pinned)
	if err != nil {
		return err
	}
	slices.SortFunc(blobs, func(left, right cacheBlob) int {
		if order := left.modified.Compare(right.modified); order != 0 {
			return order
		}
		return cmp.Compare(left.name, right.name)
	})
	for _, blob := range blobs {
		if total <= limit {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := removeCacheFile(directory, blob.name); err != nil {
			return err
		}
		total -= blob.size
	}
	if total > limit {
		return fmt.Errorf("%w: pinned bytes exceed the budget", ErrCacheCapacity)
	}
	return nil
}

func cacheSweepCandidates(ctx context.Context, directory *fileutil.Directory,
	pinned map[string]struct{}) ([]cacheBlob, int64, error) {
	names, err := directory.ReadDir()
	if err != nil {
		return nil, 0, err
	}
	var candidates []cacheBlob
	var total int64
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		if strings.HasPrefix(name, ".pending-") {
			if err := removeCacheFile(directory, name); err != nil {
				return nil, 0, err
			}
			continue
		}
		digest, ok := strings.CutSuffix(name, ".tar")
		if !ok || !validCacheDigest(digest) {
			continue
		}
		file, err := directory.OpenRegularFile(name)
		if err != nil {
			return nil, 0, err
		}
		info, statErr := file.Stat()
		if err := errors.Join(statErr, file.Close()); err != nil {
			return nil, 0, err
		}
		const maxInt64 = int64(^uint64(0) >> 1)
		if info.Size() > maxInt64-total {
			return nil, 0, ErrCacheCapacity
		}
		total += info.Size()
		if _, keep := pinned[digest]; !keep {
			candidates = append(candidates, cacheBlob{name: name, size: info.Size(), modified: info.ModTime()})
		}
	}
	return candidates, total, nil
}

func removeCacheFile(directory *fileutil.Directory, name string) (err error) {
	file, err := directory.OpenRegularFile(name)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return directory.RemoveBoundRegularFile(file, name)
}
