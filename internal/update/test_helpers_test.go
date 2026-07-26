package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newManagerWithExecutable(t *testing.T, cfg Config) (*Manager, string) {
	t.Helper()

	homePaths := cfg.HomePaths
	if strings.TrimSpace(homePaths.HomeDir) == "" {
		var err error
		homePaths, err = aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
	}

	binaryName := "agh"
	if strings.EqualFold(cfg.RuntimeOS, runtimeOSWindows) {
		binaryName = "agh.exe"
	}
	executablePath := filepath.Join(t.TempDir(), "bin", binaryName)
	if err := os.MkdirAll(filepath.Dir(executablePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(executablePath), err)
	}
	if err := os.WriteFile(executablePath, []byte("current-binary"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", executablePath, err)
	}

	cfg.HomePaths = homePaths
	if strings.TrimSpace(cfg.CurrentVersion) == "" {
		cfg.CurrentVersion = "v1.0.0"
	}
	if cfg.ExecutablePath == nil {
		cfg.ExecutablePath = func() (string, error) {
			return executablePath, nil
		}
	}
	if cfg.ResolveSymlinks == nil {
		cfg.ResolveSymlinks = func(path string) (string, error) {
			return path, nil
		}
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, executablePath
}

func testReleaseAssets(t testing.TB, manager *Manager) []ReleaseAsset {
	t.Helper()

	archiveName := "agh_linux_x86_64.tar.gz"
	if manager != nil {
		resolvedArchiveName, err := archiveAssetName(manager.runtimeOS, manager.runtimeArch)
		if err != nil {
			t.Fatalf("archiveAssetName() error = %v", err)
		}
		archiveName = resolvedArchiveName
	}
	return []ReleaseAsset{
		{Name: archiveName, DownloadURL: "https://downloads.example/archive"},
		{Name: checksumsAssetName, DownloadURL: "https://downloads.example/checksums.txt"},
		{Name: checksumsBundleAssetName, DownloadURL: "https://downloads.example/checksums.txt.sigstore.json"},
	}
}

func testGitHubAssets(t testing.TB, manager *Manager) []githubAssetResponse {
	t.Helper()

	assets := testReleaseAssets(t, manager)
	responses := make([]githubAssetResponse, 0, len(assets))
	for _, asset := range assets {
		responses = append(responses, githubAssetResponse{
			Name:               asset.Name,
			BrowserDownloadURL: asset.DownloadURL,
		})
	}
	return responses
}

func testCacheEntry(t testing.TB, manager *Manager, version string, releaseURL string, checkedAt time.Time) cacheEntry {
	t.Helper()

	return cacheEntry{
		LatestVersion: strings.TrimSpace(version),
		ReleaseURL:    strings.TrimSpace(releaseURL),
		PublishedAt:   checkedAt.Add(-time.Hour),
		Assets:        testReleaseAssets(t, manager),
		CheckedAt:     checkedAt,
	}
}

func jsonHTTPResponse(t *testing.T, statusCode int, payload any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(body)),
	}
}

type stubBundleVerifier struct {
	calls         int
	checksumsPath string
	bundlePath    string
	err           error
}

func (v *stubBundleVerifier) VerifyChecksums(_ context.Context, checksumsPath string, bundlePath string) error {
	v.calls++
	v.checksumsPath = checksumsPath
	v.bundlePath = bundlePath
	return v.err
}

type recordingBinaryApplier struct {
	applyCalls   int
	restoreCalls int
	sourcePath   string
	targetPath   string
	backupPath   string
	mode         os.FileMode
	sourceBytes  []byte
	applyErr     error
	restoreErr   error
}

func (a *recordingBinaryApplier) ApplyBinary(
	sourcePath string,
	targetPath string,
	backupPath string,
	mode os.FileMode,
) error {
	a.applyCalls++
	a.sourcePath = sourcePath
	a.targetPath = targetPath
	a.backupPath = backupPath
	a.mode = mode

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	a.sourceBytes = data
	return a.applyErr
}

func (a *recordingBinaryApplier) RestoreBinary(
	backupPath string,
	targetPath string,
	mode os.FileMode,
) error {
	a.restoreCalls++
	a.backupPath = backupPath
	a.targetPath = targetPath
	a.mode = mode
	return a.restoreErr
}

type assetResponse struct {
	status        int
	body          []byte
	contentLength string
}

func newReleaseAssetServer(t *testing.T, responses map[string]assetResponse) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		if strings.TrimSpace(response.contentLength) != "" {
			w.Header().Set("Content-Length", response.contentLength)
		}
		w.WriteHeader(status)
		if _, err := w.Write(response.body); err != nil {
			t.Errorf("Write(%q) error = %v", r.URL.Path, err)
		}
	}))
}

func createTarGzBinary(t *testing.T, binaryName string, content []byte, mode int64) []byte {
	t.Helper()

	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)

	header := &tar.Header{
		Name: binaryName,
		Mode: mode,
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("Close(tarWriter) error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("Close(gzipWriter) error = %v", err)
	}
	return archive.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
