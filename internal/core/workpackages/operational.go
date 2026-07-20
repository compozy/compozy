package workpackages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OperationalPaths identifies a package-owned directory without trusting the
// mutable Work Package plan. It is used only to preserve final-review evidence
// if the plan becomes unreadable before completion can be recorded.
type OperationalPaths struct {
	Ref           Ref
	InitiativeDir string
	PackageDir    string
}

// ResolveOperationalPaths resolves a contained package directory from a
// syntactically valid public reference. Completion still re-resolves the full
// Target before it can mutate the canonical plan.
func ResolveOperationalPaths(ctx context.Context, workspaceRoot, reference string) (OperationalPaths, error) {
	if err := context.Cause(ctx); err != nil {
		return OperationalPaths{}, fmt.Errorf("resolve work package operational paths: %w", err)
	}
	ref, err := ParsePackageRef(reference)
	if err != nil {
		return OperationalPaths{}, err
	}
	tasksRoot, err := canonicalTasksRoot(workspaceRoot)
	if err != nil {
		return OperationalPaths{}, err
	}
	initiativeDir, err := resolveInitiative(tasksRoot, ref.Initiative)
	if err != nil {
		return OperationalPaths{}, err
	}
	packageDir, err := resolveOperationalPackageDirectory(initiativeDir, ref.PackageID)
	if err != nil {
		return OperationalPaths{}, err
	}
	return OperationalPaths{Ref: ref, InitiativeDir: initiativeDir, PackageDir: packageDir}, nil
}

func resolveOperationalPackageDirectory(initiativeDir, packageID string) (string, error) {
	packagesRoot := filepath.Join(initiativeDir, "_packages")
	entries, err := os.ReadDir(packagesRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read work package directory: %w", err)
		}
		return resolvePackageDirectory(initiativeDir, Package{
			ID:        packageID,
			Directory: "_packages/" + packageID,
		})
	}

	candidates := make([]string, 0, 1)
	for _, entry := range entries {
		directory := "_packages/" + entry.Name()
		if validPackageDirectory(packageID, directory) {
			candidates = append(candidates, directory)
		}
	}
	switch len(candidates) {
	case 0:
		return resolvePackageDirectory(initiativeDir, Package{
			ID:        packageID,
			Directory: "_packages/" + packageID,
		})
	case 1:
		return resolvePackageDirectory(initiativeDir, Package{ID: packageID, Directory: candidates[0]})
	default:
		return "", newError(
			ErrInvalidPlan,
			"",
			packageID,
			"",
			[]Issue{{
				Path:    packagesRoot,
				Field:   "package_directory",
				Message: "multiple directories match package " + packageID + ": " + strings.Join(candidates, ", "),
			}},
		)
	}
}
