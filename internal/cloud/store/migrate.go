// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationLockNamespace = 1_129_270_362

//go:embed migrations/*.sql
var migrations embed.FS

type MigrationReport struct {
	Version int64
	Applied int
}

func MigrateUp(ctx context.Context, directDSN string) (_ MigrationReport, retErr error) {
	config, err := postgresConfig(directDSN)
	if err != nil {
		return MigrationReport{}, err
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return MigrationReport{}, connectionError("connect to cloud Postgres for migrations", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		retErr = errors.Join(retErr, conn.Close(closeCtx))
	}()
	// Take the lock before Goose checks for pending work or initializes its version table.
	_, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1, hashtext(current_schema()))", migrationLockNamespace)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("lock cloud migrations: %w", err)
	}

	db := stdlib.OpenDB(*config)
	defer func() { retErr = errors.Join(retErr, db.Close()) }()
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return MigrationReport{}, fmt.Errorf("read embedded cloud migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return MigrationReport{}, fmt.Errorf("configure cloud migrations: %w", err)
	}
	applied, err := provider.Up(ctx)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("apply cloud migrations: %w", err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("read cloud migration version: %w", err)
	}
	return MigrationReport{Version: version, Applied: len(applied)}, nil
}
