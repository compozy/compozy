package sessiondb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/fileutil"
)

// ownerGuardSnapshot keeps WAL recovery and shared-memory writes away from an unverified owner.
func ownerGuardSnapshot(ctx context.Context, path string) (string, func() error, error) {
	return ownerGuardSnapshotWithCopy(ctx, path, copyOwnerGuardFamily)
}

func ownerGuardSnapshotWithCopy(
	ctx context.Context,
	path string,
	copyFamily func(context.Context, string, string) error,
) (string, func() error, error) {
	wal, err := os.Stat(path + "-wal")
	if errors.Is(err, os.ErrNotExist) || (err == nil && wal.Size() == 0) {
		return sessionDBOwnerGuardDSN(path), func() error { return nil }, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("store: inspect owner guard WAL: %w", err)
	}
	directory, err := os.MkdirTemp("", "compozy-owner-guard-")
	if err != nil {
		return "", nil, fmt.Errorf("store: create owner guard snapshot: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(directory) }
	snapshot := filepath.Join(directory, "events.db")
	for attempt := 1; attempt <= 20; attempt++ {
		err = copyFamily(ctx, path, snapshot)
		if !errors.Is(err, errOwnerGuardSnapshotChanged) {
			break
		}
		if err = waitForSessionDBOpenRetry(ctx, path, attempt); err != nil {
			break
		}
		// Last-connection close checkpoints and removes WAL outside the family lease.
		// Re-select the main-only path once that checkpoint completes.
		wal, err = os.Stat(path + "-wal")
		if errors.Is(err, os.ErrNotExist) || (err == nil && wal.Size() == 0) {
			return sessionDBOwnerGuardDSN(path), cleanup, nil
		}
		if err != nil {
			break
		}
		err = errOwnerGuardSnapshotChanged
	}
	if err != nil {
		return "", nil, errors.Join(err, cleanup())
	}
	return sessionSQLiteModeDSN(snapshot, "ro"), cleanup, nil
}

var errOwnerGuardSnapshotChanged = errors.New("store: session database changed during owner snapshot")

// sqliteWALHeaderSize is the WAL header length; a WAL restart rewrites its salts.
const sqliteWALHeaderSize = 32

func copyOwnerGuardFamily(ctx context.Context, path, snapshot string) error {
	copied, err := copyOwnerGuardFamilyFiles(ctx, path, snapshot)
	if err != nil {
		return err
	}
	return copied.requireCurrent(path)
}

// ownerGuardFamilyCopy records what a snapshot copied so it can be validated.
type ownerGuardFamilyCopy struct {
	main      os.FileInfo
	wal       os.FileInfo
	walHeader []byte
}

func copyOwnerGuardFamilyFiles(ctx context.Context, path, snapshot string) (ownerGuardFamilyCopy, error) {
	main, _, err := copyOwnerGuardFile(ctx, path, snapshot)
	if err != nil {
		return ownerGuardFamilyCopy{}, err
	}
	wal, walHeader, err := copyOwnerGuardFile(ctx, path+"-wal", snapshot+"-wal")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ownerGuardFamilyCopy{}, errors.Join(errOwnerGuardSnapshotChanged, err)
		}
		return ownerGuardFamilyCopy{}, err
	}
	return ownerGuardFamilyCopy{main: main, wal: wal, walHeader: walHeader}, nil
}

// requireCurrent proves the copy is a consistent family snapshot. The family
// lease excludes checkpoints and wal_autocheckpoint is off, so only the
// last-connection close can rewrite the main file during the copy.
func (c ownerGuardFamilyCopy) requireCurrent(path string) error {
	current, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(c.main, current) || c.main.Size() != current.Size() ||
		!c.main.ModTime().Equal(current.ModTime()) ||
		sessionDBFileChangeToken(c.main) != sessionDBFileChangeToken(current) {
		return errOwnerGuardSnapshotChanged
	}
	return requireOwnerGuardWALPrefix(path+"-wal", c.wal, c.walHeader)
}

// requireOwnerGuardWALPrefix accepts frames appended by a live writer after
// the copy started. The copied prefix is still a valid WAL: SQLite recovery
// stops at the first frame whose cumulative checksum fails or that follows
// the last commit, so a torn tail or uncommitted frames are ignored. A restart
// (new header salts), truncation, or replacement invalidates the copy.
func requireOwnerGuardWALPrefix(path string, copied os.FileInfo, copiedHeader []byte) error {
	current, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Join(errOwnerGuardSnapshotChanged, err)
		}
		return err
	}
	if !os.SameFile(copied, current) || current.Size() < copied.Size() {
		return errOwnerGuardSnapshotChanged
	}
	if len(copiedHeader) < sqliteWALHeaderSize {
		// A partial header cannot identify the WAL generation; require no change.
		if current.Size() != copied.Size() || !copied.ModTime().Equal(current.ModTime()) {
			return errOwnerGuardSnapshotChanged
		}
		return nil
	}
	header, err := readOwnerGuardWALHeader(path, current)
	if err != nil {
		return err
	}
	if !bytes.Equal(header, copiedHeader) {
		return errOwnerGuardSnapshotChanged
	}
	return nil
}

func readOwnerGuardWALHeader(path string, expected os.FileInfo) (header []byte, retErr error) {
	file, err := fileutil.OpenRegularFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(errOwnerGuardSnapshotChanged, err)
		}
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(expected, info) {
		return nil, errOwnerGuardSnapshotChanged
	}
	header = make([]byte, sqliteWALHeaderSize)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, errors.Join(errOwnerGuardSnapshotChanged, err)
	}
	return header, nil
}

func copyOwnerGuardFile(
	ctx context.Context,
	source, target string,
) (info os.FileInfo, header []byte, retErr error) {
	input, err := fileutil.OpenRegularFile(source)
	if err != nil {
		return nil, nil, err
	}
	defer func() { retErr = errors.Join(retErr, input.Close()) }()
	info, err = input.Stat()
	if err != nil {
		return nil, nil, err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	defer func() { retErr = errors.Join(retErr, output.Close()) }()
	buffer := make([]byte, 128*1024)
	remaining := info.Size()
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		n, err := io.ReadFull(input, buffer[:min(int64(len(buffer)), remaining)])
		if err != nil {
			return nil, nil, errors.Join(errOwnerGuardSnapshotChanged, err)
		}
		if header == nil {
			header = slices.Clone(buffer[:min(n, sqliteWALHeaderSize)])
		}
		if _, err := output.Write(buffer[:n]); err != nil {
			return nil, nil, err
		}
		remaining -= int64(n)
	}
	return info, header, nil
}
