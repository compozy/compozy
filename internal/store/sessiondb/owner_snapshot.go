package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
)

// ownerGuardSnapshot keeps WAL recovery and shared-memory writes away from an unverified owner.
func ownerGuardSnapshot(ctx context.Context, path string) (string, func() error, error) {
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
		err = copyOwnerGuardFamily(ctx, path, snapshot)
		if !errors.Is(err, errOwnerGuardSnapshotChanged) {
			break
		}
		if err = waitForSessionDBOpenRetry(ctx, path, attempt); err != nil {
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

func copyOwnerGuardFamily(ctx context.Context, path, snapshot string) error {
	type copiedFile struct {
		path string
		info os.FileInfo
	}
	files := make([]copiedFile, 0, 2)
	for _, suffix := range []string{"", "-wal"} {
		info, err := copyOwnerGuardFile(ctx, path+suffix, snapshot+suffix)
		if err != nil {
			return err
		}
		files = append(files, copiedFile{path: path + suffix, info: info})
	}
	// Writers can append or reset WAL while the family lease excludes checkpoints.
	for _, file := range files {
		current, err := os.Stat(file.path)
		if err != nil {
			return err
		}
		if !os.SameFile(file.info, current) || file.info.Size() != current.Size() ||
			!file.info.ModTime().Equal(current.ModTime()) ||
			sessionDBFileChangeToken(file.info) != sessionDBFileChangeToken(current) {
			return errOwnerGuardSnapshotChanged
		}
	}
	return nil
}

func copyOwnerGuardFile(ctx context.Context, source, target string) (info os.FileInfo, retErr error) {
	input, err := fileutil.OpenRegularFile(source)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, input.Close()) }()
	info, err = input.Stat()
	if err != nil {
		return nil, err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, output.Close()) }()
	buffer := make([]byte, 128*1024)
	remaining := info.Size()
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := io.ReadFull(input, buffer[:min(int64(len(buffer)), remaining)])
		if err != nil {
			return nil, errors.Join(errOwnerGuardSnapshotChanged, err)
		}
		if _, err := output.Write(buffer[:n]); err != nil {
			return nil, err
		}
		remaining -= int64(n)
	}
	return info, nil
}
