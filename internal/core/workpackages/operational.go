package workpackages

import (
	"context"
	"fmt"
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
	packageDir, err := resolvePackageDirectory(initiativeDir, Package{
		ID:        ref.PackageID,
		Directory: "_packages/" + ref.PackageID,
	})
	if err != nil {
		return OperationalPaths{}, err
	}
	return OperationalPaths{Ref: ref, InitiativeDir: initiativeDir, PackageDir: packageDir}, nil
}
