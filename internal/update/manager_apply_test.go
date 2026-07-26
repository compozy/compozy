package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerApplyRelease(t *testing.T) {
	t.Run("Should apply a verified archive and preserve rollback metadata", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{}
		applier := &recordingBinaryApplier{}
		manager, executablePath := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		archiveBody := createTarGzBinary(t, "agh", []byte("#!/bin/sh\necho updated\n"), 0o755)
		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody: archiveBody,
			bundleBody:  []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle+json;version=0.3"}`),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		applied, err := manager.ApplyRelease(context.Background(), release)
		if err != nil {
			t.Fatalf("ApplyRelease() error = %v", err)
		}
		if verifier.calls != 1 {
			t.Fatalf("VerifyChecksums() calls = %d, want 1", verifier.calls)
		}
		if applier.applyCalls != 1 {
			t.Fatalf("ApplyBinary() calls = %d, want 1", applier.applyCalls)
		}
		if applied.TargetPath != executablePath || applied.Version != "v1.1.0" {
			t.Fatalf("applied = %#v, want executable %q and version v1.1.0", applied, executablePath)
		}
		if !strings.Contains(applied.BackupPath, ".agh.agh-backup-") {
			t.Fatalf("applied.BackupPath = %q, want sibling backup naming", applied.BackupPath)
		}
		if applier.targetPath != executablePath || applier.mode != 0o755 {
			t.Fatalf("binary apply = %#v, want target %q mode 0755", applier, executablePath)
		}
		if got := string(applier.sourceBytes); !strings.Contains(got, "updated") {
			t.Fatalf("applier.sourceBytes = %q, want extracted updated binary", got)
		}
	})

	t.Run("Should preserve executable mode when archive mode is not executable", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{}
		applier := &recordingBinaryApplier{}
		manager, executablePath := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		archiveBody := createTarGzBinary(t, "agh", []byte("#!/bin/sh\necho updated\n"), 0o644)
		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody: archiveBody,
			bundleBody:  []byte("{\"mediaType\":\"application/vnd.dev.sigstore.bundle+json;version=0.3\"}"),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		_, err := manager.ApplyRelease(context.Background(), release)
		if err != nil {
			t.Fatalf("ApplyRelease() error = %v", err)
		}
		if applier.targetPath != executablePath || applier.mode != 0o755 {
			t.Fatalf("binary apply = %#v, want target %q mode 0755", applier, executablePath)
		}
	})

	t.Run("Should reject oversized archive download before verification", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{}
		applier := &recordingBinaryApplier{}
		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody:          []byte("unused"),
			archiveContentLength: fmt.Sprintf("%d", maxArchiveDownloadBytes+1),
			bundleBody:           []byte("{}"),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		_, err := manager.ApplyRelease(context.Background(), release)
		if err == nil {
			t.Fatal("ApplyRelease() error = nil, want oversized archive failure")
		}
		if !strings.Contains(err.Error(), "exceeds limit") {
			t.Fatalf("ApplyRelease() error = %v, want download limit failure", err)
		}
		if verifier.calls != 0 {
			t.Fatalf("VerifyChecksums() calls = %d, want 0 after oversized archive", verifier.calls)
		}
		if applier.applyCalls != 0 {
			t.Fatalf("ApplyBinary() calls = %d, want 0 after oversized archive", applier.applyCalls)
		}
	})

	t.Run("Should reject releases that do not publish the checksum bundle", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:   runtimeOSLinux,
			RuntimeArch: runtimeArchAMD64,
		})

		archiveName, err := archiveAssetName(manager.runtimeOS, manager.runtimeArch)
		if err != nil {
			t.Fatalf("archiveAssetName() error = %v", err)
		}
		release := &Release{
			Version: "v1.1.0",
			Assets: []ReleaseAsset{
				{Name: archiveName, DownloadURL: "https://example.invalid/archive"},
				{Name: checksumsAssetName, DownloadURL: "https://example.invalid/checksums.txt"},
			},
		}

		_, err = manager.ApplyRelease(context.Background(), release)
		if err == nil {
			t.Fatal("ApplyRelease() error = nil, want missing bundle asset error")
		}
		if !strings.Contains(err.Error(), checksumsBundleAssetName) {
			t.Fatalf("ApplyRelease() error = %v, want missing bundle asset", err)
		}
	})

	t.Run("Should stop before swapping binaries when provenance verification fails", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{err: errors.New("invalid bundle")}
		applier := &recordingBinaryApplier{}
		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		archiveBody := createTarGzBinary(t, "agh", []byte("#!/bin/sh\necho updated\n"), 0o755)
		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody: archiveBody,
			bundleBody:  []byte(`{"invalid":true}`),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		_, err := manager.ApplyRelease(context.Background(), release)
		if err == nil {
			t.Fatal("ApplyRelease() error = nil, want provenance failure")
		}
		if !strings.Contains(err.Error(), "invalid bundle") {
			t.Fatalf("ApplyRelease() error = %v, want verifier failure", err)
		}
		if applier.applyCalls != 0 {
			t.Fatalf("ApplyBinary() calls = %d, want 0 after verifier failure", applier.applyCalls)
		}
	})

	t.Run("Should reject archives whose checksum does not match the signed catalog", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{}
		applier := &recordingBinaryApplier{}
		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		archiveBody := createTarGzBinary(t, "agh", []byte("#!/bin/sh\necho updated\n"), 0o755)
		archiveName, err := archiveAssetName(manager.runtimeOS, manager.runtimeArch)
		if err != nil {
			t.Fatalf("archiveAssetName() error = %v", err)
		}
		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody:   archiveBody,
			checksumsBody: fmt.Appendf(nil, "%s  %s\n", sha256Hex([]byte("different")), archiveName),
			bundleBody:    []byte(`{}`),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		_, err = manager.ApplyRelease(context.Background(), release)
		if err == nil {
			t.Fatal("ApplyRelease() error = nil, want checksum mismatch")
		}
		if !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("ApplyRelease() error = %v, want checksum mismatch", err)
		}
		if applier.applyCalls != 0 {
			t.Fatalf("ApplyBinary() calls = %d, want 0 after checksum mismatch", applier.applyCalls)
		}
	})

	t.Run("Should reject corrupt archives after checksum verification succeeds", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{}
		applier := &recordingBinaryApplier{}
		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		archiveBody := []byte("not-a-gzip-archive")
		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody: archiveBody,
			bundleBody:  []byte(`{}`),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		_, err := manager.ApplyRelease(context.Background(), release)
		if err == nil {
			t.Fatal("ApplyRelease() error = nil, want corrupt archive failure")
		}
		if !strings.Contains(err.Error(), "open gzip archive") {
			t.Fatalf("ApplyRelease() error = %v, want gzip failure", err)
		}
		if applier.applyCalls != 0 {
			t.Fatalf("ApplyBinary() calls = %d, want 0 after corrupt archive", applier.applyCalls)
		}
	})

	t.Run("Should fail when downloading a release asset returns a server error", func(t *testing.T) {
		t.Parallel()

		verifier := &stubBundleVerifier{}
		applier := &recordingBinaryApplier{}
		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:      runtimeOSLinux,
			RuntimeArch:    runtimeArchAMD64,
			BundleVerifier: verifier,
			BinaryApplier:  applier,
		})

		release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
			archiveBody:   []byte("unused"),
			archiveStatus: http.StatusInternalServerError,
			bundleBody:    []byte(`{}`),
		})
		defer server.Close()
		manager.httpClient = server.Client()

		_, err := manager.ApplyRelease(context.Background(), release)
		if err == nil {
			t.Fatal("ApplyRelease() error = nil, want download failure")
		}
		if !strings.Contains(err.Error(), "500 Internal Server Error") {
			t.Fatalf("ApplyRelease() error = %v, want server failure", err)
		}
		if applier.applyCalls != 0 {
			t.Fatalf("ApplyBinary() calls = %d, want 0 after download failure", applier.applyCalls)
		}
	})
}

