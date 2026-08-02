package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

type installDirectoryStack []os.FileInfo

func copyInstallTree(sourceDir string, targetDir string) error {
	return copyInstallTreeWithHooks(sourceDir, targetDir, installCopyHooks{})
}

func copyInstallTreeWithHooks(sourcePath string, targetPath string, hooks installCopyHooks) (err error) {
	source, sourceInfo, err := openInstallSourceTree(sourcePath, hooks)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if strings.TrimSpace(targetPath) == "" {
		return errors.New("extension: target directory is required")
	}
	target, err := fileutil.OpenOrCreateDirectory(targetPath, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("extension: open staging target %q: %w", targetPath, err)
	}
	defer func() {
		if closeErr := target.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close staging target %q: %w", targetPath, closeErr))
		}
	}()
	if err := target.Chmod(sourceInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("extension: set staging target mode %q: %w", targetPath, err)
	}
	if err := copyInstallDirectoryContents(source, target, installDirectoryStack{sourceInfo}); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return fmt.Errorf("extension: sync staging target %q: %w", targetPath, err)
	}
	return nil
}

func copyInstallDirectoryContents(
	source *installSourceDirectory,
	target *fileutil.Directory,
	active installDirectoryStack,
) error {
	runtimeDeps, hasPackageManifest, err := loadInstallRuntimeDependencies(source)
	if err != nil {
		return err
	}

	names, err := source.ReadDir()
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		if hasPackageManifest && name == "node_modules" {
			if err := copyInstallNodeModules(source, target, active, runtimeDeps); err != nil {
				return err
			}
			continue
		}
		if err := copyInstallEntry(source, target, name, active); err != nil {
			return err
		}
	}
	return nil
}

func loadInstallRuntimeDependencies(
	source *installSourceDirectory,
) (runtimeDeps map[string]struct{}, hasManifest bool, err error) {
	entry, err := source.OpenEntry("package.json")
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("extension: open package manifest: %w", err)
	}
	if entry.file == nil {
		if closeErr := entry.Close(); closeErr != nil {
			return nil, false, errors.Join(errors.New("extension: package manifest is a directory"), closeErr)
		}
		return nil, false, errors.New("extension: package manifest is a directory")
	}
	defer func() {
		if closeErr := entry.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close package manifest: %w", closeErr))
		}
	}()

	raw, err := io.ReadAll(entry.file)
	if err != nil {
		return nil, false, fmt.Errorf("extension: read package manifest: %w", err)
	}
	var manifest installPackageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, false, fmt.Errorf("extension: decode package manifest: %w", err)
	}

	runtimeDeps = make(map[string]struct{}, len(manifest.Dependencies)+len(manifest.OptionalDependencies))
	for name := range manifest.Dependencies {
		if normalized := strings.TrimSpace(name); normalized != "" {
			runtimeDeps[normalized] = struct{}{}
		}
	}
	for name := range manifest.OptionalDependencies {
		if normalized := strings.TrimSpace(name); normalized != "" {
			runtimeDeps[normalized] = struct{}{}
		}
	}
	return runtimeDeps, true, nil
}

func copyInstallNodeModules(
	source *installSourceDirectory,
	target *fileutil.Directory,
	active installDirectoryStack,
	runtimeDeps map[string]struct{},
) (err error) {
	if len(runtimeDeps) == 0 {
		return nil
	}
	entry, err := source.OpenEntry("node_modules")
	if err != nil {
		return fmt.Errorf("extension: open runtime dependency directory: %w", err)
	}
	if entry.directory == nil {
		return errors.Join(errors.New("extension: node_modules is not a directory"), entry.Close())
	}
	defer func() {
		if closeErr := entry.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	targetModules, err := target.CreateDirectory("node_modules", entry.info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("extension: create target node_modules: %w", err)
	}
	defer func() {
		if closeErr := targetModules.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close target node_modules: %w", closeErr))
		}
	}()

	names := make([]string, 0, len(runtimeDeps))
	for name := range runtimeDeps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := copyInstallRuntimeDependency(entry.directory, targetModules, name, active); err != nil {
			return err
		}
	}
	return targetModules.Sync()
}

func (s installDirectoryStack) Push(info os.FileInfo, sourcePath string) (installDirectoryStack, error) {
	for _, active := range s {
		if os.SameFile(active, info) {
			return nil, fmt.Errorf("extension: symlink directory cycle detected from %q", sourcePath)
		}
	}
	next := append(installDirectoryStack(nil), s...)
	return append(next, info), nil
}

func (e *installSourceEntry) Close() error {
	if e == nil {
		return nil
	}
	var err error
	if e.directory != nil {
		err = errors.Join(err, e.directory.Close())
		e.directory = nil
	}
	if e.file != nil {
		if closeErr := e.file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close source file: %w", closeErr))
		}
		e.file = nil
	}
	return err
}
