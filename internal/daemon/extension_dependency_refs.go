package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/fileutil"
)

func (node *dependencyNode) matchLocalDependencyRef(
	ctx context.Context, parent, parentDir string, dependency extensionpkg.ExtensionDependency, ref string,
) (string, bool, error) {
	sameRef := node.declaredRef == ref
	if sameRef && (node.installed || node.declaredBy == parent) {
		return ref, true, nil
	}
	first := node.payload.Ref
	if node.installed {
		declaration := extensionpkg.ExtensionDependency{Name: dependency.Name, Ref: node.declaredRef}
		resolved, err := resolveLocalDependencyRef(ctx, declaration, node.declaredDir, node.declaredRef)
		if errors.Is(err, os.ErrNotExist) {
			return ref, false, nil
		}
		if err != nil {
			return "", false, err
		}
		first = resolved
	}
	resolved, err := resolveLocalDependencyRef(ctx, dependency, parentDir, ref)
	if node.installed && errors.Is(err, os.ErrNotExist) {
		return ref, false, nil
	}
	return resolved, first == resolved, err
}

func dependencyDeclarationRef(dependency extensionpkg.ExtensionDependency, parentDir string) (string, error) {
	if contract.InstallExtensionSource(dependency.Source) != contract.InstallExtensionSourceLocalPath {
		return dependency.Ref, nil
	}
	if !filepath.IsAbs(parentDir) {
		return "", invalidLocalDependencyRef(
			dependency,
			"local_path dependency requires a local_path dependent",
		)
	}
	if !filepath.IsLocal(dependency.Ref) {
		return "", invalidLocalDependencyRef(dependency, "local_path ref must stay inside the source directory")
	}
	return filepath.Join(parentDir, dependency.Ref), nil
}

func resolveLocalDependencyRef(
	ctx context.Context, dependency extensionpkg.ExtensionDependency, parentDir, ref string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	_, resolved, err := fileutil.ResolveExistingPathWithinRoot(parentDir, ref)
	if errors.Is(err, fileutil.ErrPathOutsideRoot) {
		return "", invalidLocalDependencyRef(dependency, "local_path ref must resolve inside the source directory")
	}
	if err != nil {
		return "", &extensionpkg.DependencyGateError{
			Dependency: dependency.Name,
			Err:        fmt.Errorf("daemon: resolve local dependency ref %q: %w", dependency.Ref, err),
		}
	}
	return resolved, ctx.Err()
}

func invalidLocalDependencyRef(dependency extensionpkg.ExtensionDependency, message string) error {
	return &extensionpkg.DependencyGateError{
		Dependency: dependency.Name,
		Err: &extensionpkg.ManifestValidationError{
			Field: "requires.extensions.ref", Value: dependency.Ref, Message: message,
		},
	}
}
