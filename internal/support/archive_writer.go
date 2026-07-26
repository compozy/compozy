package support

import (
	"archive/tar"

	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"time"

	"github.com/compozy/agh/internal/diagnostics"
)

type bundleArchiveWriter struct {
	tar      *tar.Writer
	maxBytes int64
	written  int64
	now      time.Time
}

func (w *bundleArchiveWriter) addJSON(
	path string,
	value any,
	maxBytes int64,
	redact bool,
	manifest *Manifest,
) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("support: marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	if redact {
		data = []byte(diagnostics.Redact(string(data)))
	}
	truncated := false
	if int64(len(data)) > maxBytes {
		data = []byte(truncatedJSONArtifact)
		truncated = true
	}
	redaction := ""
	if redact {
		redaction = redactionVersion
	}
	return w.addBytes(path, data, truncated, redaction, manifest)
}

func (w *bundleArchiveWriter) addBytes(
	path string,
	data []byte,
	truncated bool,
	redaction string,
	manifest *Manifest,
) error {
	if w == nil || w.tar == nil {
		return errors.New("support: archive writer is required")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("support: artifact path is required")
	}
	if w.maxBytes > 0 && w.written+int64(len(data)) > w.maxBytes {
		return fmt.Errorf("support: artifact %s exceeds bundle cap", path)
	}
	header := &tar.Header{Name: path, Mode: 0o600, Size: int64(len(data)), ModTime: w.now}
	if err := w.tar.WriteHeader(header); err != nil {
		return fmt.Errorf("support: write %s header: %w", path, err)
	}
	if _, err := w.tar.Write(data); err != nil {
		return fmt.Errorf("support: write %s content: %w", path, err)
	}
	w.written += int64(len(data))
	manifest.Artifacts = append(manifest.Artifacts, ManifestArtifact{
		Path:             path,
		Included:         true,
		Bytes:            int64(len(data)),
		Truncated:        truncated,
		RedactionVersion: redaction,
	})
	return nil
}

func (w *bundleArchiveWriter) addManifestJSON(manifest *Manifest, maxBytes int64) error {
	if manifest == nil {
		return errors.New("support: manifest is required")
	}
	const path = "manifest.json"
	manifest.upsertArtifact(ManifestArtifact{Path: path, Included: true})
	var data []byte
	truncated := false
	for range 8 {
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("support: marshal %s: %w", path, err)
		}
		encoded = append(encoded, '\n')
		truncated = false
		if int64(len(encoded)) > maxBytes {
			encoded = []byte(truncatedJSONArtifact)
			truncated = true
		}
		previous := manifest.artifact(path)
		previousBytes := int64(0)
		previousTruncated := false
		previousFound := false
		if previous != nil {
			previousBytes = previous.Bytes
			previousTruncated = previous.Truncated
			previousFound = true
		}
		entry := ManifestArtifact{
			Path:      path,
			Included:  true,
			Bytes:     int64(len(encoded)),
			Truncated: truncated,
		}
		manifest.upsertArtifact(entry)
		data = encoded
		if previousFound && previousBytes == entry.Bytes && previousTruncated == entry.Truncated {
			break
		}
	}
	return w.writeBytes(path, data)
}

func (w *bundleArchiveWriter) writeBytes(path string, data []byte) error {
	if w == nil || w.tar == nil {
		return errors.New("support: archive writer is required")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("support: artifact path is required")
	}
	if w.maxBytes > 0 && w.written+int64(len(data)) > w.maxBytes {
		return fmt.Errorf("support: artifact %s exceeds bundle cap", path)
	}
	header := &tar.Header{Name: path, Mode: 0o600, Size: int64(len(data)), ModTime: w.now}
	if err := w.tar.WriteHeader(header); err != nil {
		return fmt.Errorf("support: write %s header: %w", path, err)
	}
	if _, err := w.tar.Write(data); err != nil {
		return fmt.Errorf("support: write %s content: %w", path, err)
	}
	w.written += int64(len(data))
	return nil
}

func (m *Manifest) artifact(path string) *ManifestArtifact {
	if m == nil {
		return nil
	}
	for i := range m.Artifacts {
		if m.Artifacts[i].Path == path {
			return &m.Artifacts[i]
		}
	}
	return nil
}

func (m *Manifest) upsertArtifact(entry ManifestArtifact) {
	if m == nil {
		return
	}
	for i := range m.Artifacts {
		if m.Artifacts[i].Path == entry.Path {
			m.Artifacts[i] = entry
			return
		}
	}
	m.Artifacts = append(m.Artifacts, entry)
}

func (m *Manifest) omit(path string, reason string) {
	if m == nil {
		return
	}
	m.Artifacts = append(m.Artifacts, ManifestArtifact{
		Path:          strings.TrimSpace(path),
		Included:      false,
		OmittedReason: strings.TrimSpace(reason),
	})
}
