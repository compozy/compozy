package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/registry"
)

type publicationSources struct {
	GeneratedAt     string             `json:"generated_at"`
	ArtifactBaseURL string             `json:"artifact_base_url"`
	Entries         []publicationEntry `json:"entries"`
}

type publicationEntry struct {
	EntryID      string                   `json:"entry_id"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Version      string                   `json:"version,omitempty"`
	PublishedAt  string                   `json:"published_at,omitempty"`
	UpdatedAt    string                   `json:"updated_at,omitempty"`
	InstallSlug  string                   `json:"install_slug"`
	ArtifactURL  string                   `json:"artifact_url,omitempty"`
	DigestSHA256 string                   `json:"digest_sha256,omitempty"`
	Tier         string                   `json:"tier"`
	Format       string                   `json:"format,omitempty"`
	Author       string                   `json:"author,omitempty"`
	Repository   string                   `json:"repository,omitempty"`
	Icon         string                   `json:"icon,omitempty"`
	Inputs       []marketplace.EntryInput `json:"inputs,omitempty"`
}

type publicationDocument[T any] struct {
	ManifestVersion int    `json:"manifest_version"`
	GeneratedAt     string `json:"generated_at"`
	Entries         []T    `json:"entries"`
}

func readPublicationSources(directory string) (*publicationSources, error) {
	// #nosec G703 -- directory is the explicit local catalog root selected by the publishing operator.
	raw, err := os.ReadFile(filepath.Join(directory, "sources.json"))
	if err != nil {
		return nil, fmt.Errorf("read catalog sources: %w", err)
	}
	var sources publicationSources
	if err := decodePublicationJSON(raw, &sources); err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339, sources.GeneratedAt); err != nil {
		return nil, fmt.Errorf("sources.generated_at: %w", err)
	}
	base, err := url.Parse(sources.ArtifactBaseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" ||
		base.Fragment != "" {
		return nil, errors.New(
			"sources.artifact_base_url must be an absolute HTTPS URL without credentials, query or fragment",
		)
	}
	if err := validatePublicationEntries(sources.Entries); err != nil {
		return nil, err
	}
	return &sources, nil
}

func validatePublicationEntries(entries []publicationEntry) error {
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if err := marketplace.ValidateIcon(entry.Icon); err != nil {
			return fmt.Errorf("source %q: %w", entry.EntryID, err)
		}
		if entry.EntryID == "" || entry.EntryID == "." || entry.EntryID == ".." ||
			strings.ContainsAny(entry.EntryID, "/\\") {
			return errors.New("sources.entry_id must be one package directory name")
		}
		if seen[entry.EntryID] {
			return fmt.Errorf("duplicate source entry %q", entry.EntryID)
		}
		seen[entry.EntryID] = true
		if entry.Version != "" || entry.ArtifactURL != "" || entry.DigestSHA256 != "" || len(entry.Inputs) > 0 {
			return fmt.Errorf("source %q version, artifacts and inputs must come from its package", entry.EntryID)
		}
	}
	if len(entries) == 0 {
		return errors.New("catalog sources must contain entries")
	}
	return nil
}

func decodePublicationJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode catalog source: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("catalog source must contain exactly one JSON value")
	}
	return nil
}

func packagePublicationEntry(
	sourceDir, stage string,
	sources *publicationSources,
	entry publicationEntry,
) (publicationEntry, error) {
	root := filepath.Join(sourceDir, "packages", entry.EntryID)
	manifest, err := extensionpkg.LoadManifest(root)
	if err != nil {
		return publicationEntry{}, err
	}
	if manifest.Name != entry.EntryID {
		return publicationEntry{}, fmt.Errorf("package %q manifest name is %q", entry.EntryID, manifest.Name)
	}
	filename := entry.EntryID + "-v" + manifest.Version + ".tar.gz"
	artifact := filepath.Join(stage, "artifacts", filename)
	if err := packageCatalogExtension(root, artifact); err != nil {
		return publicationEntry{}, err
	}
	if err := preservePublishedArtifact(filepath.Join(sourceDir, "artifacts", filename), artifact); err != nil {
		return publicationEntry{}, err
	}
	digest, err := marketplace.DigestFile(artifact)
	if err != nil {
		return publicationEntry{}, err
	}
	entry.Version = manifest.Version
	entry.ArtifactURL = strings.TrimRight(sources.ArtifactBaseURL, "/") + "/" + filename
	entry.DigestSHA256 = digest
	entry.Inputs = manifest.Inputs
	return entry, nil
}

// Compression output can change between Go releases; unchanged package contents retain their published bytes.
func preservePublishedArtifact(existing, generated string) error {
	// #nosec G703 -- existing is an artifact beneath the operator-selected catalog source.
	published, err := os.Open(existing)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	raw, readErr := io.ReadAll(io.LimitReader(published, registry.DefaultMaxArchiveSize+1))
	if err := errors.Join(readErr, published.Close()); err != nil {
		return err
	}
	if int64(len(raw)) > registry.DefaultMaxArchiveSize {
		return errors.New("catalog artifact exceeds the compressed size limit")
	}
	oldDigest, err := uncompressedArtifactDigest(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("read published artifact: %w", err)
	}
	// #nosec G703 -- generated is an artifact in the fresh publication staging directory.
	file, err := os.Open(generated)
	if err != nil {
		return err
	}
	newDigest, digestErr := uncompressedArtifactDigest(file)
	if err := errors.Join(digestErr, file.Close()); err != nil {
		return err
	}
	if oldDigest != newDigest {
		return nil
	}
	// #nosec G306 G703 -- generated is a public artifact in the fresh publication staging directory.
	return os.WriteFile(generated, raw, 0o644)
}

func uncompressedArtifactDigest(reader io.Reader) ([sha256.Size]byte, error) {
	decompressed, err := gzip.NewReader(reader)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	digest := sha256.New()
	n, copyErr := io.CopyN(digest, decompressed, registry.DefaultMaxDecompressedSize+1)
	if errors.Is(copyErr, io.EOF) {
		copyErr = nil
	}
	if n > registry.DefaultMaxDecompressedSize {
		copyErr = errors.New("catalog artifact exceeds the decompressed size limit")
	}
	if err := errors.Join(copyErr, decompressed.Close()); err != nil {
		return [sha256.Size]byte{}, err
	}
	return [sha256.Size]byte(digest.Sum(nil)), nil
}
