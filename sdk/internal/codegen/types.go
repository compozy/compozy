// Package codegen provides tools for automatically generating functional options
// from engine struct definitions using Go's AST parser and Jennifer code generator.
package codegen

// StructInfo holds metadata about a discovered struct
type StructInfo struct {
	PackageName string
	StructName  string
	Fields      []FieldInfo
}

// FieldInfo represents a single struct field with type information
type FieldInfo struct {
	Name        string
	Type        string
	IsSlice     bool
	IsPtr       bool
	IsMap       bool
	KeyType     string // For map types
	ValueType   string // For map types or slice element types
	PackagePath string // Full import path if from another package
	Comment     string // Documentation comment
}
