package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	appDiagnosticBundleManifestSchemaVersion       = 1
	appDiagnosticBundleKind                        = "compozyos_desktop_diagnostics"
	appDiagnosticBundleManifestFile                = "manifest.json"
	appDiagnosticBundleMaxBytes              int64 = 64 * 1024
)

type appDiagnosticBundle struct {
	Path     string                      `json:"path"`
	Bytes    int64                       `json:"bytes"`
	Manifest appDiagnosticBundleManifest `json:"manifest"`
}

type appDiagnosticBundleManifest struct {
	SchemaVersion int                 `json:"schema_version"`
	Kind          string              `json:"kind"`
	GeneratedAt   string              `json:"generated_at"`
	Report        appDiagnosticReport `json:"report"`
}

type appDiagnosticBundleExportResult struct {
	BundlePath string `json:"bundle_path"`
	Bytes      int64  `json:"bytes"`
}

func createAppDiagnosticBundle(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	report appDiagnosticReport,
	outputPath string,
	live bool,
	canonicalDefaultParent bool,
) (appDiagnosticBundle, error) {
	if !live {
		return writeLocalAppDiagnosticBundle(
			homePaths,
			report,
			outputPath,
			deps.now(),
			canonicalDefaultParent,
		)
	}
	result, err := deps.callAppControl(
		ctx,
		filepath.Join(homePaths.HomeDir, "app.sock"),
		appExportDiagnosticsMethod,
		map[string]any{
			"consent":     true,
			"output_path": outputPath,
		},
	)
	if err != nil {
		return appDiagnosticBundle{}, err
	}
	bundle, err := decodeAppDiagnosticBundleExport(result)
	if err != nil {
		return appDiagnosticBundle{}, err
	}
	if filepath.Clean(bundle.Path) != filepath.Clean(outputPath) {
		return appDiagnosticBundle{}, errors.New("app diagnose: app control returned an unexpected bundle path")
	}
	return bundle, nil
}

func resolveAppDiagnosticBundlePath(
	homePaths compozyconfig.HomePaths,
	rawPath string,
	now time.Time,
) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return filepath.Join(
			homePaths.HomeDir,
			supportBundlesDirName,
			appDiagnosticBundleFileName(now),
		), nil
	}
	path, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("app diagnose: resolve bundle output path: %w", err)
	}
	if filepath.Ext(path) != ".gz" {
		return "", errors.New("app diagnose: --bundle-output must end in .gz")
	}
	if filepath.Base(path) == "." || filepath.Base(path) == string(filepath.Separator) {
		return "", errors.New("app diagnose: --bundle-output must be a file path")
	}
	return path, nil
}

func appDiagnosticBundleFileName(now time.Time) string {
	utc := now.UTC()
	return "compozyos-desktop-diagnostics-" + utc.Format("20060102T150405") +
		fmt.Sprintf("%03dZ.tar.gz", utc.Nanosecond()/int(time.Millisecond))
}

func decodeAppDiagnosticBundleExport(value any) (appDiagnosticBundle, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: encode app control bundle export: %w", err)
	}
	var exported appDiagnosticBundleExportResult
	if err := json.Unmarshal(raw, &exported); err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: decode app control bundle export: %w", err)
	}
	if strings.TrimSpace(exported.BundlePath) == "" || exported.Bytes < 0 {
		return appDiagnosticBundle{}, errors.New("app diagnose: app control returned an invalid bundle export")
	}
	manifest, err := readAppDiagnosticBundleManifest(exported.BundlePath)
	if err != nil {
		return appDiagnosticBundle{}, err
	}
	return appDiagnosticBundle{
		Path:     exported.BundlePath,
		Bytes:    exported.Bytes,
		Manifest: manifest,
	}, nil
}

func writeLocalAppDiagnosticBundle(
	homePaths compozyconfig.HomePaths,
	report appDiagnosticReport,
	path string,
	now time.Time,
	canonicalDefaultParent bool,
) (bundle appDiagnosticBundle, err error) {
	manifest, manifestBytes, err := newAppDiagnosticBundleManifest(report, now)
	if err != nil {
		return appDiagnosticBundle{}, err
	}
	if int64(len(manifestBytes)) > appDiagnosticBundleMaxBytes {
		return appDiagnosticBundle{}, errors.New("app diagnose: diagnostic bundle exceeds its size limit")
	}
	if err := ensureAppDiagnosticBundleDestination(path, canonicalDefaultParent); err != nil {
		return appDiagnosticBundle{}, err
	}
	entries, err := collectAppDiagnosticBundleEntries(homePaths, report, manifestBytes)
	if err != nil {
		return appDiagnosticBundle{}, err
	}

	file, temporaryPath, err := createAppDiagnosticBundleTemporaryFile(path)
	if err != nil {
		return appDiagnosticBundle{}, err
	}
	committed := false
	linked := false
	defer func() {
		cleanupErr := cleanupAppDiagnosticBundleWrite(file, temporaryPath, path, linked, committed)
		if cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				err = errors.Join(err, cleanupErr)
			}
		}
	}()
	if err := writeAppDiagnosticBundleArchive(file, entries); err != nil {
		return appDiagnosticBundle{}, err
	}
	if err := file.Sync(); err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: sync bundle temporary file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: inspect bundle temporary file: %w", err)
	}
	if info.Size() > appDiagnosticBundleMaxBytes {
		return appDiagnosticBundle{}, errors.New("app diagnose: diagnostic bundle exceeds its size limit")
	}
	if err := file.Close(); err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: close bundle temporary file: %w", err)
	}
	file = nil
	if err := linkAppDiagnosticBundle(temporaryPath, path); err != nil {
		return appDiagnosticBundle{}, err
	}
	linked = true
	if err := syncAppDiagnosticBundleDirectory(filepath.Dir(path)); err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: sync linked bundle output directory: %w", err)
	}
	committed = true
	if err := os.Remove(temporaryPath); err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: remove linked bundle temporary file: %w", err)
	}
	if err := syncAppDiagnosticBundleDirectory(filepath.Dir(path)); err != nil {
		return appDiagnosticBundle{}, fmt.Errorf("app diagnose: sync bundle output directory: %w", err)
	}
	return appDiagnosticBundle{Path: path, Bytes: info.Size(), Manifest: manifest}, nil
}

