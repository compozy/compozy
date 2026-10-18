package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

func sdkGoRequestExpressions(files map[string][]byte) (map[string]string, error) {
	result := make(map[string]string)
	for name, content := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, content, 0)
		if err != nil {
			return nil, fmt.Errorf("parse generated Go SDK %s: %w", name, err)
		}
		for _, declaration := range file.Decls {
			method, ok := declaration.(*ast.FuncDecl)
			if !ok || method.Recv == nil {
				continue
			}
			for _, parameter := range method.Type.Params.List {
				for _, name := range parameter.Names {
					if name.Name != "body" {
						continue
					}
					expression, err := sdkGoRequestExpression(parameter.Type)
					if err != nil {
						return nil, fmt.Errorf("SDK sample %s request: %w", method.Name.Name, err)
					}
					result[method.Name.Name] = expression
				}
			}
		}
	}
	return result, nil
}

func sdkGoRequestExpression(expression ast.Expr) (string, error) {
	typeName, err := sdkGoSampleType(expression)
	if err != nil {
		return "", err
	}
	switch value := expression.(type) {
	case *ast.StarExpr:
		element, err := sdkGoSampleType(value.X)
		if err != nil {
			return "", err
		}
		return "new(" + element + ")", nil
	case *ast.MapType, *ast.ArrayType:
		return typeName + "{ /* request fields */ }", nil
	case *ast.Ident:
		switch value.Name {
		case "string":
			return `"value"`, nil
		case "bool":
			return "true", nil
		case "any":
			return "nil", nil
		}
		if ast.IsExported(value.Name) {
			return "*new(" + typeName + ")", nil
		}
		return "0", nil
	case *ast.SelectorExpr:
		if typeName == "json.RawMessage" {
			return `json.RawMessage("{}")`, nil
		}
	}
	return "*new(" + typeName + ")", nil
}

func sdkGoSampleType(expression ast.Expr) (string, error) {
	switch value := expression.(type) {
	case *ast.Ident:
		if ast.IsExported(value.Name) {
			return "client." + value.Name, nil
		}
		return value.Name, nil
	case *ast.StarExpr:
		element, err := sdkGoSampleType(value.X)
		return "*" + element, err
	case *ast.ArrayType:
		element, err := sdkGoSampleType(value.Elt)
		if err != nil {
			return "", err
		}
		var length strings.Builder
		if value.Len != nil {
			if err := format.Node(&length, token.NewFileSet(), value.Len); err != nil {
				return "", err
			}
		}
		return "[" + length.String() + "]" + element, nil
	case *ast.MapType:
		key, err := sdkGoSampleType(value.Key)
		if err != nil {
			return "", err
		}
		element, err := sdkGoSampleType(value.Value)
		return "map[" + key + "]" + element, err
	default:
		var name strings.Builder
		if err := format.Node(&name, token.NewFileSet(), expression); err != nil {
			return "", err
		}
		return name.String(), nil
	}
}
