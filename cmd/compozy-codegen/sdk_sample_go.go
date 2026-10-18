package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

const sdkGoRequestFields = "{ /* request fields */ }"

func sdkGoRequestExpressions(files map[string][]byte) (map[string]string, error) {
	parsed := make([]*ast.File, 0, len(files))
	declarations := make(map[string]ast.Expr)
	for name, content := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, content, 0)
		if err != nil {
			return nil, fmt.Errorf("parse generated Go SDK %s: %w", name, err)
		}
		parsed = append(parsed, file)
		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, entry := range group.Specs {
				if named, ok := entry.(*ast.TypeSpec); ok {
					declarations[named.Name.Name] = named.Type
				}
			}
		}
	}
	result := make(map[string]string)
	for _, file := range parsed {
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
					expression, err := sdkGoRequestExpression(parameter.Type, declarations)
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

func sdkGoRequestExpression(expression ast.Expr, declarations map[string]ast.Expr) (string, error) {
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
		if named, ok := value.X.(*ast.Ident); ok && named.Name == "UploadFile" {
			return "&" + element + "{ /* name, content type, reader */ }", nil
		}
		if sdkGoStructType(value.X, declarations) {
			return "&" + element + sdkGoRequestFields, nil
		}
		return "new(" + element + ")", nil
	case *ast.MapType, *ast.ArrayType:
		return typeName + sdkGoRequestFields, nil
	case *ast.Ident:
		if wire, ok := declarations[value.Name].(*ast.SelectorExpr); ok {
			if qualifier, ok := wire.X.(*ast.Ident); ok && qualifier.Name == sdkJSONName &&
				wire.Sel.Name == "RawMessage" {
				return typeName + `("{}")`, nil
			}
		}
		if sdkGoStructType(value, declarations) {
			return typeName + sdkGoRequestFields, nil
		}
		switch value.Name {
		case sdkParamString:
			return `"value"`, nil
		case "bool":
			return sdkSampleTrue, nil
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

func sdkGoStructType(expression ast.Expr, declarations map[string]ast.Expr) bool {
	visited := make(map[string]bool)
	for {
		switch value := expression.(type) {
		case *ast.StructType:
			return true
		case *ast.Ident:
			if visited[value.Name] {
				return false
			}
			visited[value.Name] = true
			expression = declarations[value.Name]
		default:
			return false
		}
	}
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
