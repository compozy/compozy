package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/marketplace"
)

func publishCatalog(ctx context.Context, sourceDirectory, outputDirectory string) (err error) {
	sources, err := readPublicationSources(sourceDirectory)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(outputDirectory)
	if err != nil {
		return fmt.Errorf("resolve catalog output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(output), ".compozy-catalog-publication-*")
	if err != nil {
		return fmt.Errorf("stage catalog publication: %w", err)
	}
	defer func() { err = errors.Join(err, removeCatalogTemporaryDirectory(stage)) }()
	if err := stageCatalogPublication(ctx, sourceDirectory, stage, sources); err != nil {
		return err
	}
	if err := validateCatalogForPublication(ctx, stage); err != nil {
		return err
	}
	return commitCatalogPublication(ctx, stage, output)
}

func stageCatalogPublication(ctx context.Context, source, stage string, sources *publicationSources) error {
	current := make([]publicationEntry, 0, len(sources.Entries))
	for _, metadata := range sources.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := packagePublicationEntry(source, stage, sources, metadata)
		if err != nil {
			return fmt.Errorf("publish %q: %w", metadata.EntryID, err)
		}
		current = append(current, entry)
	}
	if err := writePublicationJSON(stage, "v3/extensions.json", publicationDocument[publicationEntry]{
		ManifestVersion: marketplace.ManifestVersion, GeneratedAt: sources.GeneratedAt, Entries: current,
	}); err != nil {
		return err
	}
	// #nosec G703 -- source is the explicit local catalog root selected by the publishing operator.
	raw, err := os.ReadFile(filepath.Join(source, "marketplaces.json"))
	if err != nil {
		return fmt.Errorf("read marketplace presets: %w", err)
	}
	if _, err := marketplace.DecodePresets(raw); err != nil {
		return err
	}
	// #nosec G306 G703 -- stage is a fresh MkdirTemp directory; this fixed-name feed is public static content.
	return os.WriteFile(filepath.Join(stage, "v3", "marketplaces.json"), raw, 0o644)
}

func writePublicationJSON(directory, name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	path := filepath.Join(directory, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// #nosec G306 -- generated publication JSON is public content served by the catalog host.
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func commitCatalogPublication(ctx context.Context, stage, output string) error {
	var files []string
	err := filepath.WalkDir(stage, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	})
	if err != nil {
		return err
	}
	// Artifacts sort before feeds, so a newly visible feed never points to an absent artifact.
	slices.Sort(files)
	for _, name := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		destination := filepath.Join(output, name)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, name), destination); err != nil {
			return fmt.Errorf("publish %s: %w", name, err)
		}
	}
	return nil
}
