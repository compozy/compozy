# `tplengine` – _Enhanced Go Template Engine for Dynamic Configuration_

> **A powerful template processing engine that uses Go templates with enhanced functionality for safe processing of dynamic configuration values with strict error handling.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `tplengine` package provides a powerful template processing engine for Compozy that uses Go templates with enhanced functionality. It's designed to safely process dynamic configuration values with strict error handling and comprehensive template features, enabling dynamic YAML/JSON configuration processing with type preservation and precision handling.

The engine supports multiple output formats, includes all Sprig template functions, and provides object reference detection for seamless integration with Compozy's workflow orchestration system.

---

## 💡 Motivation

- **Dynamic Configuration**: Enable template-based configuration processing for workflow orchestration
- **Type Safety**: Preserve data types and numeric precision during template processing
- **Error Prevention**: Strict error handling with `missingkey=error` to catch configuration issues early
- **Cross-Component References**: Allow components to reference each other's outputs dynamically

---

## ⚡ Design Highlights

- **Strict Error Handling**: `missingkey=error` prevents silent failures and catches typos early
- **Multiple Output Formats**: Support for YAML, JSON, and plain text output with automatic parsing
- **Sprig Integration**: Complete integration with Sprig template functions for enhanced functionality
- **Object Reference Detection**: Smart handling of simple object references to preserve data types
- **Numeric Precision**: Optional precision preservation for large numbers and decimals
- **Concurrent Safety**: Thread-safe template engine for concurrent workflow processing
- **XSS Protection**: Built-in HTML escaping functions for secure template rendering

---

## 🚀 Getting Started

### Prerequisites

- Go 1.24+
- Understanding of Go template syntax

### Installation

```go
import "github.com/compozy/compozy/pkg/tplengine"
```

### Quick Start

```go
// Create a template engine
engine := tplengine.NewEngine(tplengine.FormatYAML)

// Render a simple template
result, err := engine.RenderString("Hello {{ .name }}!", map[string]any{
    "name": "World",
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(result) // Output: "Hello World!"

// Check if string contains templates
if tplengine.HasTemplate("{{ .name }}") {
    // Process template
}
```

---

## 📖 Usage

### Library

#### Basic Template Rendering

```go
// Create engine with specific output format
engine := tplengine.NewEngine(tplengine.FormatYAML)

// Simple string rendering
result, err := engine.RenderString("Hello {{ .name }}!", map[string]any{
    "name": "World",
})

// Process complex data structures
data := map[string]any{
    "config": map[string]any{
        "host": "{{ .env.DB_HOST }}",
        "port": "{{ .env.DB_PORT | default \"5432\" }}",
    },
}
result, err := engine.ParseAny(data, context)
```

#### File Processing

```go
// Process template files
result, err := engine.ProcessFile("config.yaml", context)
if err != nil {
    log.Fatal(err)
}

// Access different format outputs
fmt.Println("Text:", result.Text)
fmt.Println("YAML:", result.YAML)
fmt.Println("JSON:", result.JSON)
```

#### Advanced Features

```go
// Add global values available in all templates
engine.AddGlobalValue("version", "1.0.0")
engine.AddGlobalValue("environment", "production")

// Enable numeric precision preservation
engine = engine.WithPrecisionPreservation(true)

// Add named templates
err := engine.AddTemplate("greeting", "Hello {{ .name }}!")
result, err := engine.Render("greeting", context)
```

---

## 🔧 Configuration

### Output Formats

```go
const (
    FormatYAML EngineFormat = "yaml"  // Parse as YAML
    FormatJSON EngineFormat = "json"  // Parse as JSON
    FormatText EngineFormat = "text"  // Plain text only
)

// Change format
engine = engine.WithFormat(tplengine.FormatJSON)
```

### Template Behavior

The engine uses `missingkey=error` by default, which means any attempt to access a non-existent key will fail rather than silently render `<no value>`:

```go
// ❌ This will FAIL if .user.age doesn't exist
"{{ .user.age | default \"25\" }}"

// ✅ This works - check existence first
"{{ if hasKey .user \"age\" }}{{ .user.age }}{{ else }}25{{ end }}"
```

### Security Features

The engine includes XSS protection functions:

```go
// HTML escaping functions available in templates
funcMap := map[string]any{
    "htmlEscape":     html.EscapeString,
    "htmlAttrEscape": html.EscapeString,
    "jsEscape":       html_template.JSEscapeString,
}
```

