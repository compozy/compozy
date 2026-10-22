package workspacedb

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// legacyBlobLocation resolves a journal blob inside the real journal root and names its kind.
func legacyBlobLocation(sourceRoot, path string) (root, relative, kind string, err error) {
	root, err = filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return "", "", "", err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", "", err
	}
	relative, err = filepath.Rel(root, realPath)
	if err != nil || !filepath.IsLocal(relative) {
		return "", "", "", fmt.Errorf("legacy blob outside journal root: %q", path)
	}
	kind, _, ok := strings.Cut(filepath.ToSlash(relative), "/")
	if !ok || (kind != "terminal-recordings" && kind != "terminal-artifacts") {
		return "", "", "", fmt.Errorf("unexpected legacy blob kind: %q", path)
	}
	return root, relative, kind, nil
}

// legacyBlobDigest is the hex SHA-256 a journal row declares for its blob.
func legacyBlobDigest(digest string) (string, error) {
	expected := strings.TrimPrefix(digest, "sha256:")
	if len(expected) != 64 {
		return "", errors.New("invalid legacy blob digest")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return "", err
	}
	return expected, nil
}

func copyLegacyBlob(sourceRoot, targetRoot, path, digest string) (destinationPath string, resultErr error) {
	sourceRoot, relative, kind, err := legacyBlobLocation(sourceRoot, path)
	if err != nil {
		return "", err
	}
	source, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, source.Close()) }()
	input, err := source.Open(relative)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	info, err := input.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("legacy blob is not regular: %q", path)
	}
	// Content-addressed destination avoids overwriting any preexisting User blob.
	expected, err := legacyBlobDigest(digest)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(targetRoot, kind)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	output, err := os.CreateTemp(directory, ".legacy-*")
	if err != nil {
		return "", err
	}
	tempPath := output.Name()
	defer func() {
		if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(output, hash), input)
	if err := errors.Join(copyErr, output.Sync(), output.Close()); err != nil {
		return "", err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return "", errors.New("legacy blob digest mismatch")
	}
	destination := filepath.Join(directory, "legacy-"+expected)
	if existing, err := os.Open(destination); err == nil {
		existingHash := sha256.New()
		_, readErr := io.Copy(existingHash, existing)
		if err := errors.Join(readErr, existing.Close()); err != nil {
			return "", err
		}
		if hex.EncodeToString(existingHash.Sum(nil)) != expected {
			return "", errors.New("existing imported blob digest mismatch")
		}
		return destination, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return "", err
	}
	return destination, nil
}
