package extensionpkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/fileutil"
	registrygithub "github.com/compozy/compozy/internal/registry/github"
)

var ErrExtensionPublishCredentialUnavailable = errors.New("extension: publish credential unavailable")

// PublishRequest identifies one immutable generation and its GitHub release.
// The uploader owns authentication; credentials never enter this value.
type PublishRequest struct {
	GenerationDir string
	Repository    string
	TagName       string
	Draft         bool
}

// PublishResult reports the release locations and archive integrity digest.
type PublishResult struct {
	ReleaseURL   string `json:"release_url"`
	AssetURL     string `json:"asset_url"`
	DigestSHA256 string `json:"digest_sha256"`
}

// PublishRelease archives a validated generation and uploads it with a digest sidecar.
func PublishRelease(
	ctx context.Context,
	uploader registrygithub.ReleaseUploader,
	request PublishRequest,
) (*PublishResult, error) {
	if uploader == nil {
		return nil, fmt.Errorf(
			"%w: configure GITHUB_TOKEN or vault:github/publish",
			ErrExtensionPublishCredentialUnavailable,
		)
	}
	if strings.TrimSpace(request.Repository) == "" {
		return nil, errors.New("extension: publish repository is required")
	}
	if strings.TrimSpace(request.TagName) == "" {
		return nil, errors.New("extension: publish tag is required")
	}
	manifest, err := LoadManifest(strings.TrimSpace(request.GenerationDir))
	if err != nil {
		return nil, fmt.Errorf("extension: load publish generation: %w", err)
	}
	archive, err := fileutil.TarGzipDirectory(strings.TrimSpace(request.GenerationDir), nil)
	if err != nil {
		return nil, fmt.Errorf("extension: archive publish generation: %w", err)
	}
	digestBytes := sha256.Sum256(archive)
	digest := hex.EncodeToString(digestBytes[:])
	assetName := publishAssetName(manifest.Name, request.TagName)
	sidecar := []byte(digest + "  " + assetName + "\n")

	result, err := uploader.UploadRelease(ctx, registrygithub.ReleaseUploadRequest{
		Repository:    strings.TrimSpace(request.Repository),
		TagName:       strings.TrimSpace(request.TagName),
		Draft:         request.Draft,
		AssetName:     assetName,
		Archive:       archive,
		DigestSidecar: sidecar,
	})
	if err != nil {
		if errors.Is(err, registrygithub.ErrPublishCredentialUnavailable) {
			return nil, fmt.Errorf(
				"%w: configure GITHUB_TOKEN or vault:github/publish",
				ErrExtensionPublishCredentialUnavailable,
			)
		}
		return nil, fmt.Errorf("extension: publish release: %w", err)
	}
	return &PublishResult{
		ReleaseURL:   result.ReleaseURL,
		AssetURL:     result.AssetURL,
		DigestSHA256: digest,
	}, nil
}

func publishAssetName(name string, tag string) string {
	return safePublishAssetPart(name) + "-" + safePublishAssetPart(tag) + ".tar.gz"
}

func safePublishAssetPart(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, character := range strings.TrimSpace(value) {
		allowed := unicode.IsLetter(character) || unicode.IsDigit(character) || character == '.' || character == '_'
		if allowed {
			builder.WriteRune(character)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		//nolint:goconst // this is an archive-name fallback, independent of authored-context keys.
		return "extension"
	}
	return result
}
