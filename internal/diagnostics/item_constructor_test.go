package diagnostics

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticItemConstructionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should reject production DiagnosticItem literals outside constructors", func(t *testing.T) {
		t.Parallel()

		repoRoot := findRepoRoot(t)
		var violations []string
		err := filepath.WalkDir(
			filepath.Join(repoRoot, "internal"),
			func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return fmt.Errorf("walk %s: %w", path, walkErr)
				}
				if entry.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				relPath, err := filepath.Rel(repoRoot, path)
				if err != nil {
					return fmt.Errorf("rel %s: %w", path, err)
				}
				if filepath.ToSlash(relPath) == "internal/diagnostics/item.go" {
					return nil
				}
				fileSet := token.NewFileSet()
				file, err := parser.ParseFile(fileSet, path, nil, 0)
				if err != nil {
					return fmt.Errorf("parse %s: %w", relPath, err)
				}
				ast.Inspect(file, func(node ast.Node) bool {
					literal, ok := node.(*ast.CompositeLit)
					if !ok {
						return true
					}
					if isDiagnosticItemLiteral(literal.Type) {
						position := fileSet.Position(literal.Lbrace)
						violations = append(violations, fmt.Sprintf("%s:%d", filepath.ToSlash(relPath), position.Line))
					}
					return true
				})
				return nil
			},
		)
		if err != nil {
			t.Fatalf("WalkDir() error = %v", err)
		}
		if len(violations) > 0 {
			t.Fatalf("DiagnosticItem literals outside diagnostics.NewItem: %s", strings.Join(violations, ", "))
		}
	})
}

func isDiagnosticItemLiteral(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		return typed.Sel.Name == "DiagnosticItem"
	case *ast.Ident:
		return typed.Name == "DiagnosticItem"
	default:
		return false
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		} else if !os.IsNotExist(statErr) {
			t.Fatalf("Stat(go.mod) error = %v", statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with go.mod not found")
		}
		dir = parent
	}
}
