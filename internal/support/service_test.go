package support

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestBuilderBuild(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil context before creating bundle artifacts", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		builder := Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
		}
		var ctx context.Context
		if _, err := builder.Build(ctx, "op_nil_context", CreateRequest{}); err == nil {
			t.Fatal("Build(nil) error = nil, want context validation failure")
		}
		assertSupportDirectoryEmpty(t, BundlesDir(homePaths))
	})

	t.Run("Should write redacted capped manifested archive", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		writeSupportTestFile(t, homePaths.LogFile, "before compozy_claim_logsecret_1234567890 after\n")
		attachmentSecret := "attachment-content-must-not-enter-support-bundle"
		writeSupportTestFile(
			t,
			filepath.Join(homePaths.SessionAttachmentsDir, "ws_1", "sess_1", "att_secret"),
			attachmentSecret,
		)
		cfg := compozyconfig.DefaultWithHome(homePaths)
		now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
		builder := Builder{
			HomePaths:            homePaths,
			Config:               cfg,
			Now:                  func() time.Time { return now },
			ArtifactMaxBytes:     4096,
			LogTailMaxBytes:      2048,
			EventSummaryMaxBytes: 96,
			BundleMaxBytes:       25 << 20,
			Sources: Sources{
				Status: func(context.Context) (any, error) {
					return map[string]string{"claim_token": "compozy_claim_status_secret_1234567890"}, nil
				},
				Doctor: func(context.Context) (any, error) {
					return map[string]string{"status": "ok"}, nil
				},
				Providers: func(context.Context) (any, error) {
					return []map[string]string{{"name": "codex"}}, nil
				},
				ConfigApplyRecords: func(context.Context) (any, error) {
					return []map[string]string{{"record_id": "apply_1"}}, nil
				},
				EventSummaries: func(context.Context) (any, error) {
					return map[string]string{"payload": strings.Repeat("x", 512)}, nil
				},
				Sessions: func(context.Context) (any, error) {
					return []map[string]string{{"id": "sess_1"}}, nil
				},
			},
		}

		operation, err := builder.Build(t.Context(), "op_123", CreateRequest{IncludeStatus: true})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if operation.Status != OperationCompleted {
			t.Fatalf("Build() status = %s, want %s", operation.Status, OperationCompleted)
		}

		files := readSupportBundleArchive(t, operation.FilePath)
		for path, payload := range files {
			if bytes.Contains(payload, []byte(attachmentSecret)) {
				t.Fatalf("archive member %s leaked attachment bytes", path)
			}
		}
		for _, path := range []string{
			"status.json",
			"doctor.json",
			"providers.json",
			"config-apply-records.json",
			"event-summaries.json",
			"sessions.json",
			"config-redacted.json",
			"logs-tail.txt",
			"versions.json",
			"home-tree.json",
			"manifest.json",
		} {
			if _, ok := files[path]; !ok {
				t.Fatalf("archive missing %s; files=%v", path, supportArchivePaths(files))
			}
		}
		joined := string(files["status.json"]) + string(files["logs-tail.txt"]) + string(files["event-summaries.json"])
		for _, rawSecret := range []string{
			"compozy_claim_status_secret_1234567890",
			"compozy_claim_logsecret_1234567890",
		} {
			if strings.Contains(joined, rawSecret) {
				t.Fatalf("bundle artifact leaked raw secret %q in %s", rawSecret, joined)
			}
		}
		if !strings.Contains(string(files["event-summaries.json"]), "truncated") {
			t.Fatalf("event-summaries.json = %s, want truncated marker", string(files["event-summaries.json"]))
		}
		var homeTree []HomeTreeEntry
		if err := json.Unmarshal(files["home-tree.json"], &homeTree); err != nil {
			t.Fatalf("json.Unmarshal(home-tree.json) error = %v", err)
		}
		for _, entry := range homeTree {
			if strings.HasPrefix(entry.Path, "session-attachments") {
				t.Fatalf("home-tree.json exposed attachment path %q", entry.Path)
			}
		}

		var manifest Manifest
		if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
			t.Fatalf("json.Unmarshal(manifest.json) error = %v", err)
		}
		for _, path := range []string{"status.json", "logs-tail.txt", "manifest.json"} {
			artifact := supportManifestArtifact(t, manifest, path)
			if !artifact.Included {
				t.Fatalf("manifest artifact %s included = false, want true", path)
			}
			if artifact.Bytes <= 0 {
				t.Fatalf("manifest artifact %s bytes = %d, want positive", path, artifact.Bytes)
			}
		}
		if artifact := supportManifestArtifact(t, manifest, "event-summaries.json"); !artifact.Truncated {
			t.Fatalf("event-summaries.json manifest truncated = false, want true")
		}
	})

	t.Run("Should omit status artifact when request disables status", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		builder := Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(context.Context) (any, error) {
					return map[string]string{"status": "ok"}, nil
				},
			},
		}

		operation, err := builder.Build(t.Context(), "op_no_status", CreateRequest{IncludeStatus: false})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		files := readSupportBundleArchive(t, operation.FilePath)
		if _, ok := files["status.json"]; ok {
			t.Fatalf("archive contains status.json when IncludeStatus=false")
		}
		var manifest Manifest
		if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
			t.Fatalf("json.Unmarshal(manifest.json) error = %v", err)
		}
		artifact := supportManifestArtifact(t, manifest, "status.json")
		if artifact.Included || artifact.OmittedReason != "disabled by request" {
			t.Fatalf("status manifest artifact = %#v, want disabled omission", artifact)
		}
	})

	t.Run("Should write active config snapshot when available", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		bootConfig := compozyconfig.DefaultWithHome(homePaths)
		bootConfig.Defaults.Provider = "boot-provider"
		activeConfig := bootConfig
		activeConfig.Defaults.Provider = "active-provider"
		called := false
		builder := Builder{
			HomePaths: homePaths,
			Config:    bootConfig,
			ConfigSnapshot: func(context.Context) (compozyconfig.Config, error) {
				called = true
				return activeConfig, nil
			},
			Now: func() time.Time { return time.Date(2026, 5, 20, 13, 30, 0, 0, time.UTC) },
		}

		operation, err := builder.Build(t.Context(), "op_active_config", CreateRequest{IncludeStatus: true})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if !called {
			t.Fatal("ConfigSnapshot() was not called")
		}
		files := readSupportBundleArchive(t, operation.FilePath)
		var captured compozyconfig.Config
		if err := json.Unmarshal(files["config-redacted.json"], &captured); err != nil {
			t.Fatalf("json.Unmarshal(config-redacted.json) error = %v", err)
		}
		if got, want := captured.Defaults.Provider, "active-provider"; got != want {
			t.Fatalf("config-redacted defaults.provider = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve ordinary source failures as omissions while context is active", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		const sourceFailure = "status source unavailable"
		builder := Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 13, 45, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(context.Context) (any, error) {
					return nil, errors.New(sourceFailure)
				},
				Doctor: func(context.Context) (any, error) {
					return map[string]string{"status": "ok"}, nil
				},
			},
		}

		operation, err := builder.Build(t.Context(), "op_source_failure", CreateRequest{IncludeStatus: true})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if operation.Manifest == nil {
			t.Fatal("Build() manifest = nil")
		}
		status := supportManifestArtifact(t, *operation.Manifest, "status.json")
		if status.Included {
			t.Fatalf("status manifest artifact = %#v, want omitted", status)
		}
		if !strings.Contains(status.OmittedReason, sourceFailure) {
			t.Fatalf("status omission reason = %q, want %q", status.OmittedReason, sourceFailure)
		}
		if doctor := supportManifestArtifact(t, *operation.Manifest, "doctor.json"); !doctor.Included {
			t.Fatalf("doctor manifest artifact = %#v, want included", doctor)
		}
	})

	t.Run("Should abort before the next artifact when context is canceled", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		doctorStarted := make(chan struct{})
		builder := Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 13, 50, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(context.Context) (any, error) {
					cancel()
					return map[string]string{"status": "complete"}, nil
				},
				Doctor: func(context.Context) (any, error) {
					close(doctorStarted)
					return map[string]string{"status": "unexpected"}, nil
				},
			},
		}

		_, err := builder.Build(
			ctx,
			"op_canceled_between_artifacts",
			CreateRequest{IncludeStatus: true},
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Build() error = %v, want context.Canceled", err)
		}
		select {
		case <-doctorStarted:
			t.Fatal("Build() started doctor artifact after cancellation")
		default:
		}
		assertSupportDirectoryEmpty(t, BundlesDir(homePaths))
	})

	t.Run("Should publish distinct complete archives for concurrent same-second builds", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		const operationCount = 2
		var reachedStatus sync.WaitGroup
		reachedStatus.Add(operationCount)
		releaseStatus := make(chan struct{})
		builder := Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(ctx context.Context) (any, error) {
					reachedStatus.Done()
					select {
					case <-releaseStatus:
						return map[string]string{"status": "ok"}, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			},
		}
		type buildResult struct {
			operation Operation
			err       error
		}
		results := make(chan buildResult, operationCount)
		for _, operationID := range []string{"op_concurrent_one", "op_concurrent_two"} {
			go func(operationID string) {
				operation, err := builder.Build(t.Context(), operationID, CreateRequest{IncludeStatus: true})
				results <- buildResult{operation: operation, err: err}
			}(operationID)
		}

		reachedStatus.Wait()
		close(releaseStatus)
		first := <-results
		second := <-results
		for _, result := range []buildResult{first, second} {
			if result.err != nil {
				t.Fatalf("Build() error = %v", result.err)
			}
			if result.operation.Status != OperationCompleted {
				t.Fatalf("Build() status = %s, want %s", result.operation.Status, OperationCompleted)
			}
			if _, err := os.Stat(result.operation.FilePath); err != nil {
				t.Fatalf("Stat(%q) error = %v", result.operation.FilePath, err)
			}
			assertSupportPrivateFile(t, result.operation.FilePath)
			files := readSupportBundleArchive(t, result.operation.FilePath)
			if _, ok := files["manifest.json"]; !ok {
				t.Fatalf("archive %q missing manifest.json", result.operation.FilePath)
			}
		}
		if first.operation.FilePath == second.operation.FilePath {
			t.Fatalf("concurrent builds published the same path %q", first.operation.FilePath)
		}
	})

	t.Run("Should remove staging archive when final manifest exceeds bundle cap", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		builder := Builder{
			HomePaths:      homePaths,
			Config:         compozyconfig.DefaultWithHome(homePaths),
			Now:            func() time.Time { return time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC) },
			BundleMaxBytes: 1,
		}

		if _, err := builder.Build(t.Context(), "op_cleanup_failure", CreateRequest{}); err == nil {
			t.Fatal("Build() error = nil, want bundle-cap failure")
		}
		entries, err := os.ReadDir(BundlesDir(homePaths))
		if err != nil {
			t.Fatalf("ReadDir(%q) error = %v", BundlesDir(homePaths), err)
		}
		if len(entries) != 0 {
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			t.Fatalf("failed build left bundle files behind: %v", names)
		}
	})

	t.Run("Should not replace a pre-existing final archive", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		bundleDir := BundlesDir(homePaths)
		if err := os.MkdirAll(bundleDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", bundleDir, err)
		}
		builder := Builder{
			HomePaths:      homePaths,
			BundleMaxBytes: defaultBundleMaxBytes,
		}
		bundle, err := builder.openBundleFile(time.Date(2026, 5, 20, 14, 35, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("openBundleFile() error = %v", err)
		}
		stagingClosed := false
		t.Cleanup(func() {
			if !stagingClosed {
				if closeErr := bundle.file.Close(); closeErr != nil {
					t.Errorf("Close(%q) error = %v", bundle.tmpPath, closeErr)
				}
			}
			if removeErr := os.Remove(bundle.tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				t.Errorf("Remove(%q) error = %v", bundle.tmpPath, removeErr)
			}
		})
		assertSupportPrivateFile(t, bundle.tmpPath)
		if _, err := bundle.file.WriteString("new complete archive"); err != nil {
			t.Fatalf("Write(%q) error = %v", bundle.tmpPath, err)
		}
		if err := bundle.file.Close(); err != nil {
			t.Fatalf("Close(%q) error = %v", bundle.tmpPath, err)
		}
		stagingClosed = true

		finalPath := filepath.Join(bundleDir, "pre-existing.tar.gz")
		const originalContent = "existing complete archive"
		writeSupportTestFile(t, finalPath, originalContent)
		bundle.path = finalPath
		if _, err := builder.commitBundle(bundle); !errors.Is(err, os.ErrExist) {
			t.Fatalf("commitBundle() error = %v, want os.ErrExist", err)
		}

		content, err := os.ReadFile(finalPath)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", finalPath, err)
		}
		if got := string(content); got != originalContent {
			t.Fatalf("final archive content = %q, want %q", got, originalContent)
		}
		if _, err := os.Stat(bundle.tmpPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(staging %q) error = %v, want os.ErrNotExist", bundle.tmpPath, err)
		}
	})
}

