package registry

import (
	"archive/tar"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func cleanArchiveEntryPath(entry string) (string, error) {
	if hasWindowsVolumeName(entry) {
		return "", fmt.Errorf("%w: %q", ErrArchiveEntryMustBeRelative, entry)
	}
	if strings.Contains(entry, "\\") {
		return "", fmt.Errorf("%w: archive names must use portable separators", fileutil.ErrInvalidPath)
	}
	cleaned := path.Clean(entry)
	filesystemPath := filepath.FromSlash(cleaned)
	isAbsolute := strings.HasPrefix(cleaned, "/") ||
		filepath.IsAbs(filesystemPath) ||
		hasWindowsVolumeName(cleaned) ||
		filepath.VolumeName(filesystemPath) != ""
	switch {
	case cleaned == "." || cleaned == "":
		return "", ErrArchiveEntryPathRequired
	case isAbsolute:
		return "", fmt.Errorf("%w: %q", ErrArchiveEntryMustBeRelative, entry)
	case cleaned == ".." || strings.HasPrefix(cleaned, "../"):
		return "", fmt.Errorf("%w: %q", ErrArchiveEntryEscapesRoot, entry)
	default:
		for component := range strings.SplitSeq(cleaned, "/") {
			if err := fileutil.ValidatePathComponent(component); err != nil {
				return "", err
			}
		}
		return cleaned, nil
	}
}

func hasWindowsVolumeName(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	return (value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')
}

func archiveDirMode(header *tar.Header) os.FileMode {
	return archiveEntryMode(header, 0o755)
}

func archiveFileMode(header *tar.Header) os.FileMode {
	return archiveEntryMode(header, 0o644)
}

func archiveEntryMode(header *tar.Header, fallback os.FileMode) os.FileMode {
	if header == nil || header.Mode < 0 || header.Mode > math.MaxUint32 {
		return fallback
	}
	mode := os.FileMode(uint32(header.Mode)) & os.ModePerm
	if mode == 0 {
		return fallback
	}
	return mode
}

func cleanExtractionRelativePath(entryName string) (string, error) {
	if entryName == "" {
		return "", ErrArchiveEntryPathRequired
	}
	cleaned := filepath.Clean(entryName)
	if filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		return "", fmt.Errorf("%w: %q", ErrArchiveEntryMustBeRelative, entryName)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrArchiveEntryEscapesRoot, entryName)
	}
	return cleaned, nil
}
