package daytona

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func extractTar(root string, src io.Reader) (stats tarStats, err error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return tarStats{}, fmt.Errorf("sandbox/daytona: create extract root %q: %w", root, err)
	}
	extractRoot, err := os.OpenRoot(root)
	if err != nil {
		return tarStats{}, fmt.Errorf("sandbox/daytona: confine extract root %q: %w", root, err)
	}
	defer func() {
		if closeErr := extractRoot.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("sandbox/daytona: close extract root %q: %w", root, closeErr))
		}
	}()

	reader := tar.NewReader(src)
	for {
		header, readErr := reader.Next()
		if errors.Is(readErr, io.EOF) {
			return stats, nil
		}
		if readErr != nil {
			return tarStats{}, fmt.Errorf("sandbox/daytona: read tar header: %w", readErr)
		}
		if isArchiveRootMarker(header.Name) {
			continue
		}
		entryStats, extractErr := extractTarEntry(extractRoot, header, reader)
		if extractErr != nil {
			return tarStats{}, extractErr
		}
		stats.Files += entryStats.Files
		stats.Bytes += entryStats.Bytes
	}
}

func extractTarEntry(root *os.Root, header *tar.Header, reader io.Reader) (tarStats, error) {
	name, err := safeArchiveName(header.Name)
	if err != nil {
		return tarStats{}, err
	}
	name = filepath.FromSlash(name)

	switch header.Typeflag {
	case tar.TypeDir:
		return tarStats{}, extractTarDirectory(root, name, header)
	case tar.TypeReg:
		written, err := extractTarRegularFile(root, name, header, reader)
		if err != nil {
			return tarStats{}, err
		}
		return tarStats{Files: 1, Bytes: written}, nil
	case tar.TypeSymlink:
		return tarStats{}, extractTarSymlink(root, name, header)
	default:
		return tarStats{}, fmt.Errorf(
			"sandbox/daytona: unsupported tar entry %q mode %v",
			header.Name,
			header.Typeflag,
		)
	}
}

func extractTarDirectory(root *os.Root, name string, header *tar.Header) error {
	if err := root.MkdirAll(name, modePerm(header.FileInfo().Mode(), 0o755)); err != nil {
		return fmt.Errorf("sandbox/daytona: create extracted directory %q: %w", name, err)
	}
	return nil
}

func extractTarRegularFile(
	root *os.Root,
	name string,
	header *tar.Header,
	reader io.Reader,
) (written int64, err error) {
	if err := ensureTarParent(root, name); err != nil {
		return 0, err
	}
	if info, statErr := root.Lstat(name); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("%w: refusing to overwrite symlink %q", errUnsafeTarPath, name)
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return 0, fmt.Errorf("sandbox/daytona: inspect extracted file %q: %w", name, statErr)
	}

	file, err := root.OpenFile(
		name,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		modePerm(header.FileInfo().Mode(), 0o600),
	)
	if err != nil {
		return 0, fmt.Errorf("sandbox/daytona: create extracted file %q: %w", name, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("sandbox/daytona: close extracted file %q: %w", name, closeErr))
		}
	}()

	written, err = io.CopyN(file, reader, header.Size)
	if err != nil {
		return 0, fmt.Errorf("sandbox/daytona: write extracted file %q: %w", name, err)
	}
	return written, nil
}

func extractTarSymlink(root *os.Root, name string, header *tar.Header) error {
	if err := ensureTarParent(root, name); err != nil {
		return err
	}
	linkTarget, err := safeSymlinkTarget(name, header.Linkname)
	if err != nil {
		return err
	}
	if err := root.RemoveAll(name); err != nil {
		return fmt.Errorf("sandbox/daytona: replace extracted symlink %q: %w", name, err)
	}
	if err := root.Symlink(linkTarget, name); err != nil {
		return fmt.Errorf("sandbox/daytona: create extracted symlink %q: %w", name, err)
	}
	return nil
}

func ensureTarParent(root *os.Root, name string) error {
	parent := filepath.Dir(name)
	if err := root.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("sandbox/daytona: create extracted parent %q: %w", parent, err)
	}
	return nil
}

func safeSymlinkTarget(name string, linkName string) (string, error) {
	if strings.TrimSpace(linkName) == "" {
		return "", fmt.Errorf("%w: empty symlink target", errUnsafeTarPath)
	}
	if filepath.IsAbs(linkName) {
		return "", fmt.Errorf("%w: absolute symlink target %q", errUnsafeTarPath, linkName)
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(name), linkName))
	if resolved == ".." || strings.HasPrefix(resolved, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: symlink %q escapes extract root", errUnsafeTarPath, linkName)
	}
	return linkName, nil
}
