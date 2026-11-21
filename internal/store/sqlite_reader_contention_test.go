//go:build sqlite_reader_probe

package store

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	readerProbeScanRows    = 16_384
	readerProbeSessionRows = 512
	readerProbePayload     = 3_500
	readerProbeSamples     = 2_048
	readerProbeInterval    = 2 * time.Millisecond
	readerProbeSessions    = 20

	readerProbeLookupSQL = "SELECT length(payload) FROM reader_events WHERE id = ?"
	readerProbeScanSQL   = "SELECT sum(length(payload)) FROM reader_events"
)

var (
	readerProbeScan    = flag.Bool("sqlite-reader-scan", false, "run one continuous scan worker")
	readerProbeProfile = flag.String("sqlite-reader-mutex-profile", "", "write a profile of the measured phase")
)

type readerProbeLookup struct {
	stmt *sql.Stmt
	rows int
}

// TestSQLiteReaderContention owns the store-level independent readers probe.
// CI runs each arm in a separate process for a clean runtime mutex profile.
func TestSQLiteReaderContention(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	core := openReaderProbePool(ctx, t, filepath.Join(dir, "core.db"), readerProbeScanRows)
	scan := prepareReaderProbeQuery(ctx, t, core, readerProbeScanSQL)
	sameFile := [][]readerProbeLookup{
		{{stmt: prepareReaderProbeQuery(ctx, t, core, readerProbeLookupSQL), rows: readerProbeScanRows}},
		{{stmt: prepareReaderProbeQuery(ctx, t, core, readerProbeLookupSQL), rows: readerProbeScanRows}},
	}
	otherFiles := make([][]readerProbeLookup, 2)
	for index := range readerProbeSessions + 1 {
		name := "telemetry.db"
		if index > 0 {
			name = fmt.Sprintf("session-%02d.db", index)
		}
		db := openReaderProbePool(ctx, t, filepath.Join(dir, name), readerProbeSessionRows)
		worker := index % len(otherFiles)
		otherFiles[worker] = append(otherFiles[worker], readerProbeLookup{
			stmt: prepareReaderProbeQuery(ctx, t, db, readerProbeLookupSQL), rows: readerProbeSessionRows,
		})
	}

	// Warm the database file in both arms. It still exceeds SQLite's per-connection
	// page cache, so repeated scans must fetch pages without relying on cold disk I/O.
	if err := runReaderProbeScan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	if *readerProbeProfile != "" {
		runtime.SetMutexProfileFraction(1)
		defer runtime.SetMutexProfileFraction(0)
	}
	stopScan := make(chan struct{})
	var scanner errgroup.Group
	var scans int
	var scanTime time.Duration
	if *readerProbeScan {
		scanning := make(chan struct{})
		scanner.Go(func() error {
			close(scanning)
			for {
				select {
				case <-stopScan:
					return nil
				default:
				}
				start := time.Now()
				if err := runReaderProbeScan(ctx, scan); err != nil {
					return err
				}
				scanTime += time.Since(start)
				scans++
			}
		})
		<-scanning
	}

	readers, readCtx := errgroup.WithContext(ctx)
	workers := append(sameFile, otherFiles...)
	samples := make([][]time.Duration, len(workers))
	start := time.Now()
	for worker, lookups := range workers {
		readers.Go(func() error {
			durations, err := measureReaderProbeLookups(readCtx, lookups, worker)
			samples[worker] = durations
			return err
		})
	}
	readErr := readers.Wait()
	elapsed := time.Since(start)
	close(stopScan)
	scanErr := scanner.Wait()
	if *readerProbeProfile != "" {
		runtime.SetMutexProfileFraction(0)
		if err := writeReaderProbeProfile(*readerProbeProfile); err != nil {
			t.Error(err)
		}
	}
	if err := errors.Join(readErr, scanErr); err != nil {
		t.Fatal(err)
	}
	if *readerProbeScan && scans == 0 {
		t.Fatal("the scan worker completed no reads")
	}
	t.Logf("reader-probe elapsed=%v scans=%d scan_time=%v", elapsed, scans, scanTime)
	logReaderProbeLatencies(t, "same-file", samples[:len(sameFile)], elapsed)
	logReaderProbeLatencies(t, "other-files", samples[len(sameFile):], elapsed)
}