---

## 🎨 Examples

### Basic Template Processing

```go
engine := tplengine.NewEngine(tplengine.FormatText)

context := map[string]any{
    "user": map[string]any{
        "name":  "John",
        "email": "john@example.com",
    },
    "settings": map[string]any{
        "timeout": "30s",
        "debug":   true,
    },
}

// Simple variable access
result, _ := engine.RenderString("Hello {{ .user.name }}!", context)
// Result: "Hello John!"

// With Sprig functions
result, _ := engine.RenderString("{{ .user.name | upper }}", context)
// Result: "JOHN"
```

### Safe Key Access Patterns

```go
// ✅ Safe approaches for missing keys
templates := []string{
    // Check key existence
    "{{ if hasKey .user \"age\" }}{{ .user.age }}{{ else }}unknown{{ end }}",
    
    // Conditional with default
    "{{ if .user.age }}{{ .user.age }}{{ else }}0{{ end }}",
    
    // Nested conditionals
    "{{ if hasKey . \"database\" }}{{ .database.host | default \"localhost\" }}{{ else }}localhost{{ end }}",
}

for _, tmpl := range templates {
    result, err := engine.RenderString(tmpl, context)
    if err != nil {
        log.Printf("Template error: %v", err)
    }
}
```

### Complex Data Processing

```go
// Process nested data structures
data := map[string]any{
    "services": map[string]any{
        "web": map[string]any{
            "host": "{{ .env.WEB_HOST }}",
            "port": "{{ .env.WEB_PORT | default \"8080\" }}",
        },
        "db": map[string]any{
            "host": "{{ .env.DB_HOST }}",
            "port": "{{ .env.DB_PORT | default \"5432\" }}",
        },
    },
    "features": []any{
        "{{ if .flags.feature_a }}feature-a{{ end }}",
        "{{ if .flags.feature_b }}feature-b{{ end }}",
    },
}

context := map[string]any{
    "env": map[string]any{
        "WEB_HOST": "localhost",
        "DB_HOST":  "postgres.example.com",
        "DB_PORT":  "5432",
    },
    "flags": map[string]any{
        "feature_a": true,
        "feature_b": false,
    },
}

result, err := engine.ParseAny(data, context)
```

### Object Reference Detection

```go
// Simple object references preserve types
context := map[string]any{
    "tasks": map[string]any{
        "processor": map[string]any{
            "output": map[string]any{
                "data": []int{1, 2, 3, 4, 5},
            },
        },
    },
}

// This preserves the []int type
result, _ := engine.RenderString("{{ .tasks.processor.output.data }}", context)
// Result: []int{1, 2, 3, 4, 5}

// This returns a string
result, _ := engine.RenderString("{{ .tasks.processor.output.data | join \",\" }}", context)
// Result: "1,2,3,4,5"
```

### Precision Preservation

```go
// Enable precision preservation for large numbers
engine = engine.WithPrecisionPreservation(true)

context := map[string]any{
    "largeNumber": "12345678901234567890",
    "precision":   "123.456789012345678901234567890",
}

result, _ := engine.RenderString("{{ .largeNumber }}", context)
// Preserves as string to avoid precision loss

result, _ := engine.RenderString("{{ .precision }}", context)
// Converts to float64 only if no precision loss
```

---

## 📚 API Reference

### Core Types

#### `TemplateEngine`

Main template engine struct (thread-safe).

```go
type TemplateEngine struct {
    // Private fields
}

// Create new engine
func NewEngine(format EngineFormat) *TemplateEngine

// Configuration methods
func (e *TemplateEngine) WithFormat(format EngineFormat) *TemplateEngine
func (e *TemplateEngine) WithPrecisionPreservation(enabled bool) *TemplateEngine
```

#### `EngineFormat`

Output format enumeration.

```go
type EngineFormat string

const (
    FormatYAML EngineFormat = "yaml"
    FormatJSON EngineFormat = "json"
    FormatText EngineFormat = "text"
)
```

### Template Processing

#### `RenderString`

```go
func (e *TemplateEngine) RenderString(templateStr string, context map[string]any) (string, error)
```

Renders a template string with the given context.

**Parameters:**
- `templateStr`: Template string to render
- `context`: Template context data

**Returns:**
- `string`: Rendered result
- `error`: Rendering error if any

#### `ParseAny`

