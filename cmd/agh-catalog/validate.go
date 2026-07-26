package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/marketplace"
	registrypkg "github.com/compozy/agh/internal/registry"
)

func validateCatalogForPublication(directory string) (err error) {
	if err := marketplace.ValidateCatalogDirectory(directory); err != nil {
		return err
	}
	// #nosec G703 -- validation intentionally reads the explicit local catalog directory selected by the operator.
	raw, err := os.ReadFile(filepath.Join(directory, "extensions.json"))
	if err != nil {
		return fmt.Errorf("agh-catalog: read extension feed: %w", err)
	}
	document, err := marketplace.DecodeDocument(marketplace.KindExtension, raw)
	if err != nil {
		return fmt.Errorf("agh-catalog: decode extension feed: %w", err)
	}
	temporaryRoot, err := os.MkdirTemp("", "agh-catalog-validation-*")
	if err != nil {
		return fmt.Errorf("agh-catalog: create artifact validation directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeCatalogValidationDirectory(temporaryRoot))
	}()
	for _, entry := range document.Entries {
		if err := validateExtensionArtifact(directory, temporaryRoot, entry); err != nil {
			return err
		}
	}
	return nil
}

func validateExtensionArtifact(catalogDir string, temporaryRoot string, entry marketplace.Entry) error {
	details, err := marketplace.ProjectEntry(entry)
	if err != nil {
		return fmt.Errorf("agh-catalog: project extension %q: %w", entry.EntryID, err)
	}
	if details.Extension == nil {
		return fmt.Errorf("agh-catalog: extension %q has no acquisition metadata", entry.EntryID)
	}
	artifactName, err := curatedArtifactFilename(details.Extension.ArtifactURL)
	if err != nil {
		return fmt.Errorf("agh-catalog: extension %q: %w", entry.EntryID, err)
	}
	artifactPath := filepath.Join(catalogDir, "artifacts", artifactName)
	result, err := registrypkg.NewInstaller(&catalogFileDownloader{
		path: artifactPath, version: entry.Version,
	}).Install(
		context.Background(),
		entry.InstallSlug,
		registrypkg.DownloadOpts{Version: entry.Version, ExpectedSHA256: entry.DigestSHA256},
		filepath.Join(temporaryRoot, entry.EntryID),
	)
	if err != nil {
		return fmt.Errorf("agh-catalog: install extension artifact %q: %w", artifactName, err)
	}
	manifest, err := extensionpkg.LoadManifest(result.InstallPath)
	if err != nil {
		return fmt.Errorf("agh-catalog: load extension artifact %q: %w", artifactName, err)
	}
	if manifest.Version != entry.Version {
		return fmt.Errorf(
			"agh-catalog: extension artifact %q version %q does not match feed version %q",
			artifactName,
			manifest.Version,
			entry.Version,
		)
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
	_ context.Context,
	slug string,
	_ registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	file, err := os.Open(d.path)
	if err != nil {
		return nil, fmt.Errorf("agh-catalog: open artifact %q: %w", d.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("agh-catalog: stat artifact %q: %w", d.path, err),
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
		return fmt.Errorf("agh-catalog: remove artifact validation directory: %w", err)
	}
	return nil
}
