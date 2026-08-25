//go:build mage

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type goTestPackageFiles struct {
	Dir          string
	TestGoFiles  []string
	XTestGoFiles []string
}

func listGoTopLevelTests(ctx context.Context, packagePath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-race", "-json", packagePath)
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(nil))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list %s test files: %w: %s", packagePath, err, strings.TrimSpace(string(output)))
	}

	var packageFiles goTestPackageFiles
	if err := json.Unmarshal(output, &packageFiles); err != nil {
		return nil, fmt.Errorf("decode %s test files: %w", packagePath, err)
	}
	files := append(append([]string(nil), packageFiles.TestGoFiles...), packageFiles.XTestGoFiles...)
	tests := make([]string, 0, len(files))
	for _, name := range files {
		fileTests, err := topLevelTestsInFile(filepath.Join(packageFiles.Dir, name))
		if err != nil {
			return nil, fmt.Errorf("discover %s tests: %w", packagePath, err)
		}
		tests = append(tests, fileTests...)
	}
	if len(tests) == 0 {
		return nil, fmt.Errorf("discover %s tests: no top-level tests found", packagePath)
	}
	slices.Sort(tests)
	return slices.Compact(tests), nil
}

func topLevelTestsInFile(path string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	testingImports := testingImportNames(file)
	tests := make([]string, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && isTopLevelGoTest(function, testingImports) {
			tests = append(tests, function.Name.Name)
		}
	}
	return tests, nil
}

func testingImportNames(file *ast.File) map[string]struct{} {
	names := make(map[string]struct{}, 1)
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || importPath != "testing" {
			continue
		}
		name := "testing"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = struct{}{}
	}
	return names
}

func isTopLevelGoTest(function *ast.FuncDecl, testingImports map[string]struct{}) bool {
	if function.Recv != nil || !isGoTestName(function.Name.Name) || function.Type.Results != nil {
		return false
	}
	if function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return false
	}
	pointer, ok := function.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if ok {
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || selector.Sel.Name != "T" {
			return false
		}
		_, ok = testingImports[qualifier.Name]
		return ok
	}
	identifier, ok := pointer.X.(*ast.Ident)
	if !ok || identifier.Name != "T" {
		return false
	}
	_, ok = testingImports["."]
	return ok
}

func isGoTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	remainder := strings.TrimPrefix(name, "Test")
	if remainder == "" {
		return true
	}
	next, _ := utf8.DecodeRuneInString(remainder)
	return !unicode.IsLower(next)
}