func TestServiceCreate(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil context before admitting an operation", func(t *testing.T) {
		t.Parallel()

		svc := NewService(&Builder{})
		shutdownSupportService(t, svc)
		var ctx context.Context
		if _, err := svc.Create(ctx, CreateRequest{}); err == nil {
			t.Fatal("Create(nil) error = nil, want context validation failure")
		}
		svc.store.mu.RLock()
		operationCount := len(svc.store.ops)
		svc.store.mu.RUnlock()
		if operationCount != 0 {
			t.Fatalf("operation count = %d, want zero after rejected admission", operationCount)
		}
	})

	t.Run("Should detach bundle build from request cancellation", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		started := make(chan struct{})
		svc := NewService(&Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(ctx context.Context) (any, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				},
			},
		})
		shutdownSupportService(t, svc)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		created, err := svc.Create(ctx, CreateRequest{IncludeStatus: true})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("support bundle build did not start with a detached context")
		}
		operation, err := svc.Get(t.Context(), created.OperationID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if operation.Status == OperationFailed {
			t.Fatalf("operation failed after detached create: %s", operation.FailureReason)
		}
	})

	t.Run("Should cancel accepted work and reject new work during shutdown", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		canceled := make(chan struct{})
		doctorStarted := make(chan struct{})
		release := make(chan struct{})
		releaseOnce := sync.OnceFunc(func() { close(release) })
		t.Cleanup(releaseOnce)

		homePaths := newSupportTestHome(t)
		svc := NewService(&Builder{
			HomePaths: homePaths,
			Config:    compozyconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 15, 0, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(ctx context.Context) (any, error) {
					close(started)
					<-ctx.Done()
					close(canceled)
					<-release
					return nil, ctx.Err()
				},
				Doctor: func(context.Context) (any, error) {
					close(doctorStarted)
					return map[string]string{"status": "unexpected"}, nil
				},
			},
		})
		created, err := svc.Create(t.Context(), CreateRequest{IncludeStatus: true})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("support bundle build did not start")
		}

		shutdownErr := make(chan error, 1)
		go func() { shutdownErr <- svc.Shutdown(t.Context()) }()
		select {
		case <-canceled:
		case err := <-shutdownErr:
			t.Fatalf("Shutdown() returned before canceling accepted work: %v", err)
		case <-time.After(time.Second):
			t.Fatal("Shutdown() did not cancel accepted work")
		}
		select {
		case err := <-shutdownErr:
			t.Fatalf("Shutdown() returned before accepted work settled: %v", err)
		default:
		}
		if _, err := svc.Create(t.Context(), CreateRequest{}); !errors.Is(err, ErrServiceShuttingDown) {
			t.Fatalf("Create(after shutdown) error = %v, want ErrServiceShuttingDown", err)
		}

		releaseOnce()
		select {
		case err := <-shutdownErr:
			if err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Shutdown() did not return after accepted work settled")
		}
		select {
		case <-doctorStarted:
			t.Fatal("Shutdown() started an artifact after canceling accepted work")
		default:
		}

		operation, err := svc.Get(t.Context(), created.OperationID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if operation.Status != OperationFailed {
			t.Fatalf("operation status = %s, want %s", operation.Status, OperationFailed)
		}
	})
}

