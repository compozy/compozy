package desktoprelease

import (
	"fmt"
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

func validateManifestInventory(manifest updateManifest, name, version string) error {
	var expected []string
	switch name {
	case ManifestMac:
		expected = []string{
			"CompozyOS-" + version + "-mac-arm64.zip",
			"CompozyOS-" + version + "-mac-x64.zip",
		}
	case ManifestLinux:
		expected = []string{"CompozyOS-" + version + "-linux-x64.AppImage"}
	default:
		return fmt.Errorf("desktop release: unsupported channel manifest %s", name)
	}
	actual := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		parsed, err := url.Parse(file.URL)
		if err != nil {
			return fmt.Errorf("desktop release: parse manifest asset URL %q: %w", file.URL, err)
		}
		assetName := path.Base(parsed.Path)
		expectedSuffix := "/releases/download/v" + version + "/" + assetName
		if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.RawQuery != "" || parsed.Fragment != "" ||
			!strings.HasSuffix(parsed.Path, expectedSuffix) {
			return fmt.Errorf(
				"desktop release: manifest %s asset URL is not an immutable GitHub release URL: %s",
				name,
				file.URL,
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
