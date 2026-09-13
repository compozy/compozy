package pluginsource

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
)

const DefaultMaxCacheBytes int64 = 1 << 30

var (
	ErrPackageUnavailable = errors.New("marketplace package is unavailable")
	ErrCacheCapacity      = errors.New("marketplace package cache capacity exceeded")
)

// PackageCache owns immutable blobs; Root and MaxBytes must not change during operations.
type PackageCache struct {
	Root string
	// Zero selects the 1 GiB default; Sweep applies the aggregate budget at refresh commit.
	MaxBytes int64
	mu       sync.RWMutex
}

// Put verifies the stream before publishing one complete blob under its digest.
func (c *PackageCache) Put(ctx context.Context, digest string, source io.Reader) (err error) {
	if !validCacheDigest(digest) {
		return errors.New("marketplace package cache: lowercase SHA-256 digest is required")
	}
	limit, err := c.validate(ctx)
	if err != nil {
		return err
	}
	if source == nil {
		return errors.New("marketplace package cache: source is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := fileutil.OpenOrCreateDirectory(c.Root, 0o700)
	if err != nil {
		return fmt.Errorf("marketplace package cache: open root: %w", err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	name := ".pending-" + rand.Text()
	file, err := directory.CreateRegularFile(name, 0o600)
	if err != nil {
		return fmt.Errorf("marketplace package cache: create staging file: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, directory.RemoveBoundRegularFile(file, name))
		}
		err = errors.Join(err, file.Close())
	}()

	hash := sha256.New()
	reader := &cacheContextReader{ctx: ctx, source: source}
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, limit))
	if err != nil {
		return fmt.Errorf("marketplace package cache: write blob: %w", err)
	}
	if written == limit {
		var extra [1]byte
		n, readErr := io.ReadFull(reader, extra[:])
		if n > 0 {
			return ErrCacheCapacity
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("%w: supplied bytes do not match digest", ErrPackageUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("marketplace package cache: sync blob: %w", err)
	}
	committed, err = publishCacheBlob(ctx, directory, file, name, digest, limit)
	return err
}

func publishCacheBlob(ctx context.Context, directory *fileutil.Directory, file *os.File,
	staged, digest string, limit int64) (committed bool, err error) {
	name := digest + ".tar"
	committed, err = directory.MoveRegularFileNoReplace(file, staged, name)
	if !errors.Is(err, fileutil.ErrTargetExists) {
		return committed, err
	}
	existing, err := directory.OpenRegularFile(name)
	if err != nil {
		return false, fmt.Errorf("marketplace package cache: open existing blob: %w", err)
	}
	defer func() { err = errors.Join(err, existing.Close()) }()
	verifyErr := verifyCacheBlob(ctx, existing, digest, limit)
	if verifyErr == nil {
		return false, nil
	}
	if !errors.Is(verifyErr, ErrPackageUnavailable) {
		return false, verifyErr
	}
	if err := directory.RemoveBoundRegularFile(existing, name); err != nil {
		return false, err
	}
	return directory.MoveRegularFileNoReplace(file, staged, name)
}

// Open hashes the held file before transferring the immutable blob's reader to its caller.
func (c *PackageCache) Open(ctx context.Context, digest string) (io.ReadCloser, error) {
	if !validCacheDigest(digest) {
		return nil, errors.New("marketplace package cache: lowercase SHA-256 digest is required")
	}
	limit, err := c.validate(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory, err := fileutil.OpenDirectory(c.Root)
	if err != nil {
		return nil, unavailableCacheFile(err)
	}
	file, openErr := directory.OpenRegularFile(digest + ".tar")
	if err := errors.Join(openErr, directory.Close()); err != nil {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		return nil, unavailableCacheFile(err)
	}
	if err := verifyCacheBlob(ctx, file, digest, limit); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func unavailableCacheFile(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrPackageUnavailable, err)
	}
	return fmt.Errorf("marketplace package cache: open blob: %w", err)
}

func verifyCacheBlob(ctx context.Context, file *os.File, digest string, limit int64) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("marketplace package cache: stat blob: %w", err)
	}
	if info.Size() > limit {
		return fmt.Errorf("%w: blob exceeds cache capacity", ErrPackageUnavailable)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(&cacheContextReader{ctx: ctx, source: file}, limit)); err != nil {
		return fmt.Errorf("marketplace package cache: verify blob: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("%w: blob digest mismatch", ErrPackageUnavailable)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("marketplace package cache: rewind blob: %w", err)
	}
	return nil
}

func (c *PackageCache) validate(ctx context.Context) (int64, error) {
	if ctx == nil {
		return 0, errors.New("marketplace package cache: context is required")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if c == nil || strings.TrimSpace(c.Root) == "" {
		return 0, errors.New("marketplace package cache: root is required")
	}
	if c.MaxBytes < 0 {
		return 0, errors.New("marketplace package cache: capacity must be positive")
	}
	if c.MaxBytes == 0 {
		return DefaultMaxCacheBytes, nil
	}
	return c.MaxBytes, nil
}

func validCacheDigest(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(digest)
	return err == nil && hex.EncodeToString(decoded) == digest
}

type cacheContextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r *cacheContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(buffer)
}
