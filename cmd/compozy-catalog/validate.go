package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

func validateCatalogForPublication(ctx context.Context, directory string) (err error) {
	if ctx == nil {
		return errors.New("compozy-catalog: validation context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("compozy-catalog: validate catalog: %w", err)
	}
	if err := marketplace.ValidateCatalogDirectory(directory); err != nil {
		return err
	}
	// #nosec G703 -- validation intentionally reads the explicit local catalog directory selected by the operator.
	feedPath := filepath.Join(directory, "extensions.json")
	if _, statErr := os.Stat(filepath.Join(directory, "v3")); statErr == nil {
		feedPath = filepath.Join(directory, "v3", "extensions.json")
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	raw, err := os.ReadFile(feedPath)
	if err != nil {
		return fmt.Errorf("compozy-catalog: read extension feed: %w", err)
	}
	document, err := marketplace.DecodeDocument(marketplace.KindExtension, raw)
	if err != nil {
		return fmt.Errorf("compozy-catalog: decode extension feed: %w", err)
	}
	temporaryRoot, err := os.MkdirTemp("", "compozy-catalog-validation-*")
	if err != nil {
		return fmt.Errorf("compozy-catalog: create artifact validation directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeCatalogValidationDirectory(temporaryRoot))
	}()
	for _, entry := range document.Entries {
		if err := validateExtensionArtifact(
			ctx,
			directory,
			temporaryRoot,
			entry,
			document.ManifestVersion == marketplace.ManifestVersionV3,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateExtensionArtifact(
	ctx context.Context,
	catalogDir string,
	temporaryRoot string,
	entry marketplace.Entry,
	requireInputs bool,
) error {
	details, err := marketplace.ProjectEntry(entry)
	if err != nil {
		return fmt.Errorf("compozy-catalog: project extension %q: %w", entry.EntryID, err)
	}
	if details.Extension == nil {
		return fmt.Errorf("compozy-catalog: extension %q has no acquisition metadata", entry.EntryID)
	}
	artifactName, err := curatedArtifactFilename(details.Extension.ArtifactURL)
	if err != nil {
		return fmt.Errorf("compozy-catalog: extension %q: %w", entry.EntryID, err)
	}
	artifactPath := filepath.Join(catalogDir, "artifacts", artifactName)
	result, err := registrypkg.NewInstaller(&catalogFileDownloader{
		path: artifactPath, version: entry.Version,
	}).Install(
		ctx,
		entry.InstallSlug,
		registrypkg.DownloadOpts{Version: entry.Version, ExpectedSHA256: entry.DigestSHA256},
		filepath.Join(temporaryRoot, entry.EntryID),
	)
	if err != nil {
		return fmt.Errorf("compozy-catalog: install extension artifact %q: %w", artifactName, err)
	}
	manifest, err := extensionpkg.LoadManifest(result.InstallPath)
	if err != nil {
		return fmt.Errorf("compozy-catalog: load extension artifact %q: %w", artifactName, err)
	}
	if manifest.Version != entry.Version {
		return fmt.Errorf(
			"compozy-catalog: extension artifact %q version %q does not match feed version %q",
			artifactName,
			manifest.Version,
			entry.Version,
		)
	}

	var payload struct {
		Inputs []marketplace.EntryInput `json:"inputs"`
	}
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		return err
	}
	if requireInputs && !reflect.DeepEqual(payload.Inputs, manifest.Inputs) {
		return fmt.Errorf("compozy-catalog: extension %q feed inputs differ from packaged manifest", entry.EntryID)
	}
	return nil
}

func curatedArtifactFilename(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	separator := strings.LastIndex(trimmed, "/")
	if separator < 0 || separator == len(trimmed)-1 {
		return "", errors.New("artifact_url must end with a versioned .tar.gz filename")
	}
	filename := trimmed[separator+1:]
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.gz") || strings.ContainsAny(filename, "?#") {
		return "", errors.New("artifact_url must end with a versioned .tar.gz filename")
	}
	return filename, nil
}

type catalogFileDownloader struct {
	path    string
	version string
}

func (d *catalogFileDownloader) Download(
	ctx context.Context,
	slug string,
	_ registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	if ctx == nil {
		return nil, errors.New("compozy-catalog: download context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("compozy-catalog: download artifact: %w", err)
	}
	file, err := os.Open(d.path)
	if err != nil {
		return nil, fmt.Errorf("compozy-catalog: open artifact %q: %w", d.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("compozy-catalog: stat artifact %q: %w", d.path, err),
			file.Close(),
		)
	}
	return &registrypkg.DownloadResult{
		Reader: file, Slug: strings.TrimSpace(slug), Version: d.version,
		ContentSize: info.Size(), ContentType: "application/gzip",
	}, nil
}

func removeCatalogValidationDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("compozy-catalog: remove artifact validation directory: %w", err)
	}
	return nil
}
