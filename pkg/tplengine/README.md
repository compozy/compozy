# Template Engine Package

The `tplengine` package provides a powerful template processing engine for Compozy that uses Go
templates with enhanced functionality. It's designed to safely process dynamic configuration values
with strict error handling and comprehensive template features.

## Features

- **Go Template Processing**: Full Go template syntax support with `text/template`
- **Sprig Functions**: Complete integration with
  [Sprig template functions](https://masterminds.github.io/sprig/)
- **Strict Error Handling**: `missingkey=error` prevents silent failures and catches typos early
- **Multiple Output Formats**: Support for YAML, JSON, and plain text output
- **Value Conversion**: Seamless conversion between YAML and JSON formats
- **Object Reference Detection**: Smart handling of simple object references to preserve data types
- **Map Processing**: Recursive template processing for complex data structures

## Template Behavior

### Strict Error Handling with `missingkey=error`

The template engine is configured with `missingkey=error`, which means **any attempt to access a
non-existent key will immediately fail** rather than silently rendering `<no value>`. This helps
catch configuration errors early.

```go
// ❌ This will FAIL if .user.age doesn't exist
"{{ .user.age | default \"25\" }}"

// ✅ This works - check existence first
"{{ if hasKey .user \"age\" }}{{ .user.age }}{{ else }}25{{ end }}"

// ✅ This works - ensure the key exists in your context
context := map[string]any{
    "user": map[string]any{
        "age": 25, // Always provide the key
    },
}
```

### Why `missingkey=error`?

This strict behavior helps you catch:

1. **Typos**: `{{ .worklow.id }}` (should be `workflow`)
2. **Missing configuration**: `{{ .tasks.nonexistent.output }}`
3. **Schema mismatches**: `{{ .user.invalid_field }}`

The trade-off is that you must be explicit about handling missing data, but this prevents bugs from
silently slipping into production.

## Basic Usage

### Creating a Template Engine

```go
import "github.com/compozy/compozy/pkg/tplengine"

// Create engine with specific output format
engine := tplengine.NewEngine(tplengine.FormatYAML)

// Or change format later
engine = engine.WithFormat(tplengine.FormatJSON)
```

### Rendering Templates

```go
// Simple string rendering
result, err := engine.RenderString("Hello {{ .name }}!", map[string]any{
    "name": "World",
})
// Result: "Hello World!"

// Check if string contains templates first
if tplengine.HasTemplate("{{ .name }}") {
    // Process template
}
```

### Processing Files

```go
// Process a template file
result, err := engine.ProcessFile("config.yaml", context)
if err != nil {
    log.Fatal(err)
}

// Access different format outputs
fmt.Println("Text:", result.Text)
fmt.Println("YAML:", result.YAML)
fmt.Println("JSON:", result.JSON)
```

## Output Formats

The engine supports three output formats:

```go
const (
    FormatYAML EngineFormat = "yaml"  // Parse as YAML
    FormatJSON EngineFormat = "json"  // Parse as JSON  
    FormatText EngineFormat = "text"  // Plain text only
)
```

### ProcessResult Structure

```go
type ProcessResult struct {
    Text string  // Always available - the rendered template
    YAML any     // Available when format is FormatYAML
    JSON any     // Available when format is FormatJSON
}
```

## Template Syntax

### Basic Variables

```go
context := map[string]any{
    "name": "John",
    "age":  30,
}

// Simple variable access
"Hello {{ .name }}!"                    // "Hello John!"
"Age: {{ .age }}"                       // "Age: 30"
```

### Nested Object Access

```go
context := map[string]any{
    "user": map[string]any{
        "profile": map[string]any{
            "name": "John",
            "email": "john@example.com",
        },
    },
}

// Nested access
"Name: {{ .user.profile.name }}"        // "Name: John"
"Email: {{ .user.profile.email }}"     // "Email: john@example.com"
```

### Safe Key Access

Since `missingkey=error` is enabled, always check for key existence:

```go
// ✅ Safe approaches
"{{ if hasKey .user \"age\" }}{{ .user.age }}{{ else }}unknown{{ end }}"
"{{ if .user.age }}{{ .user.age }}{{ else }}0{{ end }}"  // if age might be nil/zero

// ❌ Unsafe - will fail if key doesn't exist
"{{ .user.missing_field | default \"fallback\" }}"
```

## Sprig Functions

The engine includes all Sprig template functions. Here are common examples:

### String Functions

```go
// String manipulation
"{{ .name | upper }}"                   // Uppercase
"{{ .name | lower }}"                   // Lowercase
"{{ .text | trim }}"                    // Trim whitespace
"{{ .text | replace \" \" \"_\" }}"     // Replace spaces with underscores

// String testing
"{{ contains \"world\" .message }}"     // Check if contains substring
"{{ hasPrefix \"Hello\" .message }}"    // Check prefix
"{{ hasSuffix \"!\" .message }}"        // Check suffix
```

### Conditional Logic

```go
// If-else statements
"{{ if eq .status \"active\" }}Running{{ else }}Stopped{{ end }}"

// Comparisons
"{{ if gt .count 10 }}Many{{ else }}Few{{ end }}"
"{{ if and .enabled (eq .status \"ready\") }}Available{{ end }}"

// Default values (only works if key exists)
"{{ .timeout | default \"30\" }}"
```

### Data Manipulation

```go
// Arrays/slices
"{{ len .items }}"                      // Length
"{{ index .items 0 }}"                  // First item

// JSON/YAML conversion
"{{ .config | toJson }}"                // Convert to JSON
"{{ .data | toYaml }}"                  // Convert to YAML

// Math operations
"{{ add .count 1 }}"                    // Addition
"{{ sub .total .used }}"                // Subtraction
```

### Date and Time

```go
// Current time
"{{ now }}"                             // Current timestamp
"{{ now | date \"2006-01-02\" }}"       // Formatted date

// Duration
"{{ duration \"1h30m\" }}"              // Parse duration
```

## Advanced Features

### Map Processing

The `ParseMap` method recursively processes template expressions in complex data structures:

```go
data := map[string]any{
    "config": map[string]any{
        "host": "{{ .env.DATABASE_HOST }}",
        "port": "{{ .env.DATABASE_PORT | default \"5432\" }}",
        "nested": map[string]any{
            "timeout": "{{ .settings.timeout }}",
        },
    },
    "items": []any{
        "{{ .item1 }}",
        "{{ .item2 }}",
    },
}

result, err := engine.ParseMap(data, context)
```

### Object Reference Detection

The engine can detect simple object references and preserve their original data types:

```go
// Simple object reference (preserves type)
"{{ .tasks.processor.output.data }}"

// Complex expression (returns string)
"{{ .tasks.processor.output.data | upper }}"
```

### Global Values

Add values that are available in all template contexts:

```go
engine.AddGlobalValue("version", "1.0.0")
engine.AddGlobalValue("environment", "production")

// Now available in all templates
"Version: {{ .version }}"               // "Version: 1.0.0"
```

### Template Management

```go
// Add named templates
err := engine.AddTemplate("greeting", "Hello {{ .name }}!")

// Render by name
result, err := engine.Render("greeting", context)
```

## Value Conversion

The package includes utilities for converting between YAML and JSON:

```go
// Convert YAML node to JSON
jsonStr, err := tplengine.YAMLNodeToJSON(yamlNode)

// Convert JSON string to YAML node
yamlNode, err := tplengine.JSONToYAMLNode(jsonStr)

// Value converter for complex conversions
converter := &tplengine.ValueConverter{}
jsonValue, err := converter.YAMLToJSON(yamlNode)
yamlNode, err := converter.JSONToYAML(jsonValue)
```

## Error Handling

### Template Parsing Errors

```go
// Invalid template syntax
_, err := engine.RenderString("{{ .name !", context)
// Error: template parsing error

// Missing closing brace
_, err := engine.RenderString("{{ .name ", context)
// Error: template parsing error
```

### Missing Key Errors

```go
// Missing key with missingkey=error
_, err := engine.RenderString("{{ .nonexistent }}", map[string]any{})
// Error: template execution error: map has no entry for key "nonexistent"

// Nested missing key
_, err := engine.RenderString("{{ .user.missing }}", map[string]any{
    "user": map[string]any{"name": "John"},
})
// Error: template execution error: map has no entry for key "missing"
```

### Safe Error Handling Patterns

```go
// Always validate context before templating
func safeRender(templateStr string, context map[string]any) (string, error) {
    // Validate required keys exist
    if _, ok := context["user"]; !ok {
        return "", fmt.Errorf("required key 'user' missing from context")
    }
    
    return engine.RenderString(templateStr, context)
}

// Use conditional templates
template := `
{{ if hasKey . "optional" }}
Optional value: {{ .optional }}
{{ else }}
No optional value provided
{{ end }}
`
```

## Best Practices

### 1. Always Provide Complete Context

```go
// ✅ Good: Provide all required keys
context := map[string]any{
    "user": map[string]any{
        "name":  "John",
        "email": "john@example.com",
        "age":   30,  // Always provide expected keys
    },
    "settings": map[string]any{
        "timeout": "30s",
        "debug":   false,
    },
}
```

### 2. Use Defensive Templates

```go
// ✅ Good: Check for existence
"{{ if hasKey .user \"age\" }}Age: {{ .user.age }}{{ end }}"

// ✅ Good: Provide fallbacks
"{{ if .user.age }}{{ .user.age }}{{ else }}unknown{{ end }}"

// ❌ Avoid: Assuming keys exist
"{{ .user.age | default \"25\" }}"  // Fails if .user.age doesn't exist
```

### 3. Handle Different Data Types

```go
// Boolean handling
"{{ if .enabled }}active{{ else }}inactive{{ end }}"

// Number formatting
"{{ printf \"%.2f\" .price }}"

// String operations
"{{ .name | default \"unnamed\" | title }}"
```

### 4. Use Meaningful Error Messages

```go
// Wrap template errors with context
result, err := engine.RenderString(template, context)
if err != nil {
    return fmt.Errorf("failed to render config template for %s: %w", componentID, err)
}
```

### 5. Validate Templates During Development

```go
// Test templates with sample data
func TestTemplates(t *testing.T) {
    engine := tplengine.NewEngine(tplengine.FormatText)
    
    testCases := []struct {
        template string
        context  map[string]any
        expected string
    }{
        {
            template: "Hello {{ .name }}!",
            context:  map[string]any{"name": "World"},
            expected: "Hello World!",
        },
    }
    
    for _, tc := range testCases {
        result, err := engine.RenderString(tc.template, tc.context)
        assert.NoError(t, err)
        assert.Equal(t, tc.expected, result)
    }
}
```

## Integration with Compozy

The template engine is used throughout Compozy for:

- **Configuration normalization**: Processing dynamic values in YAML configurations
- **Runtime value resolution**: Resolving template expressions with workflow context
- **Cross-component communication**: Allowing components to reference each other's outputs
- **Environment variable substitution**: Processing environment-specific configurations

See the [normalizer package](../normalizer/README.md) for detailed examples of how templates are
used in Compozy configurations.

## Migration from `<no value>` Behavior

If you're migrating from a system that allowed `<no value>` fallbacks:

### Before (with silent failures)

```yaml
config:
    host: "{{ .database.host | default \"localhost\" }}" # Silent if .database missing
    port: "{{ .database.port | default \"5432\" }}" # Silent if .database missing
```

### After (with missingkey=error)

```yaml
config:
    # Option 1: Ensure keys exist in context
    host: "{{ .database.host | default \"localhost\" }}" # Requires .database to exist
    port: "{{ .database.port | default \"5432\" }}" # Requires .database to exist

    # Option 2: Use conditional checks
    host: "{{ if hasKey . \"database\" }}{{ .database.host | default \"localhost\" }}{{ else }}localhost{{ end }}"
    port: "{{ if hasKey . \"database\" }}{{ .database.port | default \"5432\" }}{{ else }}5432{{ end }}"
```

The `missingkey=error` behavior encourages better configuration practices by making missing data
explicit rather than silently failing.
