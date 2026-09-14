package main

import (
	"bytes"
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