```go
func (e *TemplateEngine) ParseAny(value any, ctxData map[string]any) (any, error)
```

Processes a value and resolves any templates within it recursively.

**Parameters:**
- `value`: Value to process (can be string, map, slice, etc.)
- `ctxData`: Template context data

**Returns:**
- `any`: Processed value with templates resolved
- `error`: Processing error if any

#### `ProcessFile`

```go
func (e *TemplateEngine) ProcessFile(filePath string, context map[string]any) (string, error)
```

Processes a template file and returns the result.

**Parameters:**
- `filePath`: Path to template file
- `context`: Template context data

**Returns:**
- `string`: Processed file content
- `error`: Processing error if any

### Template Management

#### `AddTemplate`

```go
func (e *TemplateEngine) AddTemplate(name, templateStr string) error
```

Adds a named template to the engine.

#### `Render`

```go
func (e *TemplateEngine) Render(name string, context map[string]any) (string, error)
```

Renders a template by name.

#### `AddGlobalValue`

```go
func (e *TemplateEngine) AddGlobalValue(name string, value any) 
```

Adds a global value available in all template contexts.

### Utility Functions

#### `HasTemplate`

```go
func HasTemplate(template string) bool
```

Returns true if the string contains template markers.

#### `YAMLNodeToJSON`

```go
func YAMLNodeToJSON(node *yaml.Node) (string, error)
```

Converts a YAML node to JSON string.

#### `JSONToYAMLNode`

```go
func JSONToYAMLNode(jsonStr string) (*yaml.Node, error)
```

Converts a JSON string to YAML node.

### Value Conversion

#### `ValueConverter`

```go
type ValueConverter struct{}

func (c *ValueConverter) YAMLToJSON(node *yaml.Node) (any, error)
func (c *ValueConverter) JSONToYAML(value any) (*yaml.Node, error)
```

Provides methods to convert between YAML and JSON formats.

#### `PrecisionConverter`

```go
type PrecisionConverter struct{}

func NewPrecisionConverter() *PrecisionConverter
func (pc *PrecisionConverter) ConvertWithPrecision(value any) any
```

Handles numeric conversion with precision preservation.

---

## 🧪 Testing

### Unit Tests

```go
func TestTemplateEngine_RenderString(t *testing.T) {
    engine := tplengine.NewEngine(tplengine.FormatText)
    
    tests := []struct {
        name     string
        template string
        context  map[string]any
        expected string
        wantErr  bool
    }{
        {
            name:     "simple variable",
            template: "Hello {{ .name }}!",
            context:  map[string]any{"name": "World"},
            expected: "Hello World!",
            wantErr:  false,
        },
        {
            name:     "missing key error",
            template: "{{ .nonexistent }}",
            context:  map[string]any{},
            expected: "",
            wantErr:  true,
        },
        {
            name:     "safe key access",
            template: "{{ if hasKey . \"name\" }}{{ .name }}{{ else }}unknown{{ end }}",
            context:  map[string]any{},
            expected: "unknown",
            wantErr:  false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := engine.RenderString(tt.template, tt.context)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

### Integration Tests

```go
func TestTemplateEngine_ParseAny(t *testing.T) {
    engine := tplengine.NewEngine(tplengine.FormatYAML)
    
    data := map[string]any{
        "config": map[string]any{
            "host": "{{ .env.HOST }}",
            "port": "{{ .env.PORT | default \"8080\" }}",
        },
        "items": []any{
            "{{ .item1 }}",
            "{{ .item2 }}",
        },
    }
    
    context := map[string]any{
        "env": map[string]any{
            "HOST": "localhost",
            "PORT": "3000",
        },
        "item1": "first",
        "item2": "second",
    }
    
    result, err := engine.ParseAny(data, context)
    assert.NoError(t, err)
    
    resultMap := result.(map[string]any)
    config := resultMap["config"].(map[string]any)
    assert.Equal(t, "localhost", config["host"])
    assert.Equal(t, "3000", config["port"])
}
```

### Performance Tests

```go
func BenchmarkTemplateEngine_RenderString(b *testing.B) {
    engine := tplengine.NewEngine(tplengine.FormatText)
    template := "Hello {{ .name }}! Your age is {{ .age }}."
    context := map[string]any{
        "name": "John",
        "age":  30,
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := engine.RenderString(template, context)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

MIT License - see [LICENSE](../../LICENSE)
