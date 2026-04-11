package extensions

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Discovery scans bundled, user, and workspace extension roots.
type Discovery struct {
	WorkspaceRoot   string
	HomeDir         string
	IncludeDisabled bool
	Enablement      *EnablementStore
	BundledFS       fs.FS
}

// DiscoveryResult captures raw discovered entries plus the effective set after
// precedence resolution and enablement filtering.
type DiscoveryResult struct {
	Discovered []DiscoveredExtension
	Extensions []DiscoveredExtension
	Overrides  []OverrideRecord
	Failures   []DiscoveryFailure
	Providers  DeclaredProviders
	SkillPacks DeclaredSkillPacks
}

// DiscoveredExtension describes one discovered manifest declaration.
type DiscoveredExtension struct {
	Ref          Ref
	Manifest     *Manifest
	ExtensionDir string
	ManifestPath string
	Enabled      bool

	rootFS   fs.FS
	fsBase   string
	diskRoot string
}

// OverrideRecord describes which higher-precedence declaration won for one name.
type OverrideRecord struct {
	Name   string
	Winner OverrideSubject
	Loser  OverrideSubject
	Reason string
}

// OverrideSubject captures one declaration participating in precedence.
type OverrideSubject struct {
	Source       Source
	ManifestPath string
	Version      string
}

// DiscoveryFailure reports a manifest load failure encountered during scanning.
type DiscoveryFailure struct {
	Source       Source
	ExtensionDir string
	ManifestPath string
	Err          error
}

func (f DiscoveryFailure) Error() string {
	if f.Err == nil {
		return "extension discovery failure"
	}

	return fmt.Sprintf(
		"discover %s extension at %q: %v",
		f.Source,
		f.ExtensionDir,
		f.Err,
	)
}

func (f DiscoveryFailure) Unwrap() error {
	return f.Err
}

// Discover scans the three extension levels, resolves precedence, and returns
// the effective declarations for the configured enablement view.
func (d Discovery) Discover(ctx context.Context) (DiscoveryResult, error) {
	if err := contextError(ctx, "discover extensions"); err != nil {
		return DiscoveryResult{}, err
	}

	store, homeDir, err := d.resolveEnablementStore(ctx)
	if err != nil {
		return DiscoveryResult{}, err
	}

	workspaceRoot, err := d.resolveWorkspaceRoot()
	if err != nil {
		return DiscoveryResult{}, err
	}

	discovered, failures, err := d.scanDiscovered(ctx, store, homeDir, workspaceRoot)
	if err != nil {
		return DiscoveryResult{}, err
	}

	effective, overrides := resolveEffectiveExtensions(discovered)
	filtered := filterEffectiveExtensions(effective, d.IncludeDisabled)

	result := DiscoveryResult{
		Discovered: discovered,
		Extensions: filtered,
		Overrides:  overrides,
		Failures:   failures,
	}
	result.Providers = ExtractDeclaredProviders(result.Extensions)
	result.SkillPacks = ExtractDeclaredSkillPacks(result.Extensions)

	return result, nil
}

func (d Discovery) resolveEnablementStore(ctx context.Context) (*EnablementStore, string, error) {
	if d.Enablement != nil {
		return d.Enablement, d.Enablement.homeDir, nil
	}

	store, err := NewEnablementStore(ctx, d.HomeDir)
	if err != nil {
		return nil, "", fmt.Errorf("create discovery enablement store: %w", err)
	}
	return store, store.homeDir, nil
}

func (d Discovery) resolveWorkspaceRoot() (string, error) {
	trimmed := strings.TrimSpace(d.WorkspaceRoot)
	if trimmed == "" {
		return "", nil
	}

	root, err := normalizeWorkspaceRoot(trimmed)
	if err != nil {
		return "", err
	}
	return root, nil
}

