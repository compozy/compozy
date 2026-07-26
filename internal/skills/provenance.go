package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	provenanceVersionKey = "version"
)

const sidecarFileName = ".agh-meta.json"

// ErrSymlinkEscape reports a skill payload symlink that resolves outside the skill directory.
var ErrSymlinkEscape = errors.New("skills: symlink escapes skill directory")

// HashMismatchError reports a provenance hash mismatch for a marketplace skill.
type HashMismatchError struct {
	ExpectedHash string
	ActualHash   string
}

// Error implements the error interface.
func (e *HashMismatchError) Error() string {
	if e == nil {
		return "skills: provenance hash mismatch"
	}

	return fmt.Sprintf(
		"skills: provenance hash mismatch: expected %s, got %s",
		e.ExpectedHash,
		e.ActualHash,
	)
}

// ComputeHash returns the SHA-256 hex digest for raw SKILL.md content bytes.
func ComputeHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ComputeDirectoryHash returns a deterministic SHA-256 digest for a skill
// directory payload, excluding the provenance sidecar itself.
func ComputeDirectoryHash(skillDir string) (string, error) {
	root := strings.TrimSpace(skillDir)
	if root == "" {
		return "", errors.New("skills: skill directory is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("skills: resolve skill directory %q: %w", skillDir, err)
	}

	entries := make([]string, 0)
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absRoot {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == sidecarFileName {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		entries = append(entries, relPath)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("skills: walk skill directory %q: %w", absRoot, err)
	}

	sort.Strings(entries)
	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	for _, relPath := range entries {
		if err := writeHashEntry(hasher, absRoot, relPath, buffer); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// WriteSidecar writes marketplace provenance metadata alongside a skill's SKILL.md file.
func WriteSidecar(skillDir string, provenance Provenance) error {
	sidecarPath, err := sidecarPath(skillDir)
	if err != nil {
		return err
	}

	payload, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return fmt.Errorf("skills: marshal provenance sidecar %q: %w", sidecarPath, err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(sidecarPath, payload, 0o600); err != nil {
		return fmt.Errorf("skills: write provenance sidecar %q: %w", sidecarPath, err)
	}

	return nil
}

// ReadSidecar reads marketplace provenance metadata from a skill directory.
func ReadSidecar(skillDir string) (*Provenance, error) {
	sidecarPath, err := sidecarPath(skillDir)
	if err != nil {
		return nil, err
	}

	payload, err := os.ReadFile(sidecarPath)
	if err != nil {
		return nil, fmt.Errorf("skills: read provenance sidecar %q: %w", sidecarPath, err)
	}

	var provenance Provenance
	if err := json.Unmarshal(payload, &provenance); err != nil {
		return nil, fmt.Errorf("skills: parse provenance sidecar %q: %w", sidecarPath, err)
	}
	if err := validateSidecarProvenance(sidecarPath, provenance); err != nil {
		return nil, err
	}

	return &provenance, nil
}

// VerifyHash recomputes the installed skill payload hash for a skill directory
// and compares it with the stored provenance hash.
func VerifyHash(skillDir string, provenance *Provenance) error {
	if provenance == nil {
		return errors.New("skills: provenance is required")
	}

	actualHash, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		return err
	}
	if actualHash == provenance.Hash {
		return nil
	}

	return &HashMismatchError{
		ExpectedHash: provenance.Hash,
		ActualHash:   actualHash,
	}
}

func writeHashEntry(hasher hash.Hash, root string, relPath string, buffer []byte) error {
	normalizedPath := filepath.ToSlash(relPath)
	absPath := filepath.Join(root, relPath)

	info, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Errorf("skills: stat hashed path %q: %w", absPath, err)
	}

	if info.Mode().IsRegular() {
		file, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("skills: open hashed path %q: %w", absPath, err)
		}
		defer file.Close()

		if err := writeHashString(
			hasher,
			fmt.Sprintf("file:%s\nmode:%#o\n", normalizedPath, info.Mode().Perm()),
		); err != nil {
			return err
		}
		for {
			n, err := file.Read(buffer)
			if n > 0 {
				if _, writeErr := hasher.Write(buffer[:n]); writeErr != nil {
					return fmt.Errorf("skills: hash regular file %q: %w", absPath, writeErr)
				}
			}
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("skills: hash regular file %q: %w", absPath, err)
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return fmt.Errorf("skills: hash separator for %q: %w", absPath, err)
		}
		return nil
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := ensurePathWithinRoot(root, absPath); err != nil {
			return fmt.Errorf("skills: reject hashed symlink %q: %w", absPath, errors.Join(ErrSymlinkEscape, err))
		}
		target, err := os.Readlink(absPath)
		if err != nil {
			return fmt.Errorf("skills: read hashed symlink %q: %w", absPath, err)
		}

		if err := writeHashString(
			hasher,
			fmt.Sprintf(
				"symlink:%s\nmode:%#o\ntarget:%s\n",
				normalizedPath,
				info.Mode().Perm(),
				filepath.ToSlash(target),
			),
		); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("skills: unsupported file type in skill payload %q", absPath)
}

// HasSidecar reports whether a skill directory contains marketplace provenance metadata.
func HasSidecar(skillDir string) (bool, error) {
	sidecarPath, err := sidecarPath(skillDir)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(sidecarPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("skills: stat provenance sidecar %q: %w", sidecarPath, err)
}

func sidecarPath(skillDir string) (string, error) {
	return resolveSkillPath(skillDir, sidecarFileName)
}

func resolveSkillPath(skillDir string, fileName string) (string, error) {
	root := strings.TrimSpace(skillDir)
	if root == "" {
		return "", errors.New("skills: skill directory is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("skills: resolve skill directory %q: %w", skillDir, err)
	}

	return filepath.Join(absRoot, fileName), nil
}

func validateSidecarProvenance(sidecarPath string, provenance Provenance) error {
	requiredFields := []struct {
		name    string
		missing bool
	}{
		{name: "hash", missing: strings.TrimSpace(provenance.Hash) == ""},
		{name: "registry", missing: strings.TrimSpace(provenance.Registry) == ""},
		{name: "slug", missing: strings.TrimSpace(provenance.Slug) == ""},
		{name: provenanceVersionKey, missing: strings.TrimSpace(provenance.Version) == ""},
		{name: "installed_at", missing: provenance.InstalledAt.IsZero()},
	}

	for _, field := range requiredFields {
		if field.missing {
			return fmt.Errorf("skills: invalid provenance sidecar %q: missing %s", sidecarPath, field.name)
		}
	}

	return nil
}

func writeHashString(hasher hash.Hash, value string) error {
	if _, err := hasher.Write([]byte(value)); err != nil {
		return fmt.Errorf("skills: hash payload metadata: %w", err)
	}
	return nil
}