func openReaderProbePool(ctx context.Context, t *testing.T, path string, rows int) *sql.DB {
	t.Helper()
	writer, err := OpenSQLiteDatabaseWithPragmas(ctx, path, []string{"wal_autocheckpoint(0)"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the writer open so the fixture retains the uncheckpointed WAL tail.
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := writer.ExecContext(
		ctx,
		"CREATE TABLE reader_events (id INTEGER PRIMARY KEY, payload BLOB)",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.ExecContext(ctx, `
WITH RECURSIVE fixture(id) AS (SELECT 1 UNION ALL SELECT id + 1 FROM fixture WHERE id < ?)
INSERT INTO reader_events SELECT id, zeroblob(?) FROM fixture`, rows, readerProbePayload); err != nil {
		t.Fatal(err)
	}
	var busy, logFrames, checkpointed int
	err = writer.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed)
	if err != nil || busy != 0 || logFrames != 0 || checkpointed != 0 {
		t.Fatalf("checkpoint seed: busy=%d frames=%d checkpointed=%d error=%v", busy, logFrames, checkpointed, err)
	}
	if _, err := writer.ExecContext(ctx, "UPDATE reader_events SET payload = zeroblob(?) WHERE id > ?",
		readerProbePayload+1, rows-rows/16); err != nil {
		t.Fatal(err)
	}
	pool, err := OpenSQLiteReadPool(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	return pool
}

func prepareReaderProbeQuery(ctx context.Context, t *testing.T, pool *sql.DB, query string) *sql.Stmt {
	t.Helper()
	conn, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	stmt, err := conn.PrepareContext(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stmt.Close(); err != nil {
			t.Error(err)
		}
	})
	return stmt
}

func runReaderProbeScan(ctx context.Context, scan *sql.Stmt) error {
	var bytes int
	if err := scan.QueryRowContext(ctx).Scan(&bytes); err != nil {
		return fmt.Errorf("scan reader events: %w", err)
	}
	if want := readerProbeScanRows*readerProbePayload + readerProbeScanRows/16; bytes != want {
		return fmt.Errorf("scan payload bytes = %d, want %d", bytes, want)
	}
	return nil
}

func measureReaderProbeLookups(
	ctx context.Context, lookups []readerProbeLookup, worker int,
) ([]time.Duration, error) {
	samples := make([]time.Duration, 0, readerProbeSamples)
	ticker := time.NewTicker(readerProbeInterval)
	defer ticker.Stop()
	for sample := range readerProbeSamples {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
		lookup := lookups[sample%len(lookups)]
		id := (sample*4_099+worker*29)%lookup.rows + 1
		var bytes int
		start := time.Now()
		err := lookup.stmt.QueryRowContext(ctx, id).Scan(&bytes)
		elapsed := time.Since(start)
		if err != nil {
			return nil, fmt.Errorf("indexed read for row %d: %w", id, err)
		}
		want := readerProbePayload
		if id > lookup.rows-lookup.rows/16 {
			want++
		}
		if bytes != want {
			return nil, fmt.Errorf("row %d payload bytes = %d, want %d", id, bytes, want)
		}
		samples = append(samples, elapsed)
	}
	return samples, nil
}

func logReaderProbeLatencies(t *testing.T, cohort string, batches [][]time.Duration, elapsed time.Duration) {
	t.Helper()
	samples := slices.Concat(batches...)
	slices.Sort(samples)
	percentile := func(p int) time.Duration { return samples[(len(samples)*p+99)/100-1] }
	t.Logf("reader-probe cohort=%s n=%d reads/sec=%.1f p50=%v p95=%v p99=%v max=%v",
		cohort, len(samples), float64(len(samples))/elapsed.Seconds(),
		percentile(50), percentile(95), percentile(99), percentile(100))
}

func writeReaderProbeProfile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create mutex profile: %w", err)
	}
	return errors.Join(pprof.Lookup("mutex").WriteTo(file, 0), file.Close())
}
