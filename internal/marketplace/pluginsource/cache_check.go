package pluginsource

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/filesnap"
)

const maxVerifiedCacheBlobs = 1024

// Check verifies availability, reusing a digest only while held-file change metadata matches.
// Open independently hashes every blob before returning installation bytes.
func (c *PackageCache) Check(ctx context.Context, digest string) (err error) {
	if !validCacheDigest(digest) {
		return errors.New("marketplace package cache: lowercase SHA-256 digest is required")
	}
	limit, err := c.validate(ctx)
	if err != nil {
		return err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := openCacheBlob(c.Root, digest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	before, err := filesnap.FromFile(file)
	if err != nil {
		return err
	}
	if before.Size > limit {
		return fmt.Errorf("%w: blob exceeds cache capacity", ErrPackageUnavailable)
	}
	if c.verifiedSnapshot(digest, before) {
		return ctx.Err()
	}
	if err := verifyCacheBlob(ctx, file, digest, limit); err != nil {
		return err
	}
	after, err := filesnap.FromFile(file)
	if err != nil {
		return err
	}
	// Missing OS change tokens deliberately prevent reuse; those platforms rehash each check.
	if before.Equal(after) {
		c.rememberVerifiedSnapshot(digest, after)
	}
	return nil
}

func (c *PackageCache) verifiedSnapshot(digest string, current filesnap.Snapshot) bool {
	c.verifiedMu.Lock()
	defer c.verifiedMu.Unlock()
	previous, exists := c.verified[digest]
	return exists && previous.Equal(current)
}

func (c *PackageCache) rememberVerifiedSnapshot(digest string, snapshot filesnap.Snapshot) {
	c.verifiedMu.Lock()
	defer c.verifiedMu.Unlock()
	if c.verified == nil {
		c.verified = make(map[string]filesnap.Snapshot)
	}
	if len(c.verified) >= maxVerifiedCacheBlobs {
		clear(c.verified)
	}
	c.verified[digest] = snapshot
}

func (c *PackageCache) clearVerifiedSnapshots() {
	c.verifiedMu.Lock()
	defer c.verifiedMu.Unlock()
	clear(c.verified)
}
