package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// ParseStruct automatically discovers all fields from a struct definition
func ParseStruct(filePath string, structName string) (*StructInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}
	info := &StructInfo{
		PackageName: node.Name.Name,
		StructName:  structName,
		Fields:      make([]FieldInfo, 0),
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != structName {
			return true
		}
		found = true
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				embeddedType := getEmbeddedTypeName(field.Type)
				if embeddedType != "" {
					embeddedFields := parseEmbeddedStruct(fset, filePath, embeddedType)
					info.Fields = append(info.Fields, embeddedFields...)
				}
				continue
			}
			fieldName := field.Names[0].Name
			if !ast.IsExported(fieldName) {
				continue
			}
			fieldInfo := analyzeFieldType(field)
			fieldInfo.Name = fieldName
			if field.Doc != nil {
				fieldInfo.Comment = strings.TrimSpace(field.Doc.Text())
			}
			info.Fields = append(info.Fields, fieldInfo)
		}
		return false
	})
	if !found {
		return nil, fmt.Errorf("struct %s not found in %s", structName, filePath)
	}
	return info, nil
}

func analyzeFieldType(field *ast.Field) FieldInfo {
	info := FieldInfo{}
	switch t := field.Type.(type) {
	case *ast.Ident:
		info.Type = t.Name
	case *ast.SelectorExpr:
		info.Type = exprToString(t)
		if x, ok := t.X.(*ast.Ident); ok {
			info.PackagePath = x.Name
		}
	case *ast.StarExpr:
		info.IsPtr = true
		innerInfo := analyzeExpr(t.X)
		info.Type = innerInfo.Type
		info.PackagePath = innerInfo.PackagePath
		info.IsSlice = innerInfo.IsSlice
		info.IsMap = innerInfo.IsMap
		info.ValueType = innerInfo.ValueType
		info.KeyType = innerInfo.KeyType
	case *ast.ArrayType:
		info.IsSlice = true
		innerInfo := analyzeExpr(t.Elt)
		info.Type = innerInfo.Type
		info.PackagePath = innerInfo.PackagePath
		if innerInfo.IsPtr {
			info.ValueType = "*" + innerInfo.Type
		} else {
			info.ValueType = innerInfo.Type
		}
	case *ast.MapType:
		info.IsMap = true
		keyInfo := analyzeExpr(t.Key)
		valueInfo := analyzeExpr(t.Value)
		info.KeyType = keyInfo.Type
		info.ValueType = valueInfo.Type
		info.Type = fmt.Sprintf("map[%s]%s", keyInfo.Type, valueInfo.Type)
	default:
		info.Type = exprToString(field.Type)
	}
	return info
}

func analyzeExpr(expr ast.Expr) FieldInfo {
	info := FieldInfo{}
	switch t := expr.(type) {
	case *ast.Ident:
		info.Type = t.Name
	case *ast.SelectorExpr:
		info.Type = exprToString(t)
		if x, ok := t.X.(*ast.Ident); ok {
			info.PackagePath = x.Name
		}
	case *ast.StarExpr:
		info.IsPtr = true
		innerInfo := analyzeExpr(t.X)
		info.Type = innerInfo.Type
		info.PackagePath = innerInfo.PackagePath
	case *ast.ArrayType:
		info.IsSlice = true
		innerInfo := analyzeExpr(t.Elt)
		info.Type = innerInfo.Type
		info.ValueType = innerInfo.Type
		info.PackagePath = innerInfo.PackagePath
	default:
		info.Type = exprToString(expr)
	}
	return info
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	default:
		return "interface{}"
	}
}

func getEmbeddedTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return ""
	default:
		return ""
	}
}

func parseEmbeddedStruct(fset *token.FileSet, filePath string, typeName string) []FieldInfo {
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	var fields []FieldInfo
	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != typeName {
			return true
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}
			fieldName := field.Names[0].Name
			if !ast.IsExported(fieldName) {
				continue
			}
			fieldInfo := analyzeFieldType(field)
			fieldInfo.Name = fieldName
			if field.Doc != nil {
				fieldInfo.Comment = strings.TrimSpace(field.Doc.Text())
			}
			fields = append(fields, fieldInfo)
		}
		return false
	})
	return fields
}
