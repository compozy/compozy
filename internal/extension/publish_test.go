package extensionpkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	registrygithub "github.com/compozy/compozy/internal/registry/github"
)

type publishUploaderStub struct {
	request registrygithub.ReleaseUploadRequest
	result  registrygithub.ReleaseUploadResult
	err     error
	calls   int
}

func (s *publishUploaderStub) UploadRelease(
	_ context.Context,
	request registrygithub.ReleaseUploadRequest,
) (registrygithub.ReleaseUploadResult, error) {
	s.calls++
	s.request = request
	return s.result, s.err
}

func TestPublishRelease(t *testing.T) {
	t.Parallel()

	t.Run("Should upload an archive and matching digest sidecar without credential fields", func(t *testing.T) {
		t.Parallel()

		generationDir := writePublishGeneration(t)
		uploader := &publishUploaderStub{result: registrygithub.ReleaseUploadResult{
			ReleaseURL: "https://github.example/acme/fixture/releases/v1.2.3",
			AssetURL:   "https://github.example/acme/fixture/releases/v1.2.3/fixture-v1.2.3.tar.gz",
		}}
		result, err := PublishRelease(t.Context(), uploader, PublishRequest{
			GenerationDir: generationDir,
			Repository:    "acme/fixture",
			TagName:       "v1.2.3",
		})
		if err != nil {
			t.Fatalf("PublishRelease() error = %v", err)
		}
		if uploader.calls != 1 {
			t.Fatalf("UploadRelease() calls = %d, want 1", uploader.calls)
		}
		digest := sha256.Sum256(uploader.request.Archive)
		wantDigest := hex.EncodeToString(digest[:])
		if result.DigestSHA256 != wantDigest {
			t.Fatalf("DigestSHA256 = %q, want %q", result.DigestSHA256, wantDigest)
		}
		wantSidecar := wantDigest + "  " + uploader.request.AssetName + "\n"
		if string(uploader.request.DigestSidecar) != wantSidecar {
			t.Fatalf("DigestSidecar = %q, want %q", uploader.request.DigestSidecar, wantSidecar)
		}
		if result.ReleaseURL != uploader.result.ReleaseURL || result.AssetURL != uploader.result.AssetURL {
			t.Fatalf("PublishRelease() = %#v, want uploader URLs", result)
		}
		requestType := reflect.TypeFor[PublishRequest]()
		for field := range requestType.Fields() {
			name := strings.ToLower(field.Name)
			if strings.Contains(name, "credential") ||
				strings.Contains(name, "token") ||
				strings.Contains(name, "secret") {
				t.Fatalf("PublishRequest field %q carries credential material", field.Name)
			}
		}
	})

	t.Run("Should fail before upload when no credential-bound uploader is available", func(t *testing.T) {
		t.Parallel()

		_, err := PublishRelease(t.Context(), nil, PublishRequest{
			GenerationDir: writePublishGeneration(t),
			Repository:    "acme/fixture",
			TagName:       "v1.2.3",
		})
		if !errors.Is(err, ErrExtensionPublishCredentialUnavailable) {
			t.Fatalf("PublishRelease() error = %v, want ErrExtensionPublishCredentialUnavailable", err)
		}
		if !strings.Contains(err.Error(), "GITHUB_TOKEN") || strings.Contains(err.Error(), "secret-sentinel") {
			t.Fatalf("PublishRelease() error = %q, want binding diagnostic without secret", err)
		}
	})
}

func writePublishGeneration(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	manifest := `[extension]
name = "fixture"
version = "1.2.3"
description = "fixture"
min_compozy_version = "0.3.0"

[capabilities]
provides = []

[permissions]
requires = []
`
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.txt"), []byte("fixture"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(fixture.txt) error = %v", err)
	}
	return dir
}
