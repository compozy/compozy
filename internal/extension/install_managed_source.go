package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxInstallSymlinkResolutions = 255

type installCopyHooks struct {
	afterSymlinkResolve func()
}

type installSourceTree struct {
	root          *os.Root
	canonicalPath string
	hooks         installCopyHooks
}

type installSourceDirectory struct {
	tree         *installSourceTree
	root         *os.Root
	relativePath string
}

type installSourceEntry struct {
	directory *installSourceDirectory
	file      *os.File
	info      os.FileInfo
	symlink   bool
}

func openInstallSourceTree(path string, hooks installCopyHooks) (*installSourceDirectory, os.FileInfo, error) {
	sourcePath := strings.TrimSpace(path)
	if sourcePath == "" {
		return nil, nil, errors.New("extension: source directory is required")
	}
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, nil, fmt.Errorf("extension: resolve source directory %q: %w", path, err)
	}
	root, err := os.OpenRoot(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("extension: open source root %q: %w", absPath, err)
	}
	info, err := root.Stat(".")
	if err != nil {
		return nil, nil, closeInstallRootOnError(
			root,
			fmt.Errorf("extension: stat source root %q: %w", absPath, err),
		)
	}
	if !info.IsDir() {
		return nil, nil, closeInstallRootOnError(
			root,
			fmt.Errorf("extension: source root %q is not a directory", absPath),
		)
	}
	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, nil, closeInstallRootOnError(
			root,
			fmt.Errorf("extension: canonicalize source root %q: %w", absPath, err),
		)
	}
	canonicalInfo, err := os.Stat(canonicalPath)
	if err != nil {
		return nil, nil, closeInstallRootOnError(
			root,
			fmt.Errorf("extension: stat canonical source root %q: %w", canonicalPath, err),
		)
	}
	if !os.SameFile(info, canonicalInfo) {
		return nil, nil, closeInstallRootOnError(
			root,
			errors.New("extension: source root changed while it was acquired"),
		)
	}
	tree := &installSourceTree{root: root, canonicalPath: canonicalPath, hooks: hooks}
	return &installSourceDirectory{tree: tree, root: root, relativePath: "."}, info, nil
}

func (d *installSourceDirectory) Close() error {
	if d == nil || d.root == nil {
		return nil
	}
	root := d.root
	d.root = nil
	if err := root.Close(); err != nil {
		return fmt.Errorf("extension: close source directory: %w", err)
	}
	return nil
}

func (d *installSourceDirectory) ReadDir() (_ []string, err error) {
	if d == nil || d.root == nil {
		return nil, errors.New("extension: source directory is closed")
	}
	file, err := d.root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("extension: open source directory for enumeration: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close enumerated source directory: %w", closeErr))
		}
	}()
	entries, err := file.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("extension: enumerate source directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func (d *installSourceDirectory) OpenEntry(name string) (installSourceEntry, error) {
	if d == nil || d.root == nil || d.tree == nil || d.tree.root == nil {
		return installSourceEntry{}, errors.New("extension: source directory is closed")
	}
	if err := validateManagedInstallEntryName(name); err != nil {
		return installSourceEntry{}, err
	}
	info, err := d.root.Lstat(name)
	if err != nil {
		return installSourceEntry{}, fmt.Errorf("extension: inspect source entry %q: %w", name, err)
	}
	entry := installSourceEntry{info: info, symlink: info.Mode()&os.ModeSymlink != 0}
	if entry.symlink {
		return d.openSymlinkEntry(name, entry)
	}
	if info.IsDir() {
		root, openErr := d.root.OpenRoot(name)
		if openErr != nil {
			return entry, fmt.Errorf("extension: open source directory %q: %w", name, openErr)
		}
		directory, handleInfo, openErr := d.directoryFromHandle(root, filepath.Join(d.relativePath, name))
		if openErr != nil {
			return entry, openErr
		}
		entry.directory = directory
		entry.info = handleInfo
		return entry, nil
	}
	file, openErr := d.root.Open(name)
	if openErr != nil {
		return entry, fmt.Errorf("extension: open source file %q: %w", name, openErr)
	}
	handleInfo, openErr := validateInstallSourceFile(file, name)
	if openErr != nil {
		return entry, openErr
	}
	entry.file = file
	entry.info = handleInfo
	return entry, nil
}

func (d *installSourceDirectory) openSymlinkEntry(
	name string,
	entry installSourceEntry,
) (installSourceEntry, error) {
	linkPath := filepath.Join(d.relativePath, name)
	resolvedPath, err := d.tree.resolvePath(linkPath)
	if err != nil {
		return entry, fmt.Errorf("extension: resolve source symlink %q: %w", name, err)
	}
	if d.tree.hooks.afterSymlinkResolve != nil {
		d.tree.hooks.afterSymlinkResolve()
	}
	root, directoryErr := d.tree.root.OpenRoot(resolvedPath)
	if directoryErr == nil {
		directory, info, err := d.directoryFromHandle(root, resolvedPath)
		if err != nil {
			return entry, err
		}
		entry.directory = directory
		entry.info = info
		return entry, nil
	}
	file, fileErr := d.tree.root.Open(resolvedPath)
	if fileErr != nil {
		return entry, fmt.Errorf(
			"extension: open resolved source symlink %q: %w",
			name,
			errors.Join(directoryErr, fileErr),
		)
	}
	info, err := validateInstallSourceFile(file, resolvedPath)
	if err != nil {
		return entry, err
	}
	entry.file = file
	entry.info = info
	return entry, nil
}