func (d Discovery) scanDiscovered(
	ctx context.Context,
	store *EnablementStore,
	homeDir string,
	workspaceRoot string,
) ([]DiscoveredExtension, []DiscoveryFailure, error) {
	discovered := make([]DiscoveredExtension, 0)
	failures := make([]DiscoveryFailure, 0)

	discovered, failures, err := d.scanBundled(ctx, store, discovered, failures)
	if err != nil {
		return nil, nil, err
	}

	userRoot := filepath.Join(homeDir, ".compozy", "extensions")
	discovered, failures, err = d.scanFilesystemRoot(
		ctx,
		store,
		SourceUser,
		userRoot,
		"",
		discovered,
		failures,
	)
	if err != nil {
		return nil, nil, err
	}

	if workspaceRoot != "" {
		workspaceExtensionsRoot := filepath.Join(workspaceRoot, ".compozy", "extensions")
		discovered, failures, err = d.scanFilesystemRoot(
			ctx,
			store,
			SourceWorkspace,
			workspaceExtensionsRoot,
			workspaceRoot,
			discovered,
			failures,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	slices.SortFunc(discovered, compareDiscoveredBySource)
	return discovered, failures, nil
}

func (d Discovery) scanBundled(
	ctx context.Context,
	store *EnablementStore,
	discovered []DiscoveredExtension,
	failures []DiscoveryFailure,
) ([]DiscoveredExtension, []DiscoveryFailure, error) {
	bundledFS := d.BundledFS
	if bundledFS == nil {
		bundledFS = defaultBundledExtensionsFS()
	}

	entries, err := fs.ReadDir(bundledFS, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("read bundled extensions root: %w", err)
	}

	for _, entry := range entries {
		if err := contextError(ctx, "scan bundled extensions"); err != nil {
			return nil, nil, err
		}
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		virtualDir := path.Join(bundledExtensionsDir, dirName)
		manifest, manifestPath, loadErr := loadManifestFromFS(ctx, bundledFS, dirName)
		if loadErr != nil {
			var notFoundErr *ManifestNotFoundError
			if errors.As(loadErr, &notFoundErr) {
				slog.Warn(
					"ignore bundled extension directory without manifest",
					slog.String("source", string(SourceBundled)),
					slog.String("extension_dir", virtualDir),
				)
				continue
			}

			failures = append(
				failures,
				logDiscoveryFailure(
					SourceBundled,
					virtualDir,
					manifestPathForFSDirectory(bundledFS, dirName, virtualDir),
					loadErr,
				),
			)
			continue
		}

		enabled, err := store.Enabled(ctx, Ref{Name: manifest.Extension.Name, Source: SourceBundled})
		if err != nil {
			return nil, nil, fmt.Errorf("resolve bundled extension enablement: %w", err)
		}

		discovered = append(discovered, DiscoveredExtension{
			Ref: Ref{
				Name:   manifest.Extension.Name,
				Source: SourceBundled,
			},
			Manifest:     manifest,
			ExtensionDir: virtualDir,
			ManifestPath: path.Join(bundledExtensionsDir, manifestPath),
			Enabled:      enabled,
			rootFS:       bundledFS,
			fsBase:       dirName,
		})
	}

	return discovered, failures, nil
}

func (d Discovery) scanFilesystemRoot(
	ctx context.Context,
	store *EnablementStore,
	source Source,
	root string,
	workspaceRoot string,
	discovered []DiscoveredExtension,
	failures []DiscoveryFailure,
) ([]DiscoveredExtension, []DiscoveryFailure, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return discovered, failures, nil
		}
		return nil, nil, fmt.Errorf("read %s extensions root %q: %w", source, root, err)
	}

	for _, entry := range entries {
		if err := contextError(ctx, "scan filesystem extensions"); err != nil {
			return nil, nil, err
		}
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(root, entry.Name())
		manifest, loadErr := LoadManifest(ctx, dirPath)
		if loadErr != nil {
			var notFoundErr *ManifestNotFoundError
			if errors.As(loadErr, &notFoundErr) {
				slog.Warn(
					"ignore extension directory without manifest",
					slog.String("source", string(source)),
					slog.String("extension_dir", dirPath),
				)
				continue
			}

			failures = append(
				failures,
				logDiscoveryFailure(source, dirPath, manifestPathForDirectory(dirPath), loadErr),
			)
			continue
		}

		ref := Ref{
			Name:          manifest.Extension.Name,
			Source:        source,
			WorkspaceRoot: workspaceRoot,
		}
		enabled, err := store.Enabled(ctx, ref)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve %s extension enablement: %w", source, err)
		}

		discovered = append(discovered, DiscoveredExtension{
			Ref:          ref,
			Manifest:     manifest,
			ExtensionDir: dirPath,
			ManifestPath: manifestPathForDirectory(dirPath),
			Enabled:      enabled,
			diskRoot:     dirPath,
		})
	}

	return discovered, failures, nil
}

func compareDiscoveredBySource(left, right DiscoveredExtension) int {
	if diff := sourceRank(left.Ref.Source) - sourceRank(right.Ref.Source); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.Ref.Name, right.Ref.Name); diff != 0 {
		return diff
	}
	return strings.Compare(left.ManifestPath, right.ManifestPath)
}

func resolveEffectiveExtensions(discovered []DiscoveredExtension) ([]DiscoveredExtension, []OverrideRecord) {
	if len(discovered) == 0 {
		return nil, nil
	}

	grouped := make(map[string][]DiscoveredExtension)
	keys := make([]string, 0)
	for i := range discovered {
		entry := discovered[i]
		key := strings.ToLower(strings.TrimSpace(entry.Ref.Name))
		if _, ok := grouped[key]; !ok {
			keys = append(keys, key)
		}
		grouped[key] = append(grouped[key], entry)
	}
	slices.Sort(keys)

	effective := make([]DiscoveredExtension, 0, len(keys))
	overrides := make([]OverrideRecord, 0)
	for _, key := range keys {
		group := append([]DiscoveredExtension(nil), grouped[key]...)
		slices.SortFunc(group, compareByPrecedence)

		winner := group[0]
		effective = append(effective, winner)
		for i := 1; i < len(group); i++ {
			loser := group[i]
			overrides = append(overrides, OverrideRecord{
				Name: winner.Ref.Name,
				Winner: OverrideSubject{
					Source:       winner.Ref.Source,
					ManifestPath: winner.ManifestPath,
					Version:      winner.Manifest.Extension.Version,
				},
				Loser: OverrideSubject{
					Source:       loser.Ref.Source,
					ManifestPath: loser.ManifestPath,
					Version:      loser.Manifest.Extension.Version,
				},
				Reason: "higher_precedence_source",
			})
		}
	}

	return effective, overrides
}

