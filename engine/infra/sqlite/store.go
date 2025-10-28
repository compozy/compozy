package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Register modernc SQLite driver with database/sql.
	_ "modernc.org/sqlite"

	"github.com/compozy/compozy/pkg/logger"
)

const (
	memoryPathLiteral         = ":memory:"
	memoryDSNBase             = "file:compozy-memory?mode=memory&cache=shared"
	defaultSQLiteMaxOpenConns = 25
	defaultSQLiteMaxIdleConns = 5
	defaultConnLifetime       = time.Hour
	defaultConnIdleTime       = 15 * time.Minute
	defaultBusyTimeout        = 5 * time.Second
	defaultPingTimeout        = 5 * time.Second
)

// Store manages SQLite connections using database/sql.
type Store struct {
	db   *sql.DB
	path string
}

// NewStore opens a SQLite database with sane defaults and required PRAGMAs.
func NewStore(ctx context.Context, cfg *Config) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sqlite: config is required")
	}
	dsn, normalizedPath, err := buildDSN(cfg)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open database: %w", err)
	}
	configurePool(db, cfg)
	if err := applyBusyTimeout(ctx, db, cfg); err != nil {
		db.Close()
		return nil, err
	}
	if err := verifyConnection(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	logger.FromContext(ctx).With(
		"store_driver", "sqlite",
		"db_path", normalizedPath,
	).Info("SQLite store initialized")
	return &Store{db: db, path: normalizedPath}, nil
}

// DB exposes the underlying *sql.DB for repository usage.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close releases all resources held by the store.
func (s *Store) Close(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("sqlite: close database: %w", err)
	}
	logger.FromContext(ctx).With("store_driver", "sqlite").Info("SQLite store closed")
	return nil
}

// HealthCheck verifies core PRAGMAs and ping.
func (s *Store) HealthCheck(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("sqlite: ping failed: %w", err)
	}
	var fkEnabled int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		return fmt.Errorf("sqlite: read foreign_keys pragma: %w", err)
	}
	if fkEnabled != 1 {
		return fmt.Errorf("sqlite: foreign_keys pragma disabled")
	}
	if s.path != memoryPathLiteral {
		var journalMode string
		if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			return fmt.Errorf("sqlite: read journal_mode pragma: %w", err)
		}
		if !strings.EqualFold(journalMode, "wal") {
			return fmt.Errorf("sqlite: journal_mode expected WAL, got %s", journalMode)
		}
	}
	return nil
}

func configurePool(db *sql.DB, cfg *Config) {
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = defaultSQLiteMaxOpenConns
	}
	db.SetMaxOpenConns(maxOpen)

	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = defaultSQLiteMaxIdleConns
	}
	db.SetMaxIdleConns(maxIdle)

	lifetime := cfg.ConnMaxLifetime
	if lifetime <= 0 {
		lifetime = defaultConnLifetime
	}
	db.SetConnMaxLifetime(lifetime)

	idleTime := cfg.ConnMaxIdleTime
	if idleTime <= 0 {
		idleTime = defaultConnIdleTime
	}
	db.SetConnMaxIdleTime(idleTime)
}

func applyBusyTimeout(ctx context.Context, db *sql.DB, cfg *Config) error {
	timeout := cfg.BusyTimeout
	if timeout <= 0 {
		timeout = defaultBusyTimeout
	}
	pr := fmt.Sprintf("PRAGMA busy_timeout = %d", timeout.Milliseconds())
	if _, err := db.ExecContext(ctx, pr); err != nil {
		return fmt.Errorf("sqlite: configure busy_timeout: %w", err)
	}
	return nil
}

func verifyConnection(ctx context.Context, db *sql.DB) error {
	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("sqlite: ping: %w", err)
	}
	return nil
}

func buildDSN(cfg *Config) (string, string, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return "", "", fmt.Errorf("sqlite: path is required")
	}
	if path == memoryPathLiteral || strings.HasPrefix(path, "file::memory:") {
		return buildMemoryDSN(path, cfg), memoryPathLiteral, nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("sqlite: resolve path: %w", err)
	}
	if err := ensureDirectory(absPath); err != nil {
		return "", "", err
	}
	if err := ensureDatabaseFile(absPath); err != nil {
		return "", "", err
	}
	dsn := makeFileURI(absPath, cfg)
	return dsn, absPath, nil
}

func buildMemoryDSN(path string, cfg *Config) string {
	base := path
	if path == memoryPathLiteral {
		base = memoryDSNBase
	}
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	timeout := cfg.BusyTimeout
	if timeout <= 0 {
		timeout = defaultBusyTimeout
	}
	values.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", timeout.Milliseconds()))
	if strings.Contains(base, "?") {
		return base + "&" + values.Encode()
	}
	return base + "?" + values.Encode()
}

func ensureDirectory(absPath string) error {
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("sqlite: create directory %q: %w", dir, err)
	}
	return nil
}

func ensureDatabaseFile(absPath string) error {
	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("sqlite: path %q is a directory", absPath)
		}
		return os.Chmod(absPath, 0o600)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("sqlite: stat path %q: %w", absPath, err)
	}
	file, createErr := os.OpenFile(absPath, os.O_RDWR|os.O_CREATE, 0o600)
	if createErr != nil {
		return fmt.Errorf("sqlite: create database file %q: %w", absPath, createErr)
	}
	return file.Close()
}

func makeFileURI(absPath string, cfg *Config) string {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "journal_mode(WAL)")
	timeout := cfg.BusyTimeout
	if timeout <= 0 {
		timeout = defaultBusyTimeout
	}
	values.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", timeout.Milliseconds()))
	u := &url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(absPath),
		RawQuery: values.Encode(),
	}
	return u.String()
}
