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
	v2decoder "github.com/compozy/compozy/internal/marketplace/testdata/v2decoder"
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
	defer func() { err = errors.Join(err, removeCatalogValidationDirectory(stage)) }()
	if err := stageCatalogPublication(ctx, sourceDirectory, stage, sources); err != nil {
		return err
	}
	if err := validateCatalogForPublication(ctx, stage); err != nil {
		return err
	}
	if err := validateReleasedPublication(stage); err != nil {
		return err
	}
	return commitCatalogPublication(ctx, stage, output)
}

func stageCatalogPublication(ctx context.Context, source, stage string, sources *publicationSources) error {
	current := make([]publicationEntry, 0, len(sources.Entries))
	retained := make([]publicationEntry, 0, len(sources.Entries))
	servers := make([]publicationMCPEntry, 0)
	for _, metadata := range sources.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, manifest, err := packagePublicationEntry(source, stage, sources, metadata)
		if err != nil {
			return fmt.Errorf("publish %q: %w", metadata.EntryID, err)
		}
		server, err := retainedMCPEntry(entry, manifest)
		if err != nil {
			return err
		}
		if server != nil {
			servers = append(servers, *server)
		}
		current = append(current, entry)
		entry.Inputs, entry.Icon = nil, ""
		retained = append(retained, entry)
	}
	files := []struct {
		path     string
		document any
	}{
		{"v3/extensions.json", publicationDocument[publicationEntry]{3, sources.GeneratedAt, current}},
		{"extensions.json", publicationDocument[publicationEntry]{2, sources.GeneratedAt, retained}},
		{"mcp.json", publicationDocument[publicationMCPEntry]{2, sources.GeneratedAt, servers}},
		{"skills.json", publicationDocument[json.RawMessage]{2, sources.GeneratedAt, sources.RetainedSkills}},
	}
	for _, file := range files {
		if err := writePublicationJSON(stage, file.path, file.document); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(filepath.Join(source, "marketplaces.json"))
	if err != nil {
		return fmt.Errorf("read marketplace presets: %w", err)
	}
	if _, err := marketplace.DecodePresets(raw); err != nil {
		return err
	}
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
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func validateReleasedPublication(directory string) error {
	files := []struct {
		name string
		kind v2decoder.Kind
	}{
		{
			"extensions.json",
			v2decoder.KindExtension,
		},
		{"mcp.json", v2decoder.KindMCP},
		{"skills.json", v2decoder.KindSkill},
	}
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(directory, file.name))
		if err != nil {
			return err
		}
		if _, err := v2decoder.DecodeDocument(file.kind, raw); err != nil {
			return fmt.Errorf("released decoder rejects %s: %w", file.name, err)
		}
	}
	return nil
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
