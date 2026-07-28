package e2e

import (
	"encoding/json"

	"fmt"

	"os"

	"strings"
)

// RootDir returns the artifact root for the run.
func (c *ArtifactCollector) RootDir() string {
	return c.rootDir
}

// ManifestPath returns the stable manifest location.
func (c *ArtifactCollector) ManifestPath() string {
	return c.manifestPath
}

// ArtifactPath returns the absolute path for one captured artifact kind.
func (c *ArtifactCollector) ArtifactPath(kind ArtifactKind) (string, bool) {
	spec, ok := artifactSpecs[kind]
	if !ok {
		return "", false
	}
	targetPath, err := c.targetPath(spec)
	if err != nil {
		return "", false
	}
	return targetPath, true
}

// CaptureJSON writes one artifact as indented JSON and updates the manifest.
func (c *ArtifactCollector) CaptureJSON(kind ArtifactKind, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s artifact: %w", kind, err)
	}
	data = append(data, '\n')
	return c.captureBytes(kind, data, "application/json")
}

// CaptureText writes one text artifact and updates the manifest.
func (c *ArtifactCollector) CaptureText(kind ArtifactKind, text string) error {
	return c.captureBytes(kind, []byte(text), "text/plain")
}

// CaptureNamedJSON writes one JSON file inside a directory artifact and keeps
// the directory registered in the shared manifest.
func (c *ArtifactCollector) CaptureNamedJSON(kind ArtifactKind, name string, value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal %s named artifact %q: %w", kind, name, err)
	}
	data = append(data, '\n')
	return c.captureNamedBytes(kind, name, ".json", data, "application/json")
}

// CaptureNamedText writes one text file inside a directory artifact and keeps
// the directory registered in the shared manifest.
func (c *ArtifactCollector) CaptureNamedText(kind ArtifactKind, name string, text string) (string, error) {
	return c.captureNamedBytes(kind, name, ".txt", []byte(text), "text/plain")
}

// CaptureFile copies a single file into the canonical artifact location.
func (c *ArtifactCollector) CaptureFile(kind ArtifactKind, sourcePath string, mediaType string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read %s artifact source %q: %w", kind, sourcePath, err)
	}
	return c.captureBytes(kind, data, strings.TrimSpace(mediaType))
}

// CaptureFiles copies one or more files into a canonical directory artifact.
func (c *ArtifactCollector) CaptureFiles(kind ArtifactKind, sourcePaths []string, mediaType string) error {
	spec, err := c.spec(kind)
	if err != nil {
		return err
	}
	if !spec.isDir {
		return fmt.Errorf("artifact %s does not accept multiple files", kind)
	}

	targetDir, err := c.targetPath(spec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s artifact dir %q: %w", kind, targetDir, err)
	}

	targets, err := captureFileTargets(targetDir, sourcePaths)
	if err != nil {
		return fmt.Errorf("plan %s artifact files: %w", kind, err)
	}
	for _, target := range targets {
		if err := copyFile(target.sourcePath, target.targetPath); err != nil {
			return fmt.Errorf("copy %s artifact %q: %w", kind, target.sourcePath, err)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[kind] = ArtifactEntry{
		Kind:      kind,
		Path:      spec.relativePath,
		MediaType: strings.TrimSpace(mediaType),
	}
	return c.writeManifestLocked()
}

// Manifest returns a stable snapshot of the captured artifacts.
func (c *ArtifactCollector) Manifest() ArtifactManifest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.manifestLocked()
}

// WriteManifest persists the current manifest snapshot and returns it.
func (c *ArtifactCollector) WriteManifest() (ArtifactManifest, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	manifest := c.manifestLocked()
	if err := c.writeManifestLocked(); err != nil {
		return ArtifactManifest{}, err
	}
	return manifest, nil
}