func TestManagerDownloadFile(t *testing.T) {
	t.Parallel()

	t.Run("Should reject chunked downloads that exceed the limit", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:   runtimeOSLinux,
			RuntimeArch: runtimeArchAMD64,
		})
		oversizedBody := strings.Repeat("x", int(maxChecksumsBytes)+1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Errorf("response writer does not implement http.Flusher")
				return
			}
			w.WriteHeader(http.StatusOK)
			flusher.Flush()
			if _, err := w.Write([]byte(oversizedBody)); err != nil {
				t.Errorf("Write(%q) error = %v", r.URL.Path, err)
			}
		}))
		defer server.Close()
		manager.httpClient = server.Client()
		targetPath := filepath.Join(t.TempDir(), checksumsAssetName)

		err := manager.downloadFile(context.Background(), server.URL, targetPath, maxChecksumsBytes)
		if err == nil {
			t.Fatal("downloadFile() error = nil, want oversized chunked response failure")
		}
		if !strings.Contains(err.Error(), "exceeds limit") {
			t.Fatalf("downloadFile() error = %v, want download limit failure", err)
		}
		if _, statErr := os.Stat(targetPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want partial file removed", targetPath, statErr)
		}
	})
}

func TestManagerRestore(t *testing.T) {
	t.Parallel()

	t.Run("Should restore backup executable mode when target mode is broken", func(t *testing.T) {
		t.Parallel()

		applier := &recordingBinaryApplier{}
		manager, executablePath := newManagerWithExecutable(t, Config{
			RuntimeOS:     runtimeOSLinux,
			RuntimeArch:   runtimeArchAMD64,
			BinaryApplier: applier,
		})
		if err := os.Chmod(executablePath, 0o644); err != nil {
			t.Fatalf("Chmod(%q) error = %v", executablePath, err)
		}
		backupPath := filepath.Join(filepath.Dir(executablePath), ".agh.backup")
		if err := os.WriteFile(backupPath, []byte("backup-binary"), 0o755); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", backupPath, err)
		}

		err := manager.Restore(AppliedBinary{TargetPath: executablePath, BackupPath: backupPath})
		if err != nil {
			t.Fatalf("Restore() error = %v", err)
		}
		if applier.restoreCalls != 1 {
			t.Fatalf("RestoreBinary() calls = %d, want 1", applier.restoreCalls)
		}
		if applier.targetPath != executablePath || applier.backupPath != backupPath || applier.mode != 0o755 {
			t.Fatalf("binary restore = %#v, want backup %q target %q mode 0755", applier, backupPath, executablePath)
		}
	})
}

