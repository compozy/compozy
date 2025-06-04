package tplengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/compozy/compozy/engine/core"
	"gopkg.in/yaml.v3"
)

// EngineFormat represents the format of the template engine output
type EngineFormat string

const (
	// FormatYAML represents YAML output format
	FormatYAML EngineFormat = "yaml"
	// FormatJSON represents JSON output format
	FormatJSON EngineFormat = "json"
	// FormatText represents plain text output format
	FormatText EngineFormat = "text"
)

// TemplateEngine is the main template engine struct
type TemplateEngine struct {
	templates    map[string]*template.Template
	globalValues map[string]any
	format       EngineFormat
}

// ProcessResult contains the result of processing a template
type ProcessResult struct {
	Text string
	YAML any
	JSON any
}

// NewEngine creates a new template engine with the specified format
func NewEngine(format EngineFormat) *TemplateEngine {
	return &TemplateEngine{
		templates:    make(map[string]*template.Template),
		globalValues: make(map[string]any),
		format:       format,
	}
}

// WithFormat returns a new engine with the specified format
func (e *TemplateEngine) WithFormat(format EngineFormat) *TemplateEngine {
	e.format = format
	return e
}

// AddTemplate adds a template to the engine
func (e *TemplateEngine) AddTemplate(name, templateStr string) error {
	tmpl, err := template.New(name).Option("missingkey=error").Funcs(sprig.FuncMap()).Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	e.templates[name] = tmpl
	return nil
}

// HasTemplate returns true if the template contains template markers
func HasTemplate(template string) bool {
	return strings.Contains(template, "{{") || strings.Contains(template, "{{-")
}

// Render renders a template by name
func (e *TemplateEngine) Render(name string, context map[string]any) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	return e.renderTemplate(tmpl, context)
}

// RenderString renders a template string
func (e *TemplateEngine) RenderString(templateStr string, context map[string]any) (string, error) {
	// If no template markers, return the template as is
	if !HasTemplate(templateStr) {
		return templateStr, nil
	}

	// Create a new template and parse the string
	tmpl, err := template.New("inline").Option("missingkey=error").Funcs(sprig.FuncMap()).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	return e.renderTemplate(tmpl, context)
}

// renderTemplate renders a parsed template with the given context
func (e *TemplateEngine) renderTemplate(tmpl *template.Template, context map[string]any) (string, error) {
	processedContext := e.preprocessContext(context)
	maps.Copy(processedContext, e.globalValues)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, processedContext); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// ProcessString processes a template string and returns the result
func (e *TemplateEngine) ProcessString(templateStr string, context map[string]any) (*ProcessResult, error) {
	rendered, err := e.RenderString(templateStr, context)
	if err != nil {
		return nil, err
	}

	result := &ProcessResult{
		Text: rendered,
	}

	// Parse the result based on the format
	switch e.format {
	case FormatYAML:
		var yamlObj any
		err = yaml.Unmarshal([]byte(rendered), &yamlObj)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		result.YAML = yamlObj
	case FormatJSON:
		var jsonObj any
		err = json.Unmarshal([]byte(rendered), &jsonObj)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		result.JSON = jsonObj
	}

	return result, nil
}

// ProcessFile processes a template file and returns the result
func (e *TemplateEngine) ProcessFile(filePath string, context map[string]any) (*ProcessResult, error) {
	// Read the template file
	templateBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	// Determine format from file extension if not specified
	if e.format == "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".yaml", ".yml":
			e.format = FormatYAML
		case ".json":
			e.format = FormatJSON
		default:
			e.format = FormatText
		}
	}

	// Process the template
	return e.ProcessString(string(templateBytes), context)
}

// ParseMap processes a value and resolves any templates within it
func (e *TemplateEngine) ParseMap(value any, data map[string]any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return e.parseStringValue(v, data)
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			parsedVal, err := e.ParseMap(val, data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template in map key %s: %w", k, err)
			}
			result[k] = parsedVal
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			parsedVal, err := e.ParseMap(val, data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template in array index %d: %w", i, err)
			}
			result[i] = parsedVal
		}
		return result, nil
	default:
		// Convert boolean values to strings for non-template cases
		if boolVal, ok := v.(bool); ok {
			return fmt.Sprintf("%t", boolVal), nil
		}
		// For other types (int, float, etc.), return as is
		return v, nil
	}
}

func (e *TemplateEngine) ParseMapWithFilter(value any, data map[string]any, filter func(k string) bool) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return e.parseStringValue(v, data)
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			if filter != nil && filter(k) {
				result[k] = val
				continue
			}
			parsedVal, err := e.ParseMapWithFilter(val, data, filter)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template in map key %s: %w", k, err)
			}
			result[k] = parsedVal
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			if filter != nil && filter(strconv.Itoa(i)) {
				result[i] = val
				continue
			}
			parsedVal, err := e.ParseMapWithFilter(val, data, filter)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template in array index %d: %w", i, err)
			}
			result[i] = parsedVal
		}
		return result, nil
	default:
		// Convert boolean values to strings for non-template cases
		if boolVal, ok := v.(bool); ok {
			return fmt.Sprintf("%t", boolVal), nil
		}
		// For other types (int, float, etc.), return as is
		return v, nil
	}
}

