package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type buildResourceDeclaration struct {
	label       string
	path        string
	source      string
	destination string
}

func copyDeclaredBuildResources(sourceDir, outputDir, stageDir string, resources ResourcesConfig) error {
	declarations, err := collectBuildResourceDeclarations(sourceDir, outputDir, resources)
	if err != nil {
		return err
	}
	for _, declaration := range declarations {
		if err := rejectBuildResourceSymlinks(declaration.source); err != nil {
			return err
		}
		destination := filepath.Join(stageDir, declaration.destination)
		if _, err := os.Lstat(destination); err == nil {
			return fmt.Errorf(
				"extension: declared %s %q overlaps a loose build artifact",
				declaration.label,
				declaration.path,
			)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("extension: inspect declared resource destination %q: %w", destination, err)
		}
		if err := copyBuildPath(declaration.source, destination); err != nil {
			return fmt.Errorf("extension: copy declared %s %q: %w", declaration.label, declaration.path, err)
		}
	}
	return nil
}

func collectBuildResourceDeclarations(
	sourceDir string,
	outputDir string,
	resources ResourcesConfig,
) ([]buildResourceDeclaration, error) {
	declarations := make([]buildResourceDeclaration, 0)
	appendPaths := func(label string, paths []string) error {
		for _, path := range paths {
			trimmed := strings.TrimSpace(path)
			if trimmed == "" || filepath.IsAbs(trimmed) {
				return fmt.Errorf("%w: %s path %q must be relative", ErrManifestInvalid, label, path)
			}
			cleaned := filepath.Clean(trimmed)
			if cleaned == "." {
				return fmt.Errorf("%w: %s path must name a resource below the package root", ErrManifestInvalid, label)
			}
			resolved, err := resolveResourcePathWithinRoot(sourceDir, cleaned, label)
			if err != nil {
				return err
			}
			if err := validateBuildResourceOutputSeparation(resolved, outputDir, label, cleaned); err != nil {
				return err
			}
			declarations = append(declarations, buildResourceDeclaration{
				label: label, path: filepath.ToSlash(cleaned), source: resolved, destination: cleaned,
			})
		}
		return nil
	}
	for _, group := range staticResourcePathGroups(resources) {
		if err := appendPaths(group.label, group.paths); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(declarations, func(left, right buildResourceDeclaration) int {
		return strings.Compare(left.destination, right.destination)
	})
	for left := range declarations {
		for right := left + 1; right < len(declarations); right++ {
			if pathsOverlap(declarations[left].destination, declarations[right].destination) ||
				pathsOverlap(declarations[left].source, declarations[right].source) {
				return nil, fmt.Errorf(
					"%w: declared resource paths %q and %q overlap",
					ErrManifestInvalid,
					declarations[left].path,
					declarations[right].path,
				)
			}
		}
	}
	return declarations, nil
}

func validateBuildResourceOutputSeparation(resource, output, label, declared string) error {
	canonicalOutput, err := canonicalBuildPath(output)
	if err != nil {
		return fmt.Errorf("extension: canonicalize build output for %s %q: %w", label, declared, err)
	}
	for _, pair := range [][2]string{{resource, canonicalOutput}, {canonicalOutput, resource}} {
		overlaps, err := buildPathContains(pair[0], pair[1])
		if err != nil {
			return fmt.Errorf("extension: compare %s %q with build output: %w", label, declared, err)
		}
		if overlaps {
			return fmt.Errorf("%w: %s %q overlaps the build output", ErrManifestInvalid, label, declared)
		}
	}
	return nil
}

func pathsOverlap(left, right string) bool {
	return pathContains(left, right) || pathContains(right, left)
}

func pathContains(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func rejectBuildResourceSymlinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: declared resource %q contains a symlink", ErrManifestInvalid, path)
		}
		return nil
	})
}
