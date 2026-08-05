package extensionpkg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

type installRuntimeDependencyParents struct {
	source      *installSourceDirectory
	target      *fileutil.Directory
	sourceScope *installSourceEntry
	targetScope *fileutil.Directory
	scopeName   string
}

func copyInstallRuntimeDependency(
	sourceModules *installSourceDirectory,
	targetModules *fileutil.Directory,
	packageName string,
	active installDirectoryStack,
) (err error) {
	parts, err := installNodeModuleParts(packageName)
	if err != nil {
		return err
	}
	parents, err := openInstallRuntimeDependencyParents(sourceModules, targetModules, parts)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, parents.Close()) }()

	entry, err := parents.source.OpenEntry(parts[len(parts)-1])
	if err != nil {
		if entry.symlink {
			return fmt.Errorf("extension: reject runtime dependency symlink %q: %w", packageName, err)
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("extension: runtime dependency %q is missing: %w", packageName, err)
		}
		return fmt.Errorf("extension: open runtime dependency %q: %w", packageName, err)
	}
	defer func() {
		if closeErr := entry.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if entry.directory != nil {
		next, pushErr := active.Push(entry.info, packageName)
		if pushErr != nil {
			return pushErr
		}
		entry.directory.Rebase()
		targetPackage, createErr := parents.target.CreateDirectory(parts[len(parts)-1], entry.info.Mode().Perm())
		if createErr != nil {
			return fmt.Errorf("extension: create runtime dependency target %q: %w", packageName, createErr)
		}
		copyErr := copyInstallDirectoryContents(entry.directory, targetPackage, next)
		syncErr := targetPackage.Sync()
		closeErr := targetPackage.Close()
		if copyErr != nil || syncErr != nil || closeErr != nil {
			return errors.Join(copyErr, syncErr, closeErr)
		}
		return nil
	}
	return copyInstallFile(entry.file, parents.target, parts[len(parts)-1], entry.info.Mode().Perm())
}

func openInstallRuntimeDependencyParents(
	source *installSourceDirectory,
	target *fileutil.Directory,
	parts []string,
) (*installRuntimeDependencyParents, error) {
	parents := &installRuntimeDependencyParents{source: source, target: target}
	if len(parts) != 2 {
		return parents, nil
	}
	parents.scopeName = parts[0]
	scopeEntry, err := source.OpenEntry(parents.scopeName)
	if err != nil {
		return nil, fmt.Errorf("extension: open runtime dependency scope %q: %w", parents.scopeName, err)
	}
	parents.sourceScope = &scopeEntry
	if scopeEntry.directory == nil {
		return nil, errors.Join(
			errors.New("extension: runtime dependency scope is not a directory"),
			parents.Close(),
		)
	}
	parents.targetScope, err = target.OpenDirectoryForMutation(parents.scopeName)
	if errors.Is(err, os.ErrNotExist) {
		parents.targetScope, err = target.CreateDirectory(parents.scopeName, scopeEntry.info.Mode().Perm())
	}
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("extension: open or create runtime dependency scope %q: %w", parents.scopeName, err),
			parents.Close(),
		)
	}
	parents.source = scopeEntry.directory
	parents.target = parents.targetScope
	return parents, nil
}

func (p *installRuntimeDependencyParents) Close() error {
	var err error
	if p.targetScope != nil {
		if closeErr := p.targetScope.Close(); closeErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("extension: close runtime dependency scope %q: %w", p.scopeName, closeErr),
			)
		}
		p.targetScope = nil
	}
	err = errors.Join(err, p.sourceScope.Close())
	return err
}

func copyInstallEntry(
	source *installSourceDirectory,
	target *fileutil.Directory,
	name string,
	active installDirectoryStack,
) (err error) {
	entry, err := source.OpenEntry(name)
	if err != nil {
		if entry.symlink {
			return fmt.Errorf("extension: reject source symlink %q: %w", name, err)
		}
		return err
	}
	defer func() {
		if closeErr := entry.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if entry.directory != nil {
		next, err := active.Push(entry.info, name)
		if err != nil {
			return err
		}
		targetDirectory, err := target.CreateDirectory(name, entry.info.Mode().Perm())
		if err != nil {
			return fmt.Errorf("extension: create target directory %q: %w", name, err)
		}
		copyErr := copyInstallDirectoryContents(entry.directory, targetDirectory, next)
		syncErr := targetDirectory.Sync()
		closeErr := targetDirectory.Close()
		if copyErr != nil || syncErr != nil || closeErr != nil {
			return errors.Join(copyErr, syncErr, closeErr)
		}
		return nil
	}
	return copyInstallFile(entry.file, target, name, entry.info.Mode().Perm())
}

func copyInstallFile(
	source *os.File,
	target *fileutil.Directory,
	name string,
	perm os.FileMode,
) (err error) {
	if source == nil {
		return fmt.Errorf("extension: source file %q is required", name)
	}
	targetFile, err := target.CreateRegularFile(name, perm)
	if err != nil {
		return fmt.Errorf("extension: create target file %q: %w", name, err)
	}
	cleanup := true
	closed := false
	defer func() {
		if !closed {
			if closeErr := targetFile.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("extension: close target file %q: %w", name, closeErr))
			}
		}
		if cleanup {
			if removeErr := target.RemoveRegularFile(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("extension: remove incomplete target file %q: %w", name, removeErr))
			}
		}
	}()
	if _, err := io.Copy(targetFile, source); err != nil {
		return fmt.Errorf("extension: copy target file %q: %w", name, err)
	}
	if err := targetFile.Chmod(perm); err != nil {
		return fmt.Errorf("extension: set target file mode %q: %w", name, err)
	}
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("extension: sync target file %q: %w", name, err)
	}
	closeErr := targetFile.Close()
	closed = true
	if closeErr != nil {
		return fmt.Errorf("extension: close target file %q: %w", name, closeErr)
	}
	cleanup = false
	return nil
}

func installNodeModuleParts(packageName string) ([]string, error) {
	name := strings.TrimSpace(packageName)
	if name == "" || strings.Contains(name, `\`) {
		return nil, fmt.Errorf("extension: invalid runtime dependency name %q", packageName)
	}
	parts := strings.Split(name, "/")
	switch {
	case len(parts) == 1 && validInstallPackageSegment(parts[0], false):
		return parts, nil
	case len(parts) == 2 && strings.HasPrefix(parts[0], "@") &&
		validInstallPackageSegment(parts[0], true) && validInstallPackageSegment(parts[1], false):
		return parts, nil
	default:
		return nil, fmt.Errorf("extension: invalid runtime dependency name %q", packageName)
	}
}

func validInstallPackageSegment(segment string, scoped bool) bool {
	if scoped {
		return len(segment) > 1 && segment != "." && segment != ".." && !strings.Contains(segment, "/") &&
			!strings.Contains(segment, `\`)
	}
	return segment != "" && segment != "." && segment != ".." && !strings.Contains(segment, "/") &&
		!strings.Contains(segment, `\`)
}
