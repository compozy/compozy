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
	readerProbeSessionRows = 512
	readerProbePayload     = 3_500
	readerProbeSamples     = 2_048
	readerProbeInterval    = 2 * time.Millisecond
	readerProbeSessions    = 20

	readerProbeLookupSQL = "SELECT length(payload) FROM reader_events WHERE id = ?"
	readerProbeScanSQL   = "SELECT sum(length(payload)) FROM reader_events"
)

var (
	readerProbeMode    = flag.String("sqlite-reader-mode", "", "reader probe arm: indexed or scan")
	readerProbeCoreMiB = flag.Int("sqlite-reader-core-mib", 64, "approximate core fixture size in MiB (64 or 256)")
	readerProbeProfile = flag.String("sqlite-reader-profile-prefix", "", "output prefix for mutex and block profiles")
)

type readerProbeLookup struct {
	stmt *sql.Stmt
	rows int
}

type readerProbeLatencies struct {
	service   []time.Duration
	scheduled []time.Duration
}

// TestSQLiteReaderContention owns the store-level independent readers probe.
// CI runs each arm in a separate process for a clean runtime mutex profile.
func TestSQLiteReaderContention(t *testing.T) {
	switch *readerProbeMode {
	case "":
		t.Skip("set -sqlite-reader-mode to run the SQLite reader probe")
	case "indexed", "scan":
	default:
		t.Fatalf("unknown reader probe mode %q", *readerProbeMode)
	}
	if *readerProbeCoreMiB != 64 && *readerProbeCoreMiB != 256 {
		t.Fatalf("unknown reader probe core size %d MiB", *readerProbeCoreMiB)
	}
	// The 3500-byte payload leaves one row per 4 KiB database page.
	coreRows := *readerProbeCoreMiB * 256

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	corePath := filepath.Join(dir, "core.db")
	core := openReaderProbePool(ctx, t, corePath, coreRows)
	coreFile, err := os.Stat(corePath)
	if err != nil {
		t.Fatal(err)
	}
	scan := prepareReaderProbeQuery(ctx, t, core, readerProbeScanSQL)
	sameFile := [][]readerProbeLookup{
		{{stmt: prepareReaderProbeQuery(ctx, t, core, readerProbeLookupSQL), rows: coreRows}},
		{{stmt: prepareReaderProbeQuery(ctx, t, core, readerProbeLookupSQL), rows: coreRows}},
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
	if err := runReaderProbeScan(ctx, scan, coreRows); err != nil {
		t.Fatal(err)
	}
	if *readerProbeProfile != "" {
		runtime.SetMutexProfileFraction(1)
		runtime.SetBlockProfileRate(1)
	}
	stopScan := make(chan struct{})
	var scanner errgroup.Group
	var scans int
	var scanTime time.Duration
	if *readerProbeMode == "scan" {
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
				if err := runReaderProbeScan(ctx, scan, coreRows); err != nil {
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
	samples := make([]readerProbeLatencies, len(workers))
	start := time.Now()
	for worker, lookups := range workers {
		readers.Go(func() error {
			measure := measureReaderProbeLookups
			if worker >= len(sameFile) {
				measure = measureReaderProbeOtherFiles
			}
			durations, err := measure(readCtx, lookups, worker, start)
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
		runtime.SetBlockProfileRate(0)
		for _, kind := range []string{"mutex", "block"} {
			if err := writeReaderProbeProfile(*readerProbeProfile+"."+kind+".pb.gz", kind); err != nil {
				t.Error(err)
			}
		}
	}
	if err := errors.Join(readErr, scanErr); err != nil {
		t.Fatal(err)
	}
	if *readerProbeMode == "scan" && scans == 0 {
		t.Fatal("the scan worker completed no reads")
	}
	t.Logf("reader-probe core_bytes=%d elapsed=%v scans=%d scan_time=%v", coreFile.Size(), elapsed, scans, scanTime)
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

func runReaderProbeScan(ctx context.Context, scan *sql.Stmt, rows int) error {
	var bytes int
	if err := scan.QueryRowContext(ctx).Scan(&bytes); err != nil {
		return fmt.Errorf("scan reader events: %w", err)
	}
	if want := rows*readerProbePayload + rows/16; bytes != want {
		return fmt.Errorf("scan payload bytes = %d, want %d", bytes, want)
	}
	return nil
}

func measureReaderProbeLookups(
	ctx context.Context, lookups []readerProbeLookup, worker int, start time.Time,
) (readerProbeLatencies, error) {
	samples := readerProbeLatencies{
		service:   make([]time.Duration, 0, readerProbeSamples),
		scheduled: make([]time.Duration, 0, readerProbeSamples),
	}
	timer := time.NewTimer(readerProbeInterval)
	defer timer.Stop()
	for sample := range readerProbeSamples {
		due := start.Add(time.Duration(sample+1) * readerProbeInterval)
		timer.Reset(time.Until(due))
		select {
		case <-ctx.Done():
			return samples, ctx.Err()
		case <-timer.C:
		}
		lookup := lookups[sample%len(lookups)]
		id := (sample*4_099+worker*29)%lookup.rows + 1
		var bytes int
		started := time.Now()
		err := lookup.stmt.QueryRowContext(ctx, id).Scan(&bytes)
		ended := time.Now()
		if err != nil {
			return samples, fmt.Errorf("indexed read for row %d: %w", id, err)
		}
		want := readerProbePayload
		if id > lookup.rows-lookup.rows/16 {
			want++
		}
		if bytes != want {
			return samples, fmt.Errorf("row %d payload bytes = %d, want %d", id, bytes, want)
		}
		samples.service = append(samples.service, ended.Sub(started))
		samples.scheduled = append(samples.scheduled, ended.Sub(due))
	}
	return samples, nil
}

// Keep a named frame so the block profile can identify readers on other files.
func measureReaderProbeOtherFiles(
	ctx context.Context, lookups []readerProbeLookup, worker int, start time.Time,
) (readerProbeLatencies, error) {
	return measureReaderProbeLookups(ctx, lookups, worker, start)
}

func logReaderProbeLatencies(t *testing.T, cohort string, batches []readerProbeLatencies, elapsed time.Duration) {
	t.Helper()
	var samples, scheduled []time.Duration
	for _, batch := range batches {
		samples = append(samples, batch.service...)
		scheduled = append(scheduled, batch.scheduled...)
	}
	slices.Sort(samples)
	slices.Sort(scheduled)
	percentile := func(p int) int { return (len(samples)*p+99)/100 - 1 }
	t.Logf("reader-probe cohort=%s n=%d reads/sec=%.1f p50=%v p95=%v p99=%v max=%v scheduled_p99=%v scheduled_max=%v",
		cohort, len(samples), float64(len(samples))/elapsed.Seconds(),
		samples[percentile(50)], samples[percentile(95)], samples[percentile(99)], samples[percentile(100)],
		scheduled[percentile(99)], scheduled[percentile(100)])
}

func writeReaderProbeProfile(path string, kind string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s profile: %w", kind, err)
	}
	return errors.Join(pprof.Lookup(kind).WriteTo(file, 0), file.Close())
}
