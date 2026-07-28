package e2e

import (
	"encoding/json"

	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (c *ArtifactCollector) captureBytes(kind ArtifactKind, data []byte, mediaType string) error {
	spec, err := c.spec(kind)
	if err != nil {
		return err
	}
	if spec.isDir {
		return fmt.Errorf("artifact %s requires file collection", kind)
	}

	targetPath, err := c.targetPath(spec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s artifact parent %q: %w", kind, filepath.Dir(targetPath), err)
	}
	// #nosec G703 -- targetPath is validated by targetPath() to stay within the collector root.
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return fmt.Errorf("write %s artifact %q: %w", kind, targetPath, err)
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

func (c *ArtifactCollector) captureNamedBytes(
	kind ArtifactKind,
	name string,
	extension string,
	data []byte,
	mediaType string,
) (string, error) {
	spec, err := c.spec(kind)
	if err != nil {
		return "", err
	}
	if !spec.isDir {
		return "", fmt.Errorf("artifact %s does not accept named entries", kind)
	}

	targetDir, err := c.targetPath(spec)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s artifact dir %q: %w", kind, targetDir, err)
	}

	fileName := sanitizePathComponent(name) + extension
	targetPath := filepath.Join(targetDir, fileName)
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s named artifact %q: %w", kind, targetPath, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[kind] = ArtifactEntry{
		Kind:      kind,
		Path:      spec.relativePath,
		MediaType: strings.TrimSpace(mediaType),
	}
	if err := c.writeManifestLocked(); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (c *ArtifactCollector) spec(kind ArtifactKind) (artifactSpec, error) {
	spec, ok := artifactSpecs[kind]
	if !ok {
		return artifactSpec{}, fmt.Errorf("unknown artifact kind %q", kind)
	}
	return spec, nil
}

func (c *ArtifactCollector) targetPath(spec artifactSpec) (string, error) {
	cleanRelativePath := filepath.Clean(spec.relativePath)
	parentPrefix := ".." + string(filepath.Separator)
	if cleanRelativePath == "." || cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, parentPrefix) {
		return "", fmt.Errorf("invalid artifact path %q", spec.relativePath)
	}

	targetPath := filepath.Join(c.rootDir, cleanRelativePath)
	relativePath, err := filepath.Rel(c.rootDir, targetPath)
	if err != nil {
		return "", fmt.Errorf("rel artifact path %q: %w", targetPath, err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, parentPrefix) {
		return "", fmt.Errorf("artifact path %q escapes root %q", targetPath, c.rootDir)
	}
	return targetPath, nil
}

func (c *ArtifactCollector) manifestLocked() ArtifactManifest {
	items := make([]ArtifactEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].Kind < items[j].Kind
	})
	return ArtifactManifest{
		Version:   1,
		Artifacts: items,
	}
}

func (c *ArtifactCollector) writeManifestLocked() error {
	manifest := c.manifestLocked()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(c.manifestPath, data, 0o600); err != nil {
		return fmt.Errorf("write artifact manifest %q: %w", c.manifestPath, err)
	}
	return nil
}
