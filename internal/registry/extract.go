package registry

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	// DefaultMaxDecompressedSize caps the raw decompressed TAR stream.
	DefaultMaxDecompressedSize int64 = 500 * 1024 * 1024
	// DefaultMaxFileCount caps the number of filesystem nodes materialized.
	DefaultMaxFileCount = 10000
	// DefaultMaxArchiveDepth caps the number of path components in one entry.
	DefaultMaxArchiveDepth = 64
)

var (
	// ErrArchiveRootRequired reports that ExtractArchive received no acquired extraction root.
	ErrArchiveRootRequired = errors.New("archive extraction root is required")
	// ErrArchiveEntryPathRequired reports that an archive entry did not contain a usable path.
	ErrArchiveEntryPathRequired = errors.New("archive entry path is required")
	// ErrArchiveEntryMustBeRelative reports that an archive entry path was absolute.
	ErrArchiveEntryMustBeRelative = errors.New("archive entry must be relative")
	// ErrArchiveEntryEscapesRoot reports that an archive entry would escape the extraction root.
	ErrArchiveEntryEscapesRoot = errors.New("archive entry escapes the extraction root")
	// ErrUnsupportedArchiveEntryType reports an unsupported tar entry type.
	ErrUnsupportedArchiveEntryType = errors.New("unsupported archive entry type")
	// ErrArchiveDuplicateEntry reports that an archive declared the same path more than once.
	ErrArchiveDuplicateEntry = errors.New("archive contains duplicate entry")
	// ErrPathRootRequired reports that PathWithinRoot received a blank root path.
	ErrPathRootRequired = errors.New("root path is required")
	// ErrPathOutsideRoot reports that a path resolves outside the provided root.
	ErrPathOutsideRoot = errors.New("path must stay within the root directory")
	// ErrPathTraversesSymlink reports a symlink or reparse point in the staging tree.
	ErrPathTraversesSymlink = errors.New("path traverses symlink")

	errArchiveTooLarge     = errors.New("registry: archive exceeds max decompressed size")
	errArchiveTooManyFiles = errors.New("registry: archive exceeds max file count")
	errArchiveTooDeep      = errors.New("registry: archive entry exceeds max depth")
)

type extractLimits struct {
	maxDecompressedSize int64
	maxFileCount        int
	maxDepth            int
}

func (l extractLimits) normalized() extractLimits {
	if l.maxDecompressedSize <= 0 {
		l.maxDecompressedSize = DefaultMaxDecompressedSize
	}
	if l.maxFileCount <= 0 {
		l.maxFileCount = DefaultMaxFileCount
	}
	if l.maxDepth <= 0 {
		l.maxDepth = DefaultMaxArchiveDepth
	}
	return l
}

func extractArchive(reader io.Reader, root *fileutil.Directory, limits extractLimits) (err error) {
	if root == nil {
		return ErrArchiveRootRequired
	}
	limits = limits.normalized()

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close gzip stream: %w", closeErr))
		}
	}()

	decompressed := newDecompressedArchiveReader(gzipReader, limits.maxDecompressedSize)
	tarReader := tar.NewReader(decompressed)
	seenEntries := make(map[string]struct{})
	treeBudget := newArchiveTreeBudget(limits.maxFileCount, limits.maxDepth)
	for {
		header, readErr := tarReader.Next()
		if errors.Is(readErr, io.EOF) {
			if drainErr := drainDecompressedArchive(decompressed); drainErr != nil {
				return drainErr
			}
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read tar entry: %w", readErr)
		}

		entryName, cleanErr := cleanArchiveEntryPath(header.Name)
		if cleanErr != nil {
			return fmt.Errorf("clean archive entry %q: %w", header.Name, cleanErr)
		}
		if _, exists := seenEntries[entryName]; exists {
			return fmt.Errorf("%w: %q", ErrArchiveDuplicateEntry, header.Name)
		}
		seenEntries[entryName] = struct{}{}
		if typeErr := validateArchiveEntryType(header.Typeflag); typeErr != nil {
			return fmt.Errorf("extract archive entry %q: %w", header.Name, typeErr)
		}
		if budgetErr := treeBudget.reserve(entryName); budgetErr != nil {
			return fmt.Errorf("reserve archive entry %q: %w", header.Name, budgetErr)
		}

		if extractErr := extractArchiveEntry(tarReader, root, entryName, header); extractErr != nil {
			return fmt.Errorf("extract archive entry %q: %w", header.Name, extractErr)
		}
	}
}

