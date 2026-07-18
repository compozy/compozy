package workpackages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

// Resolver resolves public Work Package references without exposing storage paths.
type Resolver interface {
	Resolve(ctx context.Context, workspaceRoot, reference string) (Target, error)
}

// TargetResolver is the containment-checked Resolver implementation.
type TargetResolver struct{}

// ClassifyTarget classifies one initiative without treating an invalid marker as ordinary.
func (TargetResolver) ClassifyTarget(ctx context.Context, workspaceRoot, initiative string) (TargetMode, error) {
	target, err := (TargetResolver{}).Resolve(ctx, workspaceRoot, initiative)
	if err == nil {
		return target.Mode, nil
	}
	if errors.Is(err, ErrInvalidPlan) {
		return TargetModeInvalidOptIn, nil
	}
	return "", err
}

// Resolve resolves an ordinary initiative or exact initiative/package reference.
func (TargetResolver) Resolve(ctx context.Context, workspaceRoot, reference string) (Target, error) {
	if err := context.Cause(ctx); err != nil {
		return Target{}, fmt.Errorf("resolve work package target: %w", err)
	}
	ref, err := ParseRef(reference)
	if err != nil {
		return Target{}, err
	}
	return resolveRef(workspaceRoot, ref)
}

// ResolvePackage resolves a reference that must name a package.
func (TargetResolver) ResolvePackage(ctx context.Context, workspaceRoot, reference string) (Target, error) {
	if err := context.Cause(ctx); err != nil {
		return Target{}, fmt.Errorf("resolve work package target: %w", err)
	}
	ref, err := ParsePackageRef(reference)
	if err != nil {
		return Target{}, err
	}
	return resolveRef(workspaceRoot, ref)
}

// ParseRef parses either an initiative or an exact package reference.
func ParseRef(reference string, requirePackage ...bool) (Ref, error) {
	if reference == "" || strings.TrimSpace(reference) != reference {
		return Ref{}, newError(
			ErrInvalidReference,
			"",
			"",
			"",
			[]Issue{{Field: "reference", Message: "must not be blank or padded"}},
		)
	}
	segments := strings.Split(reference, "/")
	if len(segments) != 1 && len(segments) != 2 {
		return Ref{}, newError(
			ErrInvalidReference,
			"",
			"",
			"",
			[]Issue{{Field: "reference", Message: "must contain an initiative or initiative/WP-NNN"}},
		)
	}
	if !safeInitiative(segments[0]) {
		return Ref{}, newError(
			ErrInvalidReference,
			"",
			"",
			"",
			[]Issue{{Field: "initiative", Message: "must be one safe task-root component"}},
		)
	}
	ref := Ref{Initiative: segments[0]}
	if len(segments) == 2 {
		if !packageIDPattern.MatchString(segments[1]) {
			return Ref{}, newError(
				ErrInvalidReference,
				ref.Initiative,
				segments[1],
				"",
				[]Issue{{Field: "package_id", Message: "must match WP-NNN"}},
			)
		}
		ref.PackageID = segments[1]
	}
	if len(requirePackage) > 0 && requirePackage[0] && !ref.IsPackage() {
		return Ref{}, newError(
			ErrSelectionRequired,
			ref.Initiative,
			"",
			"",
			[]Issue{{Field: "package_id", Message: "a complete initiative/WP-NNN reference is required"}},
		)
	}
	return ref, nil
}

// ParsePackageRef parses one exact initiative/package reference.
func ParsePackageRef(reference string) (Ref, error) {
	return ParseRef(reference, true)
}

func resolveRef(workspaceRoot string, ref Ref) (Target, error) {
	tasksRoot, err := canonicalTasksRoot(workspaceRoot)
	if err != nil {
		return Target{}, err
	}
	initiativeDir, err := resolveInitiative(tasksRoot, ref.Initiative)
	if err != nil {
		return Target{}, err
	}
	planPath := filepath.Join(initiativeDir, ManifestFileName)
	marker, err := resolveMarker(initiativeDir, planPath)
	if err != nil {
		return Target{}, err
	}
	if !marker.present {
		if ref.IsPackage() {
			return Target{}, newError(ErrPackageNotFound, ref.Initiative, ref.PackageID, planPath, nil)
		}
		return ordinaryTarget(ref, initiativeDir), nil
	}
	content, err := os.ReadFile(marker.path)
	if err != nil {
		return Target{}, newError(
			ErrInvalidPlan,
			ref.Initiative,
			ref.PackageID,
			marker.path,
			[]Issue{{Path: marker.path, Field: "marker", Message: err.Error()}},
		)
	}
	plan, err := ParsePlanForInitiative(string(content), ref.Initiative)
	if err != nil {
		var domainErr *Error
		if errors.As(err, &domainErr) {
			domainErr.PlanPath = marker.path
			if domainErr.Initiative == "" {
				domainErr.Initiative = ref.Initiative
			}
			return Target{}, domainErr
		}
		return Target{}, err
	}
	plan.Path = marker.path
	if !ref.IsPackage() {
		return Target{
			Mode:          TargetModeInitiative,
			Ref:           ref,
			DisplayRef:    ref.String(),
			InitiativeDir: initiativeDir,
			SpecDir:       initiativeDir,
			Plan:          plan,
		}, nil
	}
	pkg, found := plan.Package(ref.PackageID)
	if !found {
		return Target{}, packageNotFound(ref, plan)
	}
	packageDir, err := resolvePackageDirectory(initiativeDir, pkg)
	if err != nil {
		return Target{}, err
	}
	return Target{
		Mode:          TargetModePackage,
		Ref:           ref,
		DisplayRef:    ref.String(),
		InitiativeDir: initiativeDir,
		SpecDir:       initiativeDir,
		PackageDir:    packageDir,
		TasksDir:      packageDir,
		ReviewsDir:    packageDir,
		MemoryDir:     filepath.Join(packageDir, "memory"),
		Plan:          plan,
		Package:       pkg,
	}, nil
}