func (d *installSourceDirectory) directoryFromHandle(
	root *os.Root,
	relativePath string,
) (*installSourceDirectory, os.FileInfo, error) {
	info, err := root.Stat(".")
	if err != nil {
		return nil, nil, closeInstallRootOnError(
			root,
			fmt.Errorf("extension: stat source directory handle: %w", err),
		)
	}
	if !info.IsDir() {
		return nil, nil, closeInstallRootOnError(
			root,
			errors.New("extension: acquired source entry is not a directory"),
		)
	}
	return &installSourceDirectory{tree: d.tree, root: root, relativePath: relativePath}, info, nil
}

func (d *installSourceDirectory) Rebase() {
	if d == nil || d.tree == nil {
		return
	}
	d.tree = &installSourceTree{
		root:          d.root,
		canonicalPath: filepath.Join(d.tree.canonicalPath, d.relativePath),
		hooks:         d.tree.hooks,
	}
	d.relativePath = "."
}

func (t *installSourceTree) resolvePath(path string) (string, error) {
	candidate, err := normalizeInstallSourcePath(path)
	if err != nil {
		return "", err
	}
	for range maxInstallSymlinkResolutions {
		components := splitInstallSourcePath(candidate)
		current := "."
		resolved := true
		for index, component := range components {
			next := filepath.Join(current, component)
			info, statErr := t.root.Lstat(next)
			if statErr != nil {
				return "", fmt.Errorf("inspect source path %q: %w", next, statErr)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				current = next
				continue
			}
			target, readErr := t.root.Readlink(next)
			if readErr != nil {
				return "", fmt.Errorf("read source symlink %q: %w", next, readErr)
			}
			candidate, err = t.resolveLinkTarget(next, target, components[index+1:])
			if err != nil {
				return "", err
			}
			resolved = false
			break
		}
		if resolved {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"source symlink chain from %q exceeds %d resolutions",
		path,
		maxInstallSymlinkResolutions,
	)
}

func (t *installSourceTree) resolveLinkTarget(
	linkPath string,
	target string,
	remainder []string,
) (string, error) {
	var candidate string
	if filepath.IsAbs(target) {
		canonicalTarget, err := filepath.EvalSymlinks(filepath.Clean(target))
		if err != nil {
			return "", fmt.Errorf("canonicalize absolute symlink target %q: %w", target, err)
		}
		relative, err := filepath.Rel(t.canonicalPath, canonicalTarget)
		if err != nil {
			return "", fmt.Errorf(
				"relate symlink target %q to source root %q: %w",
				target,
				t.canonicalPath,
				err,
			)
		}
		candidate = relative
	} else {
		candidate = filepath.Join(filepath.Dir(linkPath), target)
	}
	if len(remainder) != 0 {
		candidate = filepath.Join(append([]string{candidate}, remainder...)...)
	}
	candidate, err := normalizeInstallSourcePath(candidate)
	if err != nil {
		return "", fmt.Errorf(
			"symlink target %q escapes source root %q: %w",
			target,
			t.canonicalPath,
			err,
		)
	}
	return candidate, nil
}

func normalizeInstallSourcePath(path string) (string, error) {
	clean := filepath.Clean(path)
	escapesRoot := clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))
	if filepath.IsAbs(clean) || escapesRoot {
		return "", errors.New("path escapes source root")
	}
	return clean, nil
}

func splitInstallSourcePath(path string) []string {
	clean := filepath.Clean(path)
	if clean == "." {
		return nil
	}
	return strings.Split(clean, string(filepath.Separator))
}

func validateManagedInstallEntryName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name ||
		strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return fmt.Errorf("extension: invalid source entry name %q", name)
	}
	return nil
}

func validateInstallSourceFile(file *os.File, name string) (os.FileInfo, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, closeInstallFileOnError(
			file,
			fmt.Errorf("extension: stat source file %q: %w", name, err),
		)
	}
	if !info.Mode().IsRegular() {
		return nil, closeInstallFileOnError(
			file,
			fmt.Errorf("extension: source file %q is not regular", name),
		)
	}
	return info, nil
}

func closeInstallRootOnError(root *os.Root, cause error) error {
	if closeErr := root.Close(); closeErr != nil {
		return errors.Join(cause, fmt.Errorf("extension: close source root after failure: %w", closeErr))
	}
	return cause
}

func closeInstallFileOnError(file *os.File, cause error) error {
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(cause, fmt.Errorf("extension: close source file after failure: %w", closeErr))
	}
	return cause
}