// parseStringValue handles parsing of string values that may contain templates
func (e *TemplateEngine) parseStringValue(v string, data map[string]any) (any, error) {
	if !HasTemplate(v) {
		return v, nil
	}
	// Handle simple object references to preserve object types
	if e.isSimpleObjectReference(v) {
		if obj := e.extractObjectFromContext(v, data); obj != nil {
			// Convert boolean values to strings when returning objects
			if boolVal, ok := obj.(bool); ok {
				return fmt.Sprintf("%t", boolVal), nil
			}
			return obj, nil
		}
	}
	parsed, err := e.RenderString(v, data)
	if err != nil {
		return nil, err
	}
	// Convert boolean results from template rendering to strings
	if parsed == "true" || parsed == "false" {
		return parsed, nil
	}
	// Check if the parsed result is a JSON-like string and try to parse it
	if strings.HasPrefix(parsed, "{") || strings.HasPrefix(parsed, "[") {
		var jsonObj any
		if json.Unmarshal([]byte(parsed), &jsonObj) == nil {
			return jsonObj, nil
		}
	}
	return parsed, nil
}

// isSimpleObjectReference checks if a template string is a simple object reference
func (e *TemplateEngine) isSimpleObjectReference(template string) bool {
	// Check if it's a simple reference like {{ .tasks.something.output.data }}
	trimmed := strings.TrimSpace(template)
	hasTemplateMarkers := strings.HasPrefix(trimmed, "{{") &&
		strings.HasSuffix(trimmed, "}}") &&
		strings.Count(trimmed, "{{") == 1 &&
		strings.Count(trimmed, "}}") == 1
	hasNoFilters := !strings.Contains(trimmed, "|")
	hasObjectPath := strings.Contains(trimmed, ".output.") ||
		strings.Contains(trimmed, ".input.") ||
		strings.Contains(trimmed, ".tasks.")
	return hasTemplateMarkers && hasNoFilters && hasObjectPath
}

// extractObjectFromContext tries to extract an object directly from the context
func (e *TemplateEngine) extractObjectFromContext(template string, data map[string]any) any {
	// Extract the path from the template
	template = strings.TrimSpace(template)
	if !strings.HasPrefix(template, "{{") || !strings.HasSuffix(template, "}}") {
		return nil
	}

	path := strings.TrimSpace(template[2 : len(template)-2])
	if !strings.HasPrefix(path, ".") { // Path must start with . like {{ .foo }}
		return nil
	}

	path = path[1:] // Remove leading dot
	parts := strings.Split(path, ".")

	var currentAny any = data
	for _, part := range parts {
		var currentMap map[string]any
		var traversable bool

		switch c := currentAny.(type) {
		case map[string]any:
			currentMap = c
			traversable = true
		case *map[string]any:
			if c != nil {
				currentMap = *c
				traversable = true
			}
		case *core.Input: // core.Input is map[string]any
			if c != nil {
				currentMap = *c
				traversable = true
			}
		case *core.Output: // core.Output is map[string]any
			if c != nil {
				currentMap = *c
				traversable = true
			}
		}

		if !traversable {
			return nil // Cannot traverse
		}

		val, exists := currentMap[part]
		if !exists {
			return nil
		}
		currentAny = val
	}
	return currentAny
}

// AddGlobalValue adds a global value to the template engine
func (e *TemplateEngine) AddGlobalValue(name string, value any) {
	e.globalValues[name] = value
}

// preprocessContext adds default fields to the context and ensures proper boolean handling
func (e *TemplateEngine) preprocessContext(ctx map[string]any) map[string]any {
	if ctx == nil {
		ctx = make(map[string]any)
	}

	result := make(map[string]any)
	maps.Copy(result, ctx)

	// Add default fields if they don't exist
	if _, ok := result["env"]; !ok {
		result["env"] = make(map[string]string)
	}
	if _, ok := result["input"]; !ok {
		result["input"] = make(map[string]any)
	}
	if _, ok := result["output"]; !ok {
		result["output"] = nil
	}
	if _, ok := result["trigger"]; !ok {
		result["trigger"] = make(map[string]any)
	}
	if _, ok := result["tools"]; !ok {
		result["tools"] = make(map[string]any)
	}
	if _, ok := result["tasks"]; !ok {
		result["tasks"] = make(map[string]any)
	}
	if _, ok := result["agents"]; !ok {
		result["agents"] = make(map[string]any)
	}

	return result
}
