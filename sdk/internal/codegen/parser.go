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
	handler := func(field *ast.Field) error {
		if len(field.Names) == 0 {
			embeddedType := getEmbeddedTypeName(field.Type)
			if embeddedType == "" {
				return nil
			}
			embeddedFields, embeddedErr := parseEmbeddedStruct(fset, filePath, embeddedType)
			if embeddedErr != nil {
				return embeddedErr
			}
			info.Fields = append(info.Fields, embeddedFields...)
			return nil
		}
		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			return nil
		}
		fieldInfo := analyzeFieldType(field)
		fieldInfo.Name = fieldName
		if field.Doc != nil {
			fieldInfo.Comment = strings.TrimSpace(field.Doc.Text())
		}
		info.Fields = append(info.Fields, fieldInfo)
		return nil
	}
	found, walkErr := walkStructFields(node, structName, handler)
	if walkErr != nil {
		return nil, walkErr
	}
	if !found {
		return nil, fmt.Errorf("struct %s not found in %s", structName, filePath)
	}
	return info, nil
}

func walkStructFields(node ast.Node, structName string, fn func(*ast.Field) error) (bool, error) {
	found := false
	var walkErr error
	ast.Inspect(node, func(n ast.Node) bool {
		if walkErr != nil {
			return false
		}
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
			if err := fn(field); err != nil {
				walkErr = err
				return false
			}
		}
		return false
	})
	return found, walkErr
}

func analyzeFieldType(field *ast.Field) FieldInfo {
	return analyzeType(field.Type)
}

func analyzeType(expr ast.Expr) FieldInfo {
	switch t := expr.(type) {
	case *ast.Ident:
		return FieldInfo{Type: t.Name}
	case *ast.SelectorExpr:
		info := FieldInfo{Type: exprToString(t)}
		if x, ok := t.X.(*ast.Ident); ok {
			info.PackagePath = x.Name
		}
		return info
	case *ast.StarExpr:
		inner := analyzeType(t.X)
		return FieldInfo{
			Type:        inner.Type,
			IsPtr:       true,
			IsSlice:     inner.IsSlice,
			IsMap:       inner.IsMap,
			KeyType:     inner.KeyType,
			ValueType:   inner.ValueType,
			PackagePath: inner.PackagePath,
		}
	case *ast.ArrayType:
		inner := analyzeType(t.Elt)
		valueType := inner.Type
		if inner.IsPtr {
			valueType = "*" + inner.Type
		}
		return FieldInfo{
			Type:        inner.Type,
			IsSlice:     true,
			IsMap:       inner.IsMap,
			KeyType:     inner.KeyType,
			ValueType:   valueType,
			PackagePath: inner.PackagePath,
		}
	case *ast.MapType:
		keyInfo := analyzeType(t.Key)
		valueInfo := analyzeType(t.Value)
		valueType := valueInfo.Type
		if valueInfo.IsPtr {
			valueType = "*" + valueInfo.Type
		}
		return FieldInfo{
			Type:      fmt.Sprintf("map[%s]%s", keyInfo.Type, valueType),
			IsMap:     true,
			KeyType:   keyInfo.Type,
			ValueType: valueType,
		}
	default:
		return FieldInfo{Type: exprToString(expr)}
	}
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

func parseEmbeddedStruct(fset *token.FileSet, filePath string, typeName string) ([]FieldInfo, error) {
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse embedded struct %s in %s: %w", typeName, filePath, err)
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
	return fields, nil
}
