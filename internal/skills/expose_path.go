package skills

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/fileutil"
	"golang.org/x/text/unicode/norm"
)

const maxExposureNameBytes = 255

var exposureNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validateExposeName(name string) error {
	trimmed := strings.TrimSpace(name)
	decoded, decodeErr := url.PathUnescape(trimmed)
	unsafe := decodeErr != nil || trimmed == "" || trimmed != name || decoded != trimmed ||
		!utf8.ValidString(trimmed) || norm.NFC.String(trimmed) != trimmed || norm.NFKC.String(trimmed) != trimmed ||
		len([]byte(trimmed)) > maxExposureNameBytes || filepath.IsAbs(trimmed) ||
		trimmed == "." || trimmed == ".." || strings.ContainsAny(trimmed, `/\`) ||
		strings.ContainsRune(trimmed, '\x00') || !exposureNamePattern.MatchString(trimmed)
	if unsafe {
		return newExposureError(
			ExposureCodeUnsafeSkillName,
			"",
			"",
			fmt.Sprintf("skill name %q is unsafe as a filesystem path segment", name),
			decodeErr,
		)
	}
	return nil
}

func resolveExposeDest(root string, name string) (string, error) {
	if err := validateExposeName(name); err != nil {
		return "", err
	}
	absoluteRoot, err := filepath.Abs(filepath.Clean(strings.TrimSpace(root)))
	if err != nil {
		return "", fmt.Errorf("skills: make exposure root %q absolute: %w", root, err)
	}
	if unsafeRootSymlink(absoluteRoot) {
		return "", newExposureError(
			ExposureCodeNameConflict,
			"",
			absoluteRoot,
			fmt.Sprintf("exposure root %q traverses a symlinked parent", absoluteRoot),
			fileutil.ErrPathOutsideRoot,
		)
	}
	canonicalRoot, err := fileutil.CanonicalPathWithExistingPrefix(absoluteRoot)
	if err != nil {
		return "", fmt.Errorf("skills: resolve exposure root %q: %w", absoluteRoot, err)
	}
	// Canonicalize the parent, not the final destination. A healthy exposure is
	// itself a symlink that normally resolves outside the provider root; following
	// that final component would incorrectly reject idempotent re-expose.
	canonicalParent, err := fileutil.CanonicalPathWithExistingPrefix(filepath.Dir(filepath.Join(absoluteRoot, name)))
	if err != nil {
		return "", fmt.Errorf("skills: resolve exposure destination parent for %q: %w", name, err)
	}
	candidate := filepath.Join(canonicalParent, name)
	contained, err := fileutil.PathWithinRoot(canonicalRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("skills: relate exposure destination %q to root %q: %w", candidate, canonicalRoot, err)
	}
	if !contained {
		return "", newExposureError(
			ExposureCodeNameConflict,
			"",
			candidate,
			fmt.Sprintf("exposure destination %q escapes preset root %q", candidate, canonicalRoot),
			fileutil.ErrPathOutsideRoot,
		)
	}
	return filepath.Clean(candidate), nil
}

func unsafeRootSymlink(root string) bool {
	volume := filepath.VolumeName(root)
	relative := strings.TrimPrefix(root, volume)
	parts := strings.FieldsFunc(relative, func(r rune) bool { return r == filepath.Separator })
	current := volume + string(filepath.Separator)
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		if err != nil {
			return true
		}
		if info.Mode()&os.ModeSymlink != 0 && !allowedDarwinSystemAlias(current) {
			return true
		}
	}
	return false
}

func allowedDarwinSystemAlias(path string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	cleaned := filepath.Clean(path)
	return cleaned == "/var" || cleaned == "/tmp" || cleaned == "/etc"
}
