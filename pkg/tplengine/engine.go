package tplengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
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
	// Preprocess template to handle hyphens in field names
	processedTemplate := e.preprocessTemplateForHyphens(templateStr)

	tmpl, err := template.New(name).Option("missingkey=error").Funcs(sprig.FuncMap()).Parse(processedTemplate)
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

	// Preprocess template to handle hyphens in field names
	processedTemplate := e.preprocessTemplateForHyphens(templateStr)

	// Create a new template and parse the string
	tmpl, err := template.New("inline").Option("missingkey=error").Funcs(sprig.FuncMap()).Parse(processedTemplate)
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
		return e.parseMapType(v, data)
	case core.Output:
		return e.parseMapType(map[string]any(v), data)
	case core.Input:
		return e.parseMapType(map[string]any(v), data)
	case []any:
		return e.parseArrayType(v, data)
	default:
		// For other types (int, float, bool, etc.), return as is
		return v, nil
	}
}

// parseMapType handles parsing of map-like types
func (e *TemplateEngine) parseMapType(m map[string]any, data map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(m))
	for k, val := range m {
		parsedVal, err := e.ParseMap(val, data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template in map key %s: %w", k, err)
		}
		result[k] = parsedVal
	}
	return result, nil
}

// parseArrayType handles parsing of array types
func (e *TemplateEngine) parseArrayType(arr []any, data map[string]any) ([]any, error) {
	result := make([]any, len(arr))
	for i, val := range arr {
		parsedVal, err := e.ParseMap(val, data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template in array index %d: %w", i, err)
		}
		result[i] = parsedVal
	}
	return result, nil
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
			return e.prepareValueForTemplate(obj)
		}
	}

	return e.renderAndProcessTemplate(v, data)
}

func (e *TemplateEngine) prepareValueForTemplate(obj any) (any, error) {
	// For simple object references, preserve the original type
	// Only convert to string when necessary for template processing
	switch val := obj.(type) {
	case *core.Output:
		// Dereference core.Output to map[string]any for template processing
		if val != nil {
			return *val, nil
		}
		return nil, nil
	case core.Output:
		// Return core.Output as map[string]any for template processing
		return map[string]any(val), nil
	default:
		// Return the value as-is to preserve its type
		return obj, nil
	}
}

func (e *TemplateEngine) renderAndProcessTemplate(v string, data map[string]any) (any, error) {
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

	if !hasTemplateMarkers {
		return false
	}

	// Extract the content between {{ and }}
	content := strings.TrimSpace(trimmed[2 : len(trimmed)-2])

	// Must start with a dot and have no spaces or special template functions
	hasNoFilters := !strings.Contains(content, "|") && !strings.Contains(content, " ")
	hasObjectPath := strings.HasPrefix(content, ".") && strings.Contains(content, ".")

	return hasNoFilters && hasObjectPath
}

// extractObjectFromContext tries to extract an object directly from the context
func (e *TemplateEngine) extractObjectFromContext(template string, data map[string]any) any {
	path := e.extractPathFromTemplate(template)
	if path == "" {
		return nil
	}

	parts := strings.Split(path, ".")
	return e.traverseObjectPath(data, parts)
}

func (e *TemplateEngine) extractPathFromTemplate(template string) string {
	// Extract the path from the template
	template = strings.TrimSpace(template)
	if !strings.HasPrefix(template, "{{") || !strings.HasSuffix(template, "}}") {
		return ""
	}

	path := strings.TrimSpace(template[2 : len(template)-2])
	if !strings.HasPrefix(path, ".") { // Path must start with . like {{ .foo }}
		return ""
	}

	return path[1:] // Remove leading dot
}

func (e *TemplateEngine) traverseObjectPath(data map[string]any, parts []string) any {
	var currentAny any = data
	for _, part := range parts {
		if part == "" {
			continue // Skip empty parts from double dots
		}

		currentMap, traversable := e.extractTraversableMap(currentAny)
		if !traversable {
			return nil // Cannot traverse
		}

		val, exists := currentMap[part]
		if !exists {
			return nil
		}
		currentAny = val
	}

	// Return the final value preserving its original type
	return currentAny
}

func (e *TemplateEngine) extractTraversableMap(currentAny any) (map[string]any, bool) {
	switch c := currentAny.(type) {
	case map[string]any:
		return c, true
	case *map[string]any:
		if c != nil {
			return *c, true
		}
	case *core.Input: // core.Input is map[string]any
		if c != nil {
			return *c, true
		}
	case *core.Output: // core.Output is map[string]any
		if c != nil {
			return *c, true
		}
	case core.Input: // Direct core.Input (not pointer)
		return c, true
	case core.Output: // Direct core.Output (not pointer)
		return c, true
	}
	return nil, false
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

// preprocessTemplateForHyphens converts template expressions with hyphens to use index syntax
// This function processes dot-path expressions within templates, even inside conditionals
func (e *TemplateEngine) preprocessTemplateForHyphens(templateStr string) string {
	// Find all template expressions
	re := regexp.MustCompile(`{{[^}]*}}`)

	return re.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Extract the content between {{ and }}
		content := strings.TrimSpace(match[2 : len(match)-2])

		// Find dot-path patterns that contain hyphens
		// This regex looks for dot-paths that may contain hyphens
		pathPattern := regexp.MustCompile(`(\.[a-zA-Z_][a-zA-Z0-9_-]*(?:\.[a-zA-Z_][a-zA-Z0-9_-]*)*)`)

		processedContent := pathPattern.ReplaceAllStringFunc(content, func(pathMatch string) string {
			// Only process if it contains hyphens
			if !strings.Contains(pathMatch, "-") {
				return pathMatch
			}

			// Split the path into segments
			pathSegments := strings.Split(pathMatch[1:], ".") // Remove leading dot

			// Convert entire path to index syntax for safety
			result := "index ."
			for _, segment := range pathSegments {
				result += fmt.Sprintf(` %q`, segment)
			}
			return "(" + result + ")"
		})

		return "{{ " + processedContent + " }}"
	})
}
