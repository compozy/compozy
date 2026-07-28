package extensionpkg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"
)

func copyInstallRuntimeDependency(
	sourceRoot string,
	sourcePath string,
	targetPath string,
	activeDirs map[string]struct{},
) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("extension: stat runtime dependency %q: %w", sourcePath, err)
	}

	switch {
	case info.IsDir():
		return copyInstallPackageRoot(sourcePath, sourcePath, targetPath, activeDirs)
	case info.Mode()&os.ModeSymlink != 0:
		resolvedPath, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			return fmt.Errorf("extension: resolve runtime dependency symlink %q: %w", sourcePath, err)
		}
		if err := ensureInstallPathWithinRoot(sourceRoot, resolvedPath); err != nil {
			return fmt.Errorf("extension: reject runtime dependency symlink %q: %w", sourcePath, err)
		}
		resolvedInfo, err := os.Stat(resolvedPath)
		if err != nil {
			return fmt.Errorf("extension: stat runtime dependency target %q: %w", resolvedPath, err)
		}
		switch {
		case resolvedInfo.IsDir():
			return copyInstallPackageRoot(sourcePath, resolvedPath, targetPath, activeDirs)
		case resolvedInfo.Mode().IsRegular():
			return copyInstallFile(resolvedPath, targetPath, resolvedInfo.Mode().Perm())
		default:
			return fmt.Errorf("extension: unsupported runtime dependency target type for %q", sourcePath)
		}
	case info.Mode().IsRegular():
		return copyInstallFile(sourcePath, targetPath, info.Mode().Perm())
	default:
		return fmt.Errorf("extension: unsupported runtime dependency type for %q", sourcePath)
	}
}

func copyInstallPackageRoot(
	sourcePath string,
	sourceDir string,
	targetDir string,
	activeDirs map[string]struct{},
) error {
	absSourceDir, err := filepath.Abs(strings.TrimSpace(sourceDir))
	if err != nil {
		return fmt.Errorf("extension: resolve package root %q: %w", sourceDir, err)
	}
	canonicalSourceRoot, err := canonicalizeInstallPath(absSourceDir)
	if err != nil {
		return fmt.Errorf("extension: canonicalize package root %q: %w", absSourceDir, err)
	}

	info, err := os.Stat(absSourceDir)
	if err != nil {
		return fmt.Errorf("extension: stat package root %q: %w", absSourceDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("extension: package root %q is not a directory", absSourceDir)
	}

	nextActiveDirs, err := pushInstallCopyDir(activeDirs, absSourceDir, sourcePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, info.Mode().Perm()); err != nil {
		return fmt.Errorf("extension: create package target directory %q: %w", targetDir, err)
	}
	if err := os.Chmod(targetDir, info.Mode().Perm()); err != nil {
		return fmt.Errorf("extension: set package target directory mode %q: %w", targetDir, err)
	}

	return copyInstallDirectoryContents(canonicalSourceRoot, absSourceDir, targetDir, nextActiveDirs)
}

func copyInstallEntry(sourceRoot string, sourcePath string, targetPath string, activeDirs map[string]struct{}) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("extension: stat source path %q: %w", sourcePath, err)
	}

	switch {
	case info.IsDir():
		nextActiveDirs, err := pushInstallCopyDir(activeDirs, sourcePath, sourcePath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
			return fmt.Errorf("extension: create target directory %q: %w", targetPath, err)
		}
		if err := os.Chmod(targetPath, info.Mode().Perm()); err != nil {
			return fmt.Errorf("extension: set target directory mode %q: %w", targetPath, err)
		}
		return copyInstallDirectoryContents(sourceRoot, sourcePath, targetPath, nextActiveDirs)
	case info.Mode().IsRegular():
		return copyInstallFile(sourcePath, targetPath, info.Mode().Perm())
	case info.Mode()&os.ModeSymlink != 0:
		return copyInstallSymlink(sourceRoot, sourcePath, targetPath, activeDirs)
	default:
		return fmt.Errorf("extension: unsupported file type in extension payload %q", sourcePath)
	}
}

func copyInstallFile(sourcePath string, targetPath string, perm os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("extension: create target file parent %q: %w", filepath.Dir(targetPath), err)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("extension: open source file %q: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("extension: close source file %q: %w", sourcePath, closeErr)
		}
	}()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("extension: create target file %q: %w", targetPath, err)
	}
	defer func() {
		if closeErr := target.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("extension: close target file %q: %w", targetPath, closeErr)
		}
	}()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("extension: copy file %q to %q: %w", sourcePath, targetPath, err)
	}
	if err := target.Chmod(perm); err != nil {
		return fmt.Errorf("extension: set target file mode %q: %w", targetPath, err)
	}

	return nil
}

func copyInstallSymlink(sourceRoot string, sourcePath string, targetPath string, activeDirs map[string]struct{}) error {
	resolvedPath, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return fmt.Errorf("extension: resolve source symlink %q: %w", sourcePath, err)
	}
	if err := ensureInstallPathWithinRoot(sourceRoot, resolvedPath); err != nil {
		return fmt.Errorf("extension: reject source symlink %q: %w", sourcePath, err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return fmt.Errorf("extension: stat resolved symlink target %q: %w", resolvedPath, err)
	}

	switch {
	case info.IsDir():
		nextActiveDirs, err := pushInstallCopyDir(activeDirs, resolvedPath, sourcePath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
			return fmt.Errorf(
				"extension: create target directory %q for symlinked source %q: %w",
				targetPath,
				sourcePath,
				err,
			)
		}
		if err := os.Chmod(targetPath, info.Mode().Perm()); err != nil {
			return fmt.Errorf(
				"extension: set target directory mode %q for symlinked source %q: %w",
				targetPath,
				sourcePath,
				err,
			)
		}
		return copyInstallDirectoryContents(sourceRoot, resolvedPath, targetPath, nextActiveDirs)
	case info.Mode().IsRegular():
		return copyInstallFile(resolvedPath, targetPath, info.Mode().Perm())
	default:
		return fmt.Errorf("extension: unsupported symlink target type for %q", sourcePath)
	}
}
