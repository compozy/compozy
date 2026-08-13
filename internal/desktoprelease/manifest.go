package desktoprelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BuildRequest struct {
	Version     string
	PublishedAt time.Time
	DesktopDir  string
	RuntimeDir  string
	RepoRoot    string
	OutputDir   string
	BaseURL     string
}

func BuildFeeds(ctx context.Context, request BuildRequest) error {
	if err := ValidateVersion(request.Version); err != nil {
		return err
	}
	if err := AssertExactDesktopInventory(ctx, request.DesktopDir, request.Version); err != nil {
		return err
	}
	if err := AssertRuntimeInventory(ctx, request.RuntimeDir); err != nil {
		return err
	}
	baseURL, err := validateDistributionOrigin(request.BaseURL)
	if err != nil {
		return err
	}
	heads, err := DiscoverSchemaHeads(ctx, request.RepoRoot)
	if err != nil {
		return err
	}
	latest, err := buildLatestManifest(request, baseURL)
	if err != nil {
		return err
	}
	runtime, err := buildRuntimeManifest(request, baseURL, heads)
	if err != nil {
		return err
	}
	if err := ValidateLatestManifest(latest); err != nil {
		return err
	}
	if err := ValidateRuntimeManifest(runtime); err != nil {
		return err
	}
	if err := os.MkdirAll(request.OutputDir, 0o755); err != nil {
		return fmt.Errorf("desktop release: create feed directory: %w", err)
	}
	if err := writeCanonicalJSON(filepath.Join(request.OutputDir, "latest.json"), latest); err != nil {
		return err
	}
	if err := writeCanonicalJSON(filepath.Join(request.OutputDir, "runtime.json"), runtime); err != nil {
		return err
	}
	return nil
}

func buildLatestManifest(request BuildRequest, baseURL *url.URL) (LatestManifest, error) {
	version := request.Version
	platformFiles := desktopUpdaterArtifactNames(version)
	platforms := make(map[string]UpdaterPlatform, len(platformFiles))
	for platform, name := range platformFiles {
		signatureBytes, err := os.ReadFile(filepath.Join(request.DesktopDir, name+".sig"))
		if err != nil {
			return LatestManifest{}, fmt.Errorf("desktop release: read %s signature: %w", platform, err)
		}
		signature := strings.TrimSpace(string(signatureBytes))
		platforms[platform] = UpdaterPlatform{
			Signature: signature,
			URL:       distributionURL(baseURL, "desktop", "v", version, name),
		}
	}
	return LatestManifest{
		Version:   version,
		Notes:     "CompozyOS " + version,
		PubDate:   request.PublishedAt.UTC().Format(time.RFC3339),
		Platforms: platforms,
	}, nil
}

func buildRuntimeManifest(
	request BuildRequest,
	baseURL *url.URL,
	heads map[string]uint64,
) (RuntimeManifest, error) {
	platforms := make(map[string]RuntimePlatform, len(platformKeys))
	for platform, name := range RuntimeArtifactNames() {
		path := filepath.Join(request.RuntimeDir, name)
		digest, size, err := digestFile(path)
		if err != nil {
			return RuntimeManifest{}, fmt.Errorf("desktop release: hash %s runtime artifact: %w", platform, err)
		}
		platforms[platform] = RuntimePlatform{
			URL:    distributionURL(baseURL, "desktop", "v", "runtime", request.Version, name),
			SHA256: digest,
			Size:   size,
		}
	}
	return RuntimeManifest{
		SchemaVersion: 1,
		Version:       request.Version,
		Platforms:     platforms,
		SchemaHeads:   heads,
	}, nil
}

func ValidateLatestManifest(manifest LatestManifest) error {
	if err := ValidateVersion(manifest.Version); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, manifest.PubDate); err != nil {
		return fmt.Errorf("desktop release: latest.json pub_date: %w", err)
	}
	if err := validateExactPlatforms(manifest.Platforms); err != nil {
		return err
	}
	for platform, entry := range manifest.Platforms {
		if entry.Signature == "" ||
			strings.HasPrefix(entry.Signature, "http://") ||
			strings.HasPrefix(entry.Signature, "https://") {
			return fmt.Errorf("desktop release: %s signature must be non-empty content, not a URL", platform)
		}
		if _, err := validateDistributionURL(entry.URL); err != nil {
			return fmt.Errorf("desktop release: %s updater URL: %w", platform, err)
		}
	}
	return nil
}

func ValidateRuntimeManifest(manifest RuntimeManifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("desktop release: runtime schema_version = %d, want 1", manifest.SchemaVersion)
	}
	if err := ValidateVersion(manifest.Version); err != nil {
		return err
	}
	if err := validateExactPlatforms(manifest.Platforms); err != nil {
		return err
	}
	for platform, entry := range manifest.Platforms {
		if entry.Size <= 0 {
			return fmt.Errorf("desktop release: %s runtime size must be positive", platform)
		}
		decoded, err := hex.DecodeString(entry.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("desktop release: %s runtime sha256 is malformed", platform)
		}
		if _, err := validateDistributionURL(entry.URL); err != nil {
			return fmt.Errorf("desktop release: %s runtime URL: %w", platform, err)
		}
	}
	for _, stream := range schemaStreamSpecs {
		if manifest.SchemaHeads[stream.name] == 0 {
			return fmt.Errorf("desktop release: runtime schema_heads.%s is required", stream.name)
		}
	}
	if len(manifest.SchemaHeads) != len(schemaStreamSpecs) {
		return fmt.Errorf("desktop release: runtime schema_heads contains unexpected streams")
	}
	return nil
}

func validateExactPlatforms[T any](platforms map[string]T) error {
	if len(platforms) != len(platformKeys) {
		return fmt.Errorf("desktop release: platform count = %d, want %d", len(platforms), len(platformKeys))
	}
	for _, platform := range platformKeys {
		if _, ok := platforms[platform]; !ok {
			return fmt.Errorf("desktop release: required platform %s is missing", platform)
		}
	}
	return nil
}

func validateDistributionURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("desktop release: parse distribution URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "releases.compozy.com" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("desktop release: distribution URL must use https://releases.compozy.com")
	}
	return parsed, nil
}

func validateDistributionOrigin(raw string) (*url.URL, error) {
	parsed, err := validateDistributionURL(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("desktop release: distribution origin must not include a path")
	}
	return parsed, nil
}

func desktopUpdaterArtifactNames(version string) map[string]string {
	appName := "CompozyOS_" + version + "_universal.app.tar.gz"
	return map[string]string{
		platformDarwinAArch64: appName,
		platformDarwinX8664:   appName,
		platformLinuxX8664:    "CompozyOS_" + version + "_amd64.AppImage",
	}
}

func distributionURL(base *url.URL, segments ...string) string {
	clone := *base
	escaped := make([]string, len(segments))
	for index, segment := range segments {
		escaped[index] = url.PathEscape(segment)
	}
	clone.Path = strings.TrimRight(clone.Path, "/") + "/" + strings.Join(escaped, "/")
	return clone.String()
}

func digestFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	digest := sha256.New()
	size, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	if copyErr != nil {
		return "", 0, fmt.Errorf("read artifact: %w", copyErr)
	}
	if closeErr != nil {
		return "", 0, fmt.Errorf("close artifact: %w", closeErr)
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}