func createAppDiagnosticBundleTemporaryFile(path string) (*os.File, string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return nil, "", fmt.Errorf("app diagnose: create bundle temporary file: %w", err)
	}
	temporaryPath := file.Name()
	if err := file.Chmod(0o600); err != nil {
		return nil, "", errors.Join(
			fmt.Errorf("app diagnose: set bundle temporary file permissions: %w", err),
			closeAppDiagnosticBundleTemporaryFile(file),
			removeAppDiagnosticBundleTemporaryFile(temporaryPath),
		)
	}
	return file, temporaryPath, nil
}

func cleanupAppDiagnosticBundleWrite(
	file *os.File,
	temporaryPath string,
	path string,
	linked bool,
	committed bool,
) error {
	if committed {
		return nil
	}
	cleanupErr := closeAppDiagnosticBundleTemporaryFile(file)
	if !committed && linked {
		cleanupErr = errors.Join(cleanupErr, removeLinkedAppDiagnosticBundle(temporaryPath, path))
	}
	return errors.Join(cleanupErr, removeAppDiagnosticBundleTemporaryFile(temporaryPath))
}

func closeAppDiagnosticBundleTemporaryFile(file *os.File) error {
	if file == nil {
		return nil
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("app diagnose: close bundle temporary file: %w", err)
	}
	return nil
}

func removeLinkedAppDiagnosticBundle(temporaryPath string, path string) error {
	temporaryInfo, err := os.Lstat(temporaryPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("app diagnose: inspect linked bundle temporary file: %w", err)
	}
	bundleInfo, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("app diagnose: inspect uncommitted bundle: %w", err)
	}
	if !os.SameFile(temporaryInfo, bundleInfo) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("app diagnose: remove uncommitted bundle: %w", err)
	}
	return nil
}

func removeAppDiagnosticBundleTemporaryFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("app diagnose: remove bundle temporary file: %w", err)
	}
	return nil
}

func linkAppDiagnosticBundle(temporaryPath string, path string) error {
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("app diagnose: bundle output path already exists")
		}
		return fmt.Errorf("app diagnose: create bundle destination: %w", err)
	}
	return nil
}

func newAppDiagnosticBundleManifest(
	report appDiagnosticReport,
	now time.Time,
) (appDiagnosticBundleManifest, []byte, error) {
	reportBytes, err := json.Marshal(report)
	if err != nil {
		return appDiagnosticBundleManifest{}, nil, fmt.Errorf("app diagnose: encode report for bundle: %w", err)
	}
	validatedReport, err := validateAppDiagnosticReport(reportBytes)
	if err != nil {
		return appDiagnosticBundleManifest{}, nil, err
	}
	manifest := appDiagnosticBundleManifest{
		SchemaVersion: appDiagnosticBundleManifestSchemaVersion,
		Kind:          appDiagnosticBundleKind,
		GeneratedAt:   now.UTC().Format(time.RFC3339Nano),
		Report:        validatedReport,
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return appDiagnosticBundleManifest{}, nil, fmt.Errorf("app diagnose: encode bundle manifest: %w", err)
	}
	return manifest, encoded, nil
}

func ensureAppDiagnosticBundleDestination(path string, canonicalDefaultParent bool) error {
	parent := filepath.Dir(path)
	if err := validateAppDiagnosticBundleOutputAncestry(parent); err != nil {
		return err
	}
	parentInfo, err := os.Lstat(parent)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return fmt.Errorf("app diagnose: create bundle output directory: %w", err)
		}
		parentInfo, err = os.Lstat(parent)
		if err != nil {
			return fmt.Errorf("app diagnose: inspect created bundle output directory: %w", err)
		}
		if err := validateAppDiagnosticBundleOutputAncestry(parent); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("app diagnose: inspect bundle output directory: %w", err)
	}
	if parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return errors.New("app diagnose: bundle output directory is invalid")
	}
	if canonicalDefaultParent {
		if err := os.Chmod(parent, 0o700); err != nil {
			return fmt.Errorf("app diagnose: set default bundle directory permissions: %w", err)
		}
	}
	return rejectExistingAppDiagnosticBundle(path)
}

func validateAppDiagnosticBundleOutputAncestry(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("app diagnose: resolve bundle output directory ancestry: %w", err)
	}
	volume := filepath.VolumeName(absolutePath)
	root := volume + string(filepath.Separator)
	relativePath := strings.TrimPrefix(absolutePath, root)
	current := root
	for part := range strings.SplitSeq(relativePath, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("app diagnose: inspect bundle output directory ancestry: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 && !isAppDiagnosticBundleSystemAlias(current) {
			return errors.New("app diagnose: bundle output directory ancestry must not contain symlinks")
		}
	}
	return nil
}

func rejectExistingAppDiagnosticBundle(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("app diagnose: inspect bundle output path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("app diagnose: bundle output path must not be a symlink")
	}
	return errors.New("app diagnose: bundle output path already exists")
}
