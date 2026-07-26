package storeseed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
)

type Seed struct {
	dir  string
	path string
}

// NewGlobal creates a closed seed containing the current global schema.
func NewGlobal(ctx context.Context) (*Seed, error) {
	return newSeed(ctx, initializeGlobal)
}

// NewMemory creates a closed seed containing the current memory schema.
func NewMemory(ctx context.Context) (*Seed, error) {
	return newSeed(ctx, initializeMemory)
}

// NewCombined creates a closed seed containing the current global and memory schemas.
func NewCombined(ctx context.Context) (*Seed, error) {
	return newSeed(ctx, func(ctx context.Context, path string) error {
		if err := initializeGlobal(ctx, path); err != nil {
			return err
		}
		return initializeMemory(ctx, path)
	})
}

func newSeed(
	ctx context.Context,
	initialize func(context.Context, string) error,
) (_ *Seed, err error) {
	if ctx == nil {
		return nil, errors.New("store seed context is required")
	}
	dir, err := os.MkdirTemp("", "agh-store-seed-*")
	if err != nil {
		return nil, fmt.Errorf("create store seed directory: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		if cleanupErr := os.RemoveAll(dir); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove failed store seed %q: %w", dir, cleanupErr))
		}
	}()

	path := filepath.Join(dir, store.GlobalDatabaseName)
	if err := initialize(ctx, path); err != nil {
		return nil, fmt.Errorf("initialize store seed: %w", err)
	}
	return &Seed{dir: dir, path: path}, nil
}

func initializeGlobal(ctx context.Context, path string) error {
	database, err := globaldb.OpenGlobalDB(ctx, path)
	if err != nil {
		return fmt.Errorf("open global schema: %w", err)
	}
	if err := database.Close(ctx); err != nil {
		return fmt.Errorf("close global schema: %w", err)
	}
	return nil
}

func initializeMemory(ctx context.Context, path string) error {
	memoryStore := memory.NewStore(
		filepath.Join(filepath.Dir(path), "memory"),
		memory.WithCatalogDatabasePath(path),
	)
	if err := memoryStore.OpenCatalog(ctx); err != nil {
		return fmt.Errorf("open memory schema: %w", err)
	}
	if err := memoryStore.CloseCatalog(ctx); err != nil {
		return fmt.Errorf("close memory schema: %w", err)
	}
	return nil
}

// Clone copies the closed seed into an isolated writable database path.
func (s *Seed) Clone(targetPath string) error {
	if s == nil || s.path == "" {
		return errors.New("store seed is not initialized")
	}
	if targetPath == "" {
		return errors.New("store seed target path is required")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create store seed target directory: %w", err)
	}
	if err := removeSQLiteFiles(targetPath); err != nil {
		return fmt.Errorf("clear store seed target: %w", err)
	}

	for _, suffix := range []string{"", "-wal", "-shm"} {
		sourcePath := s.path + suffix
		if err := copySeedFile(sourcePath, targetPath+suffix); err != nil {
			if suffix != "" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			cleanupErr := removeSQLiteFiles(targetPath)
			return errors.Join(fmt.Errorf("clone store seed: %w", err), cleanupErr)
		}
	}
	return nil
}

// Close removes the process-local seed files.
func (s *Seed) Close() error {
	if s == nil || s.dir == "" {
		return nil
	}
	dir := s.dir
	s.dir = ""
	s.path = ""
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove store seed %q: %w", dir, err)
	}
	return nil
}

func copySeedFile(sourcePath string, targetPath string) (err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open seed source %q: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close seed source %q: %w", sourcePath, closeErr))
		}
	}()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open seed target %q: %w", targetPath, err)
	}
	defer func() {
		if closeErr := target.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close seed target %q: %w", targetPath, closeErr))
		}
	}()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy seed %q to %q: %w", sourcePath, targetPath, err)
	}
	return nil
}

func removeSQLiteFiles(path string) error {
	var cleanupErr error
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("remove partial store seed %q: %w", path+suffix, err),
			)
		}
	}
	return cleanupErr
}
