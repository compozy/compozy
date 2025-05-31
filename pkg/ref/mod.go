package ref

import (
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const (
	// DefaultMaxDepth is the maximum depth for reference resolution to prevent infinite recursion.
	DefaultMaxDepth = 20
	// MaxFileSize is the maximum file size (10MB) for loading documents.
	MaxFileSize = 10 << 20
)

// Type defines the type of reference.
type Type string

// Mode defines the merge mode for combining reference and inline values.
type Mode string

const (
	// Reference types
	TypeProperty Type = "property"
	TypeFile     Type = "file"
	TypeGlobal   Type = "global"

	// Merge modes
	ModeMerge   Mode = "merge"
	ModeReplace Mode = "replace"
	ModeAppend  Mode = "append"
)

// Document represents a parsed YAML/JSON document with methods for accessing its contents.
type Document interface {
	Get(path string) (any, error)
	AsMap() (map[string]any, bool)
	AsSlice() ([]any, bool)
}

// simpleDocument is a basic implementation of the Document interface for any type.
type simpleDocument struct {
	data any
}

func (d *simpleDocument) Get(path string) (any, error) {
	return walkGJSONPath(d.data, path, nil)
}

func (d *simpleDocument) AsMap() (map[string]any, bool) {
	m, ok := d.data.(map[string]any)
	return m, ok
}

func (d *simpleDocument) AsSlice() ([]any, bool) {
	s, ok := d.data.([]any)
	return s, ok
}

// -----------------------------------------------------------------------------
// Ref
// -----------------------------------------------------------------------------

// Ref represents a reference to a property, file, or global configuration.
type Ref struct {
	Type Type   `json:"type" yaml:"type"`
	Path string `json:"path" yaml:"path"`
	Mode Mode   `json:"mode" yaml:"mode"`
	File string `json:"file" yaml:"file"`
}

// String returns the string representation of the reference.
func (r *Ref) String() string {
	if r == nil {
		return ""
	}
	var source string
	switch r.Type {
	case TypeGlobal:
		source = "$global"
	case TypeFile:
		source = r.File
	case TypeProperty:
		source = ""
	default:
		source = ""
	}
	var result strings.Builder
	hasSource := source != ""
	hasPath := r.Path != ""
	if hasSource {
		result.WriteString(source)
	}
	if hasSource && hasPath {
		result.WriteString("::")
	}
	if hasPath {
		result.WriteString(r.Path)
	}
	if r.Mode != "" && r.Mode != ModeMerge {
		result.WriteString("!")
		result.WriteString(string(r.Mode))
	}
	return result.String()
}

// DocMetadata holds metadata for document resolution.
type DocMetadata struct {
	CurrentDoc      Document
	CurrentDocJSON  []byte // Read-only: pre-marshaled JSON for GJSON path lookups
	FilePath        string
	ProjectRoot     string
	MaxDepth        int
	ResolutionStack []string
	// Per-goroutine cycle detection (not shared across goroutines)
	inStack map[string]struct{} // Fast lookup for cycle detection
}

// Error represents a reference resolution error with context.
type Error struct {
	Message  string
	FilePath string
	Line     int
	Column   int
	Err      error
}

func (e *Error) Error() string {
	if e.FilePath != "" && e.Line > 0 {
		return fmt.Sprintf("%s at %s:%d:%d", e.Message, e.FilePath, e.Line, e.Column)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}
