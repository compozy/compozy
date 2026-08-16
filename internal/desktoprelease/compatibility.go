package desktoprelease

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func WriteCompatibility(path string, compatibility Compatibility) error {
	if err := ValidateVersion(compatibility.RuntimeVersion); err != nil {
		return fmt.Errorf("desktop release: runtime_version: %w", err)
	}
	if err := ValidateVersion(compatibility.MinAppVersion); err != nil {
		return fmt.Errorf("desktop release: min_app_version: %w", err)
	}
	if filepath.Base(path) != CompatibilityFile {
		return fmt.Errorf("desktop release: compatibility path must end in %s", CompatibilityFile)
	}
	contents, err := canonicalJSON(compatibility)
	if err != nil {
		return fmt.Errorf("desktop release: encode compatibility: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("desktop release: create compatibility directory: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("desktop release: write compatibility: %w", err)
	}
	return nil
}

func ReadCompatibility(path string) (Compatibility, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Compatibility{}, fmt.Errorf("desktop release: read compatibility: %w", err)
	}
	var compatibility Compatibility
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&compatibility); err != nil {
		return Compatibility{}, fmt.Errorf("desktop release: decode compatibility: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Compatibility{}, fmt.Errorf("desktop release: compatibility contains multiple JSON documents")
		}
		return Compatibility{}, fmt.Errorf("desktop release: decode compatibility trailing data: %w", err)
	}
	if err := ValidateVersion(compatibility.RuntimeVersion); err != nil {
		return Compatibility{}, fmt.Errorf("desktop release: runtime_version: %w", err)
	}
	if err := ValidateVersion(compatibility.MinAppVersion); err != nil {
		return Compatibility{}, fmt.Errorf("desktop release: min_app_version: %w", err)
	}
	return compatibility, nil
}

func VerifyChecksumsCatalog(ctx context.Context, catalogPath string, artifacts []Artifact) error {
	contents, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("desktop release: read checksums catalog: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	entries := make(map[string]string)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("desktop release: checksums canceled: %w", err)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return fmt.Errorf("desktop release: malformed checksums line %q", scanner.Text())
		}
		if _, duplicate := entries[fields[1]]; duplicate {
			return fmt.Errorf("desktop release: duplicate checksum for %s", fields[1])
		}
		if decoded, decodeErr := hex.DecodeString(fields[0]); decodeErr != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("desktop release: malformed checksum for %s", fields[1])
		}
		entries[fields[1]] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("desktop release: scan checksums catalog: %w", err)
	}
	missing := make([]string, 0)
	for _, artifact := range artifacts {
		if entries[artifact.Name] != artifact.SHA256 {
			missing = append(missing, artifact.Name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("desktop release: checksums catalog does not authenticate %v", missing)
	}
	return nil
}