func compareByPrecedence(left, right DiscoveredExtension) int {
	if diff := sourceRank(right.Ref.Source) - sourceRank(left.Ref.Source); diff != 0 {
		return diff
	}
	return strings.Compare(left.ManifestPath, right.ManifestPath)
}

func filterEffectiveExtensions(entries []DiscoveredExtension, includeDisabled bool) []DiscoveredExtension {
	if len(entries) == 0 {
		return nil
	}
	if includeDisabled {
		return append([]DiscoveredExtension(nil), entries...)
	}

	filtered := make([]DiscoveredExtension, 0, len(entries))
	for i := range entries {
		if entries[i].Enabled {
			filtered = append(filtered, entries[i])
		}
	}
	return filtered
}

func sourceRank(source Source) int {
	switch source {
	case SourceBundled:
		return 0
	case SourceUser:
		return 1
	case SourceWorkspace:
		return 2
	default:
		return -1
	}
}

func manifestPathForDirectory(dir string) string {
	tomlPath := filepath.Join(dir, ManifestFileNameTOML)
	if _, err := os.Stat(tomlPath); err == nil {
		return tomlPath
	}
	return filepath.Join(dir, ManifestFileNameJSON)
}

func manifestPathForFSDirectory(root fs.FS, dir string, virtualDir string) string {
	tomlPath := path.Join(dir, ManifestFileNameTOML)
	if _, err := fs.Stat(root, tomlPath); err == nil {
		return path.Join(virtualDir, ManifestFileNameTOML)
	}
	return path.Join(virtualDir, ManifestFileNameJSON)
}

func logDiscoveryFailure(source Source, extensionDir, manifestPath string, err error) DiscoveryFailure {
	failure := DiscoveryFailure{
		Source:       source,
		ExtensionDir: extensionDir,
		ManifestPath: manifestPath,
		Err:          err,
	}

	slog.Error(
		"extension discovery failed",
		slog.String("source", string(source)),
		slog.String("extension_dir", extensionDir),
		slog.String("manifest_path", manifestPath),
		slog.String("error", err.Error()),
	)

	return failure
}

func loadManifestFromFS(ctx context.Context, root fs.FS, dir string) (*Manifest, string, error) {
	if err := contextError(ctx, "load extension manifest"); err != nil {
		return nil, "", err
	}
	if root == nil {
		return nil, "", fmt.Errorf("load extension manifest: filesystem is nil")
	}

	resolvedDir := strings.Trim(strings.TrimSpace(dir), "/")
	if resolvedDir == "" {
		return nil, "", fmt.Errorf("load extension manifest: directory is empty")
	}

	tomlPath := path.Join(resolvedDir, ManifestFileNameTOML)
	jsonPath := path.Join(resolvedDir, ManifestFileNameJSON)

	if _, err := fs.Stat(root, tomlPath); err == nil {
		manifest, loadErr := loadManifestFileFromFS(ctx, root, tomlPath, manifestFormatTOML)
		if loadErr != nil {
			return nil, "", loadErr
		}
		if _, err := fs.Stat(root, jsonPath); err == nil {
			slog.Warn(
				"extension.toml takes precedence over extension.json",
				slog.String("dir", resolvedDir),
				slog.String("manifest_path", tomlPath),
				slog.String("ignored_manifest_path", jsonPath),
			)
		}
		return manifest, tomlPath, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("stat extension manifest %q: %w", tomlPath, err)
	}

	if _, err := fs.Stat(root, jsonPath); err == nil {
		manifest, loadErr := loadManifestFileFromFS(ctx, root, jsonPath, manifestFormatJSON)
		return manifest, jsonPath, loadErr
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("stat extension manifest %q: %w", jsonPath, err)
	}

	return nil, "", &ManifestNotFoundError{
		Dir:            resolvedDir,
		CandidatePaths: []string{tomlPath, jsonPath},
	}
}

func loadManifestFileFromFS(
	ctx context.Context,
	root fs.FS,
	filePath string,
	format ManifestFormat,
) (*Manifest, error) {
	if err := contextError(ctx, "load extension manifest file"); err != nil {
		return nil, err
	}

	data, err := fs.ReadFile(root, filePath)
	if err != nil {
		return nil, fmt.Errorf("read extension manifest %q: %w", filePath, err)
	}

	raw, err := decodeRawManifest(data, format)
	if err != nil {
		return nil, &ManifestDecodeError{Path: filePath, Format: format, Err: err}
	}
	if err := raw.validatePresence(); err != nil {
		return nil, &ManifestValidationError{Path: filePath, Err: err}
	}

	manifest := raw.toManifest()
	if err := ValidateManifest(ctx, manifest); err != nil {
		return nil, &ManifestValidationError{Path: filePath, Err: err}
	}

	return manifest, nil
}
