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
	files := make(map[string][]byte, 4)
	for _, name := range []string{ManifestMac, ManifestLinux, ManifestWindows, GenerationFile} {
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
		files[path.Join(ChannelDirectory, name)] = contents
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
		assetName, err := manifestAssetName(file.URL, version, repository)
		if err != nil {
			return err
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

func manifestAssetName(assetURL, version, repository string) (string, error) {
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return "", fmt.Errorf("desktop release: parse manifest asset URL %q: %w", assetURL, err)
	}
	name := path.Base(parsed.Path)
	expectedPath := "/" + repository + "/releases/download/v" + version + "/" + name
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != expectedPath {
		return "", fmt.Errorf(
			"desktop release: manifest asset URL is not an immutable GitHub release URL: %s",
			assetURL,
		)
	}
	return name, nil
}

func bindWindowsManifestIntegrity(artifacts []Artifact, commit ChannelCommit, repository string) error {
	contents, present := commit.Files[path.Join(ChannelDirectory, ManifestWindows)]
	if !present {
		return nil // Historical generations predate Windows.
	}
	manifest, err := decodeUpdateManifest(contents, ManifestWindows)
	if err != nil {
		return err
	}
	version := commit.Generation.Version
	if manifest.Version != version || len(manifest.Files) != 1 {
		return errors.New("desktop release: Windows manifest does not match the generation")
	}
	file := manifest.Files[0]
	name, err := manifestAssetName(file.URL, version, repository)
	if err != nil {
		return err
	}
	if name != "CompozyOS-"+version+"-win-x64.exe" {
		return errors.New("desktop release: Windows manifest does not name the canonical installer")
	}
	for i := range artifacts {
		if artifacts[i].Name == name {
			if artifacts[i].Size != file.Size {
				return errors.New("desktop release: Windows manifest size does not match the remote installer")
			}
			artifacts[i].expectedSHA512 = file.SHA512
			return nil
		}
	}
	return errors.New("desktop release: Windows manifest installer is missing")
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
		ManifestMac:     {"-mac-arm64.zip", "-mac-x64.zip"},
		ManifestLinux:   {"-linux-x64.AppImage"},
		ManifestWindows: {"-win-x64.exe"},
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
