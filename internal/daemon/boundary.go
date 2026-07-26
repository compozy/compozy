package daemon

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	boundaryTrueKey = "true"
	boundaryYesKey  = "yes"
)

const moduleImportPath = "github.com/compozy/agh"

// Boundaries performs a best-effort import boundary verification for local source checkouts.
func (d *Daemon) Boundaries(context.Context) error {
	root := strings.TrimSpace(d.boundaryRoot)
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("daemon: resolve working directory for boundary check: %w", err)
		}
		root = cwd
	}

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("daemon: stat go.mod for boundary check: %w", err)
	}

	violations, err := verifyImportBoundaries(root)
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}

	return errors.Join(violations...)
}

func (d *Daemon) shouldVerifyBoundaries() bool {
	if d.verifyBoundaries {
		return true
	}

	envGetter := d.getenv
	if envGetter == nil {
		envGetter = os.Getenv
	}
	value := strings.ToLower(strings.TrimSpace(envGetter("AGH_DEV_VERIFY_BOUNDARIES")))
	return value == "1" || value == boundaryTrueKey || value == boundaryYesKey
}

func verifyImportBoundaries(root string) ([]error, error) {
	internalRoot := filepath.Join(root, "internal")
	forbiddenImports := map[string]struct{}{
		moduleImportPath + "/internal/daemon":      {},
		moduleImportPath + "/internal/api/httpapi": {},
		moduleImportPath + "/internal/api/udsapi":  {},
		moduleImportPath + "/internal/cli":         {},
	}
	memoryContractForbiddenImports := map[string]struct{}{
		moduleImportPath + "/internal/memory":                {},
		moduleImportPath + "/internal/memory/controller":     {},
		moduleImportPath + "/internal/memory/recall":         {},
		moduleImportPath + "/internal/memory/extractor":      {},
		moduleImportPath + "/internal/memory/provider/local": {},
		moduleImportPath + "/internal/store/workspacedb":     {},
	}
	daemonPackage := moduleImportPath + "/internal/daemon"
	memoryContractPackage := moduleImportPath + "/internal/memory/contract"

	violations := make([]error, 0)
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("daemon: parse %q for boundary verification: %w", path, err)
		}

		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(root, dir)
		if err != nil {
			return fmt.Errorf("daemon: resolve relative package path for %q: %w", dir, err)
		}
		importer := moduleImportPath + "/" + filepath.ToSlash(relDir)
		if importer == daemonPackage || strings.HasPrefix(importer, daemonPackage+"/") {
			return nil
		}

		for _, spec := range parsed.Imports {
			target, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("daemon: decode import path in %q: %w", path, err)
			}
			if _, forbidden := forbiddenImports[target]; forbidden {
				violations = append(
					violations,
					fmt.Errorf("daemon: boundary violation: %s imports %s", importer, target),
				)
			}
			if importer == memoryContractPackage {
				if _, forbidden := memoryContractForbiddenImports[target]; forbidden {
					violations = append(
						violations,
						fmt.Errorf("daemon: boundary violation: %s imports %s", importer, target),
					)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return violations, nil
}
