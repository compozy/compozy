package main

import (
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
	maxFunctionLines = 30
	excludedDirs     = "vendor,node_modules,.git,bin,docs/node_modules"
)

type FunctionInfo struct {
	File     string
	Name     string
	Lines    int
	StartPos int
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	functions, err := analyzeFunctionLengths(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	reportResults(functions)
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

func shouldSkipDir(name string, excluded []string) bool {
	for _, dir := range excluded {
		if name == dir {
			return true
		}
	}
	return false
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

		// Only count function body lines, not the signature
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

func reportResults(functions []FunctionInfo) {
	if len(functions) == 0 {
		fmt.Println("✅ All functions are within the 30-line limit!")
		return
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
	os.Exit(1)
}