type assetFixture struct {
	archiveBody          []byte
	checksumsBody        []byte
	bundleBody           []byte
	archiveStatus        int
	archiveContentLength string
}

func newReleaseFixtureServer(
	t *testing.T,
	manager *Manager,
	fixture assetFixture,
) (*Release, string, *httptest.Server) {
	t.Helper()

	archiveName, err := archiveAssetName(manager.runtimeOS, manager.runtimeArch)
	if err != nil {
		t.Fatalf("archiveAssetName() error = %v", err)
	}

	checksumsBody := fixture.checksumsBody
	if len(checksumsBody) == 0 {
		checksumsBody = fmt.Appendf(nil, "%s  %s\n", sha256Hex(fixture.archiveBody), archiveName)
	}

	server := newReleaseAssetServer(t, map[string]assetResponse{
		"/archive": {
			status:        fixture.archiveStatus,
			body:          fixture.archiveBody,
			contentLength: fixture.archiveContentLength,
		},
		"/checksums.txt": {
			body: checksumsBody,
		},
		"/checksums.txt.sigstore.json": {
			body: fixture.bundleBody,
		},
	})

	release := &Release{
		Version:    "v1.1.0",
		ReleaseURL: "https://github.com/compozy/agh/releases/tag/v1.1.0",
		Assets: []ReleaseAsset{
			{Name: archiveName, DownloadURL: server.URL + "/archive"},
			{Name: checksumsAssetName, DownloadURL: server.URL + "/checksums.txt"},
			{Name: checksumsBundleAssetName, DownloadURL: server.URL + "/checksums.txt.sigstore.json"},
		},
	}
	return release, archiveName, server
}
