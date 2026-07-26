package loop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

const (
	// DefinitionFileName is the canonical filename inside one loop-definition directory.
	DefinitionFileName = "loop.yaml"
)

var (
	// ErrDefinitionExists reports a write/fork that would overwrite an existing loop definition.
	ErrDefinitionExists = errors.New("loop definition already exists")
	// ErrDefinitionNotFound reports a missing loop definition.
	ErrDefinitionNotFound = errors.New("loop definition not found")
)

// WriteDefinitionOptions configures a writable loop definition write.
type WriteDefinitionOptions struct {
	Source    Source
	Overwrite bool
	Linter    Linter
}

// IsDefinitionFileName reports whether name can hold a loop definition document.
func IsDefinitionFileName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "loop.yaml", "loop.yml":
		return true
	default:
		return false
	}
}

// WriteDefinition validates and writes one loop definition under root/<meta.name>/loop.yaml.
func WriteDefinition(root string, data []byte, opts WriteDefinitionOptions) (ResourceSpec, string, error) {
	if strings.TrimSpace(root) == "" {
		return ResourceSpec{}, "", errors.New("loop: writable root is required")
	}
	source := opts.Source.Normalize()
	if source == "" {
		source = SourceUser
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ResourceSpec{}, "", fmt.Errorf("loop: resolve root %q: %w", root, err)
	}
	spec, _, err := projectResource(data, ResourceParseOptions{
		Source: source,
		Linter: opts.Linter,
	})
	if err != nil {
		return ResourceSpec{}, "", err
	}

	loopDir, err := safeLoopDir(rootAbs, spec.Name)
	if err != nil {
		return ResourceSpec{}, "", err
	}
	definitionPath := filepath.Join(loopDir, DefinitionFileName)
	spec.Dir = loopDir
	spec.FilePath = definitionPath
	if _, err := validateResourceSpec(
		context.Background(),
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		spec,
	); err != nil {
		return ResourceSpec{}, "", err
	}

	parentDir := filepath.Dir(loopDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return ResourceSpec{}, "", fmt.Errorf("loop: create parent directory %q: %w", parentDir, err)
	}
	if err := os.Mkdir(loopDir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			if !opts.Overwrite {
				return ResourceSpec{}, "", fmt.Errorf("%w: %s", ErrDefinitionExists, spec.Name)
			}
		} else {
			return ResourceSpec{}, "", fmt.Errorf("loop: create definition directory %q: %w", loopDir, err)
		}
	}
	if err := os.MkdirAll(loopDir, 0o755); err != nil {
		return ResourceSpec{}, "", fmt.Errorf("loop: create definition directory %q: %w", loopDir, err)
	}
	if err := os.WriteFile(definitionPath, data, 0o600); err != nil {
		return ResourceSpec{}, "", fmt.Errorf("loop: write definition %q: %w", definitionPath, err)
	}
	return spec, definitionPath, nil
}

// ForkDefinitionFile copies a file-backed loop definition directory into a writable root.
func ForkDefinitionFile(sourceFilePath string, targetRoot string) (_ string, err error) {
	sourcePath := strings.TrimSpace(sourceFilePath)
	if sourcePath == "" {
		return "", errors.New("loop: source definition path is required")
	}
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", fmt.Errorf("loop: resolve source definition %q: %w", sourceFilePath, err)
	}
	if strings.TrimSpace(targetRoot) == "" {
		return "", errors.New("loop: fork target root is required")
	}
	targetRootAbs, err := filepath.Abs(targetRoot)
	if err != nil {
		return "", fmt.Errorf("loop: resolve target root %q: %w", targetRoot, err)
	}
	data, err := os.ReadFile(sourceAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s", ErrDefinitionNotFound, sourceAbs)
		}
		return "", fmt.Errorf("loop: read source definition %q: %w", sourceAbs, err)
	}
	spec, _, err := projectResource(data, ResourceParseOptions{
		Source:   SourceUser,
		Dir:      filepath.Dir(sourceAbs),
		FilePath: sourceAbs,
	})
	if err != nil {
		return "", err
	}
	targetDir, err := safeLoopDir(targetRootAbs, spec.Name)
	if err != nil {
		return "", err
	}
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", fmt.Errorf("loop: create fork parent %q: %w", parentDir, err)
	}
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("%w: %s", ErrDefinitionExists, spec.Name)
		}
		return "", fmt.Errorf("loop: create fork target %q: %w", targetDir, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			err = errors.Join(err, os.RemoveAll(targetDir))
		}
	}()

	if err := copyDefinitionDir(filepath.Dir(sourceAbs), targetDir); err != nil {
		return "", fmt.Errorf("loop: fork definition %q: %w", spec.Name, err)
	}
	cleanup = false
	return filepath.Join(targetDir, DefinitionFileName), nil
}

func copyDefinitionDir(sourceDir string, targetDir string) (err error) {
	sourceRoot, err := os.OpenRoot(sourceDir)
	if err != nil {
		return fmt.Errorf("loop: open source root %q: %w", sourceDir, err)
	}
	defer func() {
		if closeErr := sourceRoot.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("loop: close source root %q: %w", sourceDir, closeErr))
		}
	}()
	targetRoot, err := os.OpenRoot(targetDir)
	if err != nil {
		return fmt.Errorf("loop: open fork target root %q: %w", targetDir, err)
	}
	defer func() {
		if closeErr := targetRoot.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("loop: close fork target root %q: %w", targetDir, closeErr))
		}
	}()
	return fs.WalkDir(sourceRoot.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		return copyDefinitionEntry(sourceRoot, targetRoot, path, entry, walkErr)
	})
}

func copyDefinitionEntry(
	sourceRoot *os.Root,
	targetRoot *os.Root,
	path string,
	entry fs.DirEntry,
	walkErr error,
) error {
	if walkErr != nil {
		return walkErr
	}
	if path == "." {
		return nil
	}
	rel := filepath.FromSlash(path)
	if entry.IsDir() {
		return targetRoot.MkdirAll(rel, 0o755)
	}
	if entry.Type()&fs.ModeSymlink != 0 {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := sourceRoot.ReadFile(rel)
	if err != nil {
		return err
	}
	if parent := filepath.Dir(rel); parent != "." {
		if err := targetRoot.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	return targetRoot.WriteFile(rel, data, 0o600)
}

// DeleteWritableDefinition deletes a user/workspace loop definition directory.
func DeleteWritableDefinition(root string, name string) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("loop: writable root is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("loop: resolve root %q: %w", root, err)
	}
	loopDir, err := safeLoopDir(rootAbs, name)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(loopDir); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrDefinitionNotFound, name)
		}
		return fmt.Errorf("loop: inspect definition %q: %w", loopDir, statErr)
	}
	if err := os.RemoveAll(loopDir); err != nil {
		return fmt.Errorf("loop: delete definition %q: %w", loopDir, err)
	}
	return nil
}

func validateLoopName(name string) (string, error) {
	return ValidateName(name)
}

func safeLoopDir(root string, name string) (string, error) {
	cleanName, err := validateLoopName(name)
	if err != nil {
		return "", err
	}
	return safeForkPath(root, cleanName)
}

func safeForkPath(root string, rel string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("loop: root is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("loop: relative path is required: %q", rel)
	}
	rootClean := filepath.Clean(root)
	target := filepath.Clean(filepath.Join(rootClean, rel))
	within, err := filepath.Rel(rootClean, target)
	if err != nil {
		return "", fmt.Errorf("loop: resolve path %q: %w", target, err)
	}
	if within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("loop: path escapes root: %q", rel)
	}
	return target, nil
}
