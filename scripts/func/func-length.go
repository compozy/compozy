package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxFunctionLines = 50
	excludedDirs     = "vendor,node_modules,.git,bin,docs/node_modules"
)

var errFuncLengthViolations = errors.New("function length violations detected")

type FunctionInfo struct {
	File     string
	Name     string
	Lines    int
	StartPos int
}

func runFuncLengthCheck(root string) error {
	functions, err := analyzeFunctionLengths(root)
	if err != nil {
		return err
	}
	violations := reportResults(functions)
	if violations > 0 {
		return fmt.Errorf("%w: %d functions exceed %d lines", errFuncLengthViolations, violations, maxFunctionLines)
	}
	return nil
}

func analyzeFunctionLengths(root string) ([]FunctionInfo, error) {
	var functions []FunctionInfo
	excluded := strings.Split(excludedDirs, ",")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name(), excluded) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		funcs, err := analyzeFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, err)
			return nil
		}
		functions = append(functions, funcs...)
		return nil
	})
	return functions, err
}

func analyzeFile(filename string) ([]FunctionInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var functions []FunctionInfo
	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		bodyStartLine := fset.Position(funcDecl.Body.Pos()).Line
		bodyEndLine := fset.Position(funcDecl.End()).Line
		lines := bodyEndLine - bodyStartLine + 1

		if lines > maxFunctionLines {
			funcName := funcDecl.Name.Name
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				recvType := extractReceiverType(funcDecl.Recv.List[0].Type)
				funcName = fmt.Sprintf("(%s).%s", recvType, funcName)
			}

			functions = append(functions, FunctionInfo{
				File:     filename,
				Name:     funcName,
				Lines:    lines,
				StartPos: bodyStartLine,
			})
		}

		return true
	})
	return functions, nil
}

func extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + extractReceiverType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return extractReceiverType(t.X)
	case *ast.IndexListExpr:
		return extractReceiverType(t.X)
	default:
		return "?"
	}
}

func reportResults(functions []FunctionInfo) int {
	if len(functions) == 0 {
		fmt.Printf("✅ All functions are within the %d-line limit!\n", maxFunctionLines)
		return 0
	}
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Lines == functions[j].Lines {
			return functions[i].File < functions[j].File
		}
		return functions[i].Lines > functions[j].Lines
	})
	fmt.Printf("Found %d functions with more than %d lines:\n\n", len(functions), maxFunctionLines)
	for _, fn := range functions {
		fmt.Printf("📄 %s:%d\n", fn.File, fn.StartPos)
		fmt.Printf("   Function: %s\n", fn.Name)
		fmt.Printf("   Lines: %d (exceeds limit by %d)\n\n", fn.Lines, fn.Lines-maxFunctionLines)
	}
	fmt.Printf("Total violations: %d\n", len(functions))
	return len(functions)
}