func extractArchiveEntry(
	tarReader *tar.Reader,
	root *fileutil.Directory,
	entryName string,
	header *tar.Header,
) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return extractArchiveDirectory(root, entryName, header)
	case tar.TypeReg, 0:
		return extractArchiveFile(tarReader, root, entryName, header)
	default:
		return fmt.Errorf("%w %d", ErrUnsupportedArchiveEntryType, header.Typeflag)
	}
}

func validateArchiveEntryType(typeflag byte) error {
	switch typeflag {
	case tar.TypeDir, tar.TypeReg, 0:
		return nil
	default:
		return fmt.Errorf("%w %d", ErrUnsupportedArchiveEntryType, typeflag)
	}
}

func extractArchiveDirectory(root *fileutil.Directory, entryName string, header *tar.Header) (err error) {
	directory, err := openExtractionDirectory(root, entryName, true, archiveDirMode(header))
	if err != nil {
		return fmt.Errorf("open archive directory %q: %w", entryName, err)
	}
	defer func() {
		err = closeOwnedExtractionDirectory(root, directory, err)
	}()
	if err := directory.Chmod(archiveDirMode(header)); err != nil {
		return fmt.Errorf("set archive directory mode %q: %w", entryName, err)
	}
	return nil
}

func extractArchiveFile(
	tarReader *tar.Reader,
	root *fileutil.Directory,
	entryName string,
	header *tar.Header,
) (err error) {
	parent, name, err := openExtractionParent(root, entryName, true, 0o755)
	if err != nil {
		return fmt.Errorf("open archive parent %q: %w", entryName, err)
	}
	defer func() {
		err = closeOwnedExtractionDirectory(root, parent, err)
	}()

	fileMode := archiveFileMode(header)
	file, err := parent.CreateRegularFile(name, fileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return classifyExistingArchiveEntry(parent, name)
		}
		return fmt.Errorf("create archive file %q: %w", entryName, mapExtractionAccessError(err))
	}

	if _, err := io.Copy(file, tarReader); err != nil {
		return cleanupArchiveFile(file, parent, name, fmt.Errorf("write archive file %q: %w", entryName, err), false)
	}
	if err := file.Chmod(fileMode); err != nil {
		return cleanupArchiveFile(file, parent, name, fmt.Errorf("set archive file mode %q: %w", entryName, err), false)
	}
	if err := file.Close(); err != nil {
		return cleanupArchiveFile(nil, parent, name, fmt.Errorf("close archive file %q: %w", entryName, err), true)
	}
	return nil
}

func classifyExistingArchiveEntry(parent *fileutil.Directory, name string) error {
	file, err := parent.OpenRegularFile(name)
	if err != nil {
		return fmt.Errorf("%w: %q", mapExtractionAccessError(err), name)
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("close existing archive file %q: %w", name, closeErr)
	}
	return fmt.Errorf("%w: %q", ErrArchiveDuplicateEntry, name)
}

func cleanupArchiveFile(
	file *os.File,
	parent *fileutil.Directory,
	name string,
	baseErr error,
	alreadyClosed bool,
) error {
	if file != nil && !alreadyClosed {
		if closeErr := file.Close(); closeErr != nil {
			baseErr = errors.Join(baseErr, fmt.Errorf("close archive file %q after failure: %w", name, closeErr))
		}
	}
	if parent == nil {
		return errors.Join(baseErr, ErrArchiveRootRequired)
	}
	if removeErr := parent.RemoveRegularFile(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		baseErr = errors.Join(baseErr, fmt.Errorf("remove partial archive file %q: %w", name, removeErr))
	}
	return baseErr
}