func TestServiceCleanupRetention(t *testing.T) {
	t.Run(
		"Should retain an expired operation and report a redacted diagnostic when artifact removal fails",
		func(t *testing.T) {
			t.Parallel()

			const operationID = "expired-operation"
			const artifactSecret = "compozy_claim_cleanup_artifact_secret_1234567890"
			retention := time.Hour
			now := time.Date(2026, 5, 20, 16, 0, 0, 0, time.UTC)

			homePaths := newSupportTestHome(t)
			artifactPath := filepath.Join(homePaths.HomeDir, artifactSecret+".tgz")
			writeSupportTestFile(t, artifactPath, "retain this artifact")

			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))

			svc := NewService(&Builder{
				HomePaths: homePaths,
				Config:    compozyconfig.DefaultWithHome(homePaths),
				Now:       func() time.Time { return now },
			})
			shutdownSupportService(t, svc)
			svc.retention = retention
			svc.logger = logger
			removeErr := errors.New("remove " + artifactSecret)
			removeAttempts := 0
			svc.store.removeFile = func(path string) error {
				removeAttempts++
				if removeAttempts == 1 {
					return removeErr
				}
				return os.Remove(path)
			}

			expiredAt := now.Add(-2 * retention)
			svc.store.create(operationID)
			svc.store.markCompleted(operationID, Operation{
				OperationID: operationID,
				Status:      OperationCompleted,
				FilePath:    artifactPath,
				CreatedAt:   expiredAt,
				UpdatedAt:   expiredAt,
				CompletedAt: &expiredAt,
			})

			if _, err := svc.Create(t.Context(), CreateRequest{}); err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if _, err := svc.Get(t.Context(), operationID); err != nil {
				t.Fatalf("Get(expired operation) error = %v, want retained operation", err)
			}
			if _, err := os.Stat(artifactPath); err != nil {
				t.Fatalf("Stat(retained artifact) error = %v", err)
			}

			output := logs.String()
			const diagnosticMessage = "support: retain expired bundle operation after artifact cleanup failure"
			if got, want := strings.Count(output, diagnosticMessage), 1; got != want {
				t.Fatalf("cleanup diagnostic count = %d, want %d; logs=%q", got, want, output)
			}
			for _, forbidden := range []string{artifactPath, artifactSecret} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("cleanup diagnostic leaked %q: %q", forbidden, output)
				}
			}

			if _, err := svc.Create(t.Context(), CreateRequest{}); err != nil {
				t.Fatalf("Create(retry cleanup) error = %v", err)
			}
			if _, err := svc.Get(t.Context(), operationID); !errors.Is(err, ErrOperationNotFound) {
				t.Fatalf("Get(expired operation after retry) error = %v, want ErrOperationNotFound", err)
			}
			if _, err := os.Stat(artifactPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Stat(artifact after retry) error = %v, want os.ErrNotExist", err)
			}
			if got, want := removeAttempts, 2; got != want {
				t.Fatalf("artifact removal attempts = %d, want %d", got, want)
			}
			if got, want := strings.Count(logs.String(), diagnosticMessage), 1; got != want {
				t.Fatalf("cleanup diagnostic count after retry = %d, want %d; logs=%q", got, want, logs.String())
			}
		},
	)
}

func shutdownSupportService(t *testing.T, svc *Service) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := svc.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
}

func newSupportTestHome(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func writeSupportTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assertSupportDirectoryEmpty(t *testing.T, path string) {
	t.Helper()

	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", path, err)
	}
	if len(entries) == 0 {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	t.Fatalf("directory %q contains unexpected entries: %v", path, names)
}

func assertSupportPrivateFile(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("file %q permissions = %#o, want %#o", path, got, want)
	}
}

func readSupportBundleArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("Close(%q) error = %v", path, err)
		}
	}()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip.NewReader(%q) error = %v", path, err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			t.Fatalf("gzip.Close(%q) error = %v", path, err)
		}
	}()
	reader := tar.NewReader(gzipReader)
	files := map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next(%q) error = %v", path, err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", header.Name, err)
		}
		files[header.Name] = data
	}
	return files
}

func supportManifestArtifact(t *testing.T, manifest Manifest, path string) ManifestArtifact {
	t.Helper()

	for _, artifact := range manifest.Artifacts {
		if artifact.Path == path {
			return artifact
		}
	}
	t.Fatalf("manifest missing %s; artifacts=%#v", path, manifest.Artifacts)
	return ManifestArtifact{}
}

func supportArchivePaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	return paths
}
