package desktoprelease

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type updateFile struct {
	URL    string `yaml:"url"`
	SHA512 string `yaml:"sha512"`
	Size   int64  `yaml:"size"`
}

type updateManifest struct {
	Version     string       `yaml:"version"`
	Files       []updateFile `yaml:"files"`
	Path        string       `yaml:"path,omitempty"`
	SHA512      string       `yaml:"sha512,omitempty"`
	ReleaseDate string       `yaml:"releaseDate"`
}

func LoadChannelFiles(dir string) (map[string][]byte, error) {
	files := make(map[string][]byte, 3)
	for _, name := range []string{ManifestMac, ManifestLinux, GenerationFile} {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("desktop release: read channel file %s: %w", name, err)
		}
		if len(contents) == 0 {
			return nil, fmt.Errorf("desktop release: channel file %s is empty", name)
		}
		if name != GenerationFile {
			if _, err := decodeUpdateManifest(contents, name); err != nil {
				return nil, err
			}
		}
		files[filepath.Join(ChannelDirectory, name)] = contents
	}
	return files, nil
}

func decodeUpdateManifest(contents []byte, name string) (updateManifest, error) {
	var manifest updateManifest
	if err := yaml.Unmarshal(contents, &manifest); err != nil {
		return updateManifest{}, fmt.Errorf("desktop release: decode update manifest %s: %w", name, err)
	}
	if err := ValidateVersion(manifest.Version); err != nil {
		return updateManifest{}, fmt.Errorf("desktop release: manifest %s: %w", name, err)
	}
	if len(manifest.Files) == 0 {
		return updateManifest{}, fmt.Errorf("desktop release: update manifest %s has no files", name)
	}
	if manifest.ReleaseDate == "" {
		return updateManifest{}, fmt.Errorf("desktop release: update manifest %s has no release date", name)
	}
	for _, file := range manifest.Files {
		if file.URL == "" || file.SHA512 == "" || file.Size <= 0 {
			return updateManifest{}, fmt.Errorf("desktop release: update manifest %s has an incomplete file", name)
		}
	}
	return manifest, nil
}

func validateManifestInventory(
	manifest updateManifest,
	name, version, repository, assetDir string,
) error {
	expected, err := manifestArtifactNames(version, name)
	if err != nil {
		return err
	}
	actual := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		parsed, err := url.Parse(file.URL)
		if err != nil {
			return fmt.Errorf("desktop release: parse manifest asset URL %q: %w", file.URL, err)
		}
		assetName := path.Base(parsed.Path)
		expectedPath := "/" + repository + "/releases/download/v" + version + "/" + assetName
		if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != expectedPath {
			return fmt.Errorf(
				"desktop release: manifest %s asset URL is not an immutable GitHub release URL: %s",
				name,
				file.URL,
			)
		}
		sha512Digest, size, err := inspectManifestIntegrity(filepath.Join(assetDir, assetName))
		if err != nil {
			return err
		}
		if file.Size != size || file.SHA512 != sha512Digest {
			return fmt.Errorf(
				"desktop release: manifest %s integrity does not match %s",
				name,
				assetName,
			)
		}
		actual = append(actual, assetName)
	}
	slices.Sort(actual)
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("desktop release: manifest %s assets = %v, want %v", name, actual, expected)
	}
	return nil
}

func inspectManifestIntegrity(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("desktop release: open manifest artifact %q: %w", filepath.Base(filePath), err)
	}
	digest := sha512.New()
	size, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	var hashErr error
	if copyErr != nil {
		hashErr = fmt.Errorf("desktop release: hash manifest artifact %q: %w", filepath.Base(filePath), copyErr)
	}
	var cleanupErr error
	if closeErr != nil {
		cleanupErr = fmt.Errorf("desktop release: close manifest artifact %q: %w", filepath.Base(filePath), closeErr)
	}
	if err := errors.Join(hashErr, cleanupErr); err != nil {
		return "", 0, err
	}
	if size <= 0 {
		return "", 0, fmt.Errorf("desktop release: manifest artifact %q is empty", filepath.Base(filePath))
	}
	return base64.StdEncoding.EncodeToString(digest.Sum(nil)), size, nil
}

func manifestArtifactNames(version string, manifestName string) ([]string, error) {
	all := DesktopArtifactNames(version)
	wantedSuffixes := map[string][]string{
		ManifestMac:   {"-mac-arm64.zip", "-mac-x64.zip"},
		ManifestLinux: {"-linux-x64.AppImage"},
	}
	suffixes, ok := wantedSuffixes[manifestName]
	if !ok {
		return nil, fmt.Errorf("desktop release: unsupported channel manifest %s", manifestName)
	}

	artifacts := make([]string, 0, len(suffixes))
	for _, artifact := range all {
		for _, suffix := range suffixes {
			if strings.HasSuffix(artifact, suffix) {
				artifacts = append(artifacts, artifact)
				break
			}
		}
	}
	if len(artifacts) != len(suffixes) {
		return nil, fmt.Errorf("desktop release: canonical inventory does not define %s artifacts", manifestName)
	}
	return artifacts, nil
}
