//go:build mage

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type sourcePolicyViolation struct {
	path    string
	line    int
	message string
}

// SourcePolicy enforces repository-specific Go source ownership rules.
func SourcePolicy() error {
	violations, err := inspectSourcePolicies(".")
	if err != nil {
		return err
	}
	for _, violation := range violations {
		fmt.Printf("VIOLATION: %s:%d: %s\n", violation.path, violation.line, violation.message)
	}
	if len(violations) > 0 {
		return fmt.Errorf("found %d source policy violations", len(violations))
	}
	fmt.Println("OK: all Go source policies respected")
	return nil
}

func inspectSourcePolicies(root string) ([]sourcePolicyViolation, error) {
	paths, err := policyGoFiles(root)
	if err != nil {
		return nil, err
	}
	violations := make([]sourcePolicyViolation, 0)
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, fmt.Errorf("resolve source policy path %q: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, "_integration_test.go") {
			violation, err := integrationBuildConstraintViolation(path, rel)
			if err != nil {
				return nil, err
			}
			if violation != nil {
				violations = append(violations, *violation)
			}
		}
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		fileViolations, err := inspectProductionGoPolicy(path, rel)
		if err != nil {
			return nil, err
		}
		violations = append(violations, fileViolations...)
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].path == violations[j].path {
			if violations[i].line == violations[j].line {
				return violations[i].message < violations[j].message
			}
			return violations[i].line < violations[j].line
		}
		return violations[i].path < violations[j].path
	})
	return violations, nil
}

func policyGoFiles(root string) ([]string, error) {
	files := make([]string, 0)
	for _, sourceRoot := range []string{"cmd", "internal"} {
		path := filepath.Join(root, sourceRoot)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("inspect source policy root %q: %w", path, err)
		}
		if err := filepath.WalkDir(path, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				switch entry.Name() {
				case "testdata", "vendor":
					return filepath.SkipDir
				default:
					return nil
				}
			}
			if filepath.Ext(name) == ".go" {
				files = append(files, name)
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walk source policy root %q: %w", path, err)
		}
	}
	sort.Strings(files)
	return files, nil
}

func inspectProductionGoPolicy(path string, rel string) ([]sourcePolicyViolation, error) {
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse production Go source %q: %w", rel, err)
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}
	imports := importedPaths(file)
	violations := make([]sourcePolicyViolation, 0)
	if !pathWithin(rel, "internal/providerenv") && !pathWithin(rel, "internal/testutil") {
		violations = append(violations, inspectProviderHomeMutations(fileset, file, imports, rel)...)
	}
	if !pathWithin(rel, "internal/procutil") {
		violations = append(violations, inspectProcessControl(fileset, file, imports, rel)...)
	}
	return violations, nil
}

func importedPaths(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		imports[name] = importPath
	}
	return imports
}

func importedCall(call *ast.CallExpr, imports map[string]string, importPath string, name string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && imports[ident.Name] == importPath
}

func isImportedType(expr ast.Expr, imports map[string]string, importPath string, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && imports[ident.Name] == importPath
}

func isImportedPointerType(expr ast.Expr, imports map[string]string, importPath string, name string) bool {
	pointer, ok := expr.(*ast.StarExpr)
	return ok && isImportedType(pointer.X, imports, importPath, name)
}

func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func pathWithin(path string, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}
