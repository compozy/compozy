package support

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestBuilderBuild(t *testing.T) {
	t.Parallel()

	t.Run("ShouldWriteRedactedCappedManifestedArchive", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		writeSupportTestFile(t, homePaths.LogFile, "before agh_claim_logsecret_1234567890 after\n")
		cfg := aghconfig.DefaultWithHome(homePaths)
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
					return map[string]string{"claim_token": "agh_claim_status_secret_1234567890"}, nil
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
			"agh_claim_status_secret_1234567890",
			"agh_claim_logsecret_1234567890",
		} {
			if strings.Contains(joined, rawSecret) {
				t.Fatalf("bundle artifact leaked raw secret %q in %s", rawSecret, joined)
			}
		}
		if !strings.Contains(string(files["event-summaries.json"]), "truncated") {
			t.Fatalf("event-summaries.json = %s, want truncated marker", string(files["event-summaries.json"]))
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

	t.Run("ShouldOmitStatusArtifactWhenRequestDisablesStatus", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		builder := Builder{
			HomePaths: homePaths,
			Config:    aghconfig.DefaultWithHome(homePaths),
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
		bootConfig := aghconfig.DefaultWithHome(homePaths)
		bootConfig.Defaults.Provider = "boot-provider"
		activeConfig := bootConfig
		activeConfig.Defaults.Provider = "active-provider"
		called := false
		builder := Builder{
			HomePaths: homePaths,
			Config:    bootConfig,
			ConfigSnapshot: func(context.Context) (aghconfig.Config, error) {
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
		var captured aghconfig.Config
		if err := json.Unmarshal(files["config-redacted.json"], &captured); err != nil {
			t.Fatalf("json.Unmarshal(config-redacted.json) error = %v", err)
		}
		if got, want := captured.Defaults.Provider, "active-provider"; got != want {
			t.Fatalf("config-redacted defaults.provider = %q, want %q", got, want)
		}
	})
}

func TestServiceCreate(t *testing.T) {
	t.Parallel()

	t.Run("ShouldDetachBundleBuildFromRequestCancellation", func(t *testing.T) {
		t.Parallel()

		homePaths := newSupportTestHome(t)
		svc := NewService(&Builder{
			HomePaths: homePaths,
			Config:    aghconfig.DefaultWithHome(homePaths),
			Now:       func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) },
			Sources: Sources{
				Status: func(ctx context.Context) (any, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					return map[string]string{"status": "ok"}, nil
				},
			},
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		created, err := svc.Create(ctx, CreateRequest{IncludeStatus: true})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		deadline := time.After(2 * time.Second)
		for {
			operation, err := svc.Get(t.Context(), created.OperationID)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if operation.Status == OperationCompleted {
				if _, err := os.Stat(operation.FilePath); err != nil {
					t.Fatalf("Stat(%q) error = %v", operation.FilePath, err)
				}
				return
			}
			if operation.Status == OperationFailed {
				t.Fatalf("operation failed after detached create: %s", operation.FailureReason)
			}
			select {
			case <-deadline:
				t.Fatal("support bundle operation did not complete before deadline")
			case <-time.After(10 * time.Millisecond):
			}
		}
	})
}

func newSupportTestHome(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
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