func ordinaryTarget(ref Ref, initiativeDir string) Target {
	return Target{
		Mode:          TargetModeOrdinary,
		Ref:           ref,
		DisplayRef:    ref.String(),
		InitiativeDir: initiativeDir,
		SpecDir:       initiativeDir,
		PackageDir:    initiativeDir,
		TasksDir:      initiativeDir,
		ReviewsDir:    initiativeDir,
		MemoryDir:     filepath.Join(initiativeDir, "memory"),
	}
}

func canonicalTasksRoot(workspaceRoot string) (string, error) {
	root, err := filepath.Abs(model.TasksBaseDirForWorkspace(workspaceRoot))
	if err != nil {
		return "", fmt.Errorf("resolve workspace task root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newError(
				ErrInitiativeNotFound,
				"",
				"",
				"",
				[]Issue{{Path: root, Field: "workspace", Message: "task root does not exist"}},
			)
		}
		return "", fmt.Errorf("resolve workspace task root: %w", err)
	}
	return resolved, nil
}

func resolveInitiative(tasksRoot, initiative string) (string, error) {
	candidate := filepath.Join(tasksRoot, initiative)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newError(
				ErrInitiativeNotFound,
				initiative,
				"",
				"",
				[]Issue{{Field: "initiative", Message: "does not exist"}},
			)
		}
		return "", fmt.Errorf("resolve initiative %q: %w", initiative, err)
	}
	if err := requireContained(tasksRoot, resolved); err != nil {
		return "", newError(
			ErrContainment,
			initiative,
			"",
			"",
			[]Issue{{Path: candidate, Field: "initiative", Message: err.Error()}},
		)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat initiative %q: %w", initiative, err)
	}
	if !info.IsDir() {
		return "", newError(ErrInitiativeNotFound, initiative, "", "", nil)
	}
	return resolved, nil
}

type markerResolution struct {
	present bool
	path    string
}

func resolveMarker(initiativeDir, path string) (markerResolution, error) {
	_, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return markerResolution{}, nil
		}
		return markerResolution{}, fmt.Errorf("inspect work package marker: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return markerResolution{}, fmt.Errorf("resolve work package marker: %w", err)
	}
	if err := requireContained(initiativeDir, resolved); err != nil {
		return markerResolution{}, newError(
			ErrContainment,
			"",
			"",
			path,
			[]Issue{{Path: path, Field: "marker", Message: err.Error()}},
		)
	}
	return markerResolution{present: true, path: resolved}, nil
}

func resolvePackageDirectory(initiativeDir string, pkg Package) (string, error) {
	packagesRoot := filepath.Join(initiativeDir, "_packages")
	resolvedPackagesRoot, err := filepath.EvalSymlinks(packagesRoot)
	if err != nil {
		// A vanished root is the aggregate form of a missing package directory:
		// classify it as ErrPackageNotFound so aggregate sync degrades every
		// declared package to a Missing placeholder instead of hard-aborting.
		// Any other resolution failure (symlink loop, permission) still fails
		// closed via a wrapped error, and a root that resolves outside the
		// initiative is caught by the containment check below.
		if errors.Is(err, os.ErrNotExist) {
			return "", newError(
				ErrPackageNotFound,
				"",
				pkg.ID,
				"",
				[]Issue{{Path: packagesRoot, Field: "package_directory", Message: "package root does not exist"}},
			)
		}
		return "", fmt.Errorf("resolve package root %q: %w", pkg.ID, err)
	}
	if err := requireContained(initiativeDir, resolvedPackagesRoot); err != nil {
		return "", newError(
			ErrContainment,
			"",
			pkg.ID,
			"",
			[]Issue{{Path: packagesRoot, Field: "package_directory", Message: err.Error()}},
		)
	}
	candidate := filepath.Join(initiativeDir, filepath.FromSlash(pkg.Directory))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newError(
				ErrPackageNotFound,
				"",
				pkg.ID,
				"",
				[]Issue{{Path: candidate, Field: "package_directory", Message: "does not exist"}},
			)
		}
		return "", fmt.Errorf("resolve package directory %q: %w", pkg.ID, err)
	}
	if err := requireContained(resolvedPackagesRoot, resolved); err != nil {
		return "", newError(
			ErrContainment,
			"",
			pkg.ID,
			"",
			[]Issue{{Path: candidate, Field: "package_directory", Message: err.Error()}},
		)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat package directory %q: %w", pkg.ID, err)
	}
	if !info.IsDir() {
		return "", newError(ErrPackageNotFound, "", pkg.ID, "", nil)
	}
	return resolved, nil
}

func requireContained(root, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return fmt.Errorf("compare containment: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("resolved path escapes allowed root")
	}
	return nil
}

func safeInitiative(value string) bool {
	if value == "" || value == "." || value == ".." || strings.HasPrefix(value, ".") {
		return false
	}
	if strings.ContainsAny(value, "/\\") || filepath.IsAbs(value) || filepath.Clean(value) != value {
		return false
	}
	return model.IsActiveWorkflowDirName(value)
}

func packageNotFound(ref Ref, plan Plan) error {
	err := newError(ErrPackageNotFound, ref.Initiative, ref.PackageID, plan.Path, nil)
	err.ValidPackageIDs = plan.PackageIDs()
	return err
}
