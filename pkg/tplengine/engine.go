package tplengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	html_template "html/template"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/compozy/compozy/engine/core"
)

// ErrMissingKey is returned (wrapped) when a template references a
// non-existent map key while running with missingkey=error.
// Callers can use errors.Is(err, ErrMissingKey) to detect this case
// instead of matching error strings.
var ErrMissingKey = errors.New("tplengine: missing key")

const (
	trueString  = "true"
	falseString = "false"
)

// Pre-compiled regular expressions for template processing performance
var (
	templateExpressionRegex = regexp.MustCompile(`{{[^}]*}}`)
	hyphenatedPathRegex     = regexp.MustCompile(`(\.[a-zA-Z_][a-zA-Z0-9_-]*(?:\.[a-zA-Z_][a-zA-Z0-9_-]*)*)`)
	taskRefRe               = regexp.MustCompile(`\.tasks\.([a-zA-Z0-9_-]+)(?:\.|\[|\s|}|$)`)
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
// It is safe for concurrent use by multiple goroutines
type TemplateEngine struct {
	mu                       sync.RWMutex
	templates                map[string]*template.Template
	globalValues             map[string]any
	format                   EngineFormat
	preserveNumericPrecision bool
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
	e.mu.Lock()
	e.format = format
	e.mu.Unlock()
	return e
}

// WithPrecisionPreservation enables or disables numeric precision preservation
func (e *TemplateEngine) WithPrecisionPreservation(enabled bool) *TemplateEngine {
	e.mu.Lock()
	e.preserveNumericPrecision = enabled
	e.mu.Unlock()
	return e
}

// getFuncMap returns the function map for templates, including HTML safety functions
//
// Security functions available in templates:
//   - htmlEscape: Escapes HTML content to prevent XSS (e.g., {{ .userInput | htmlEscape }})
//   - htmlAttrEscape: Escapes HTML attribute values (e.g., <div title="{{ .title | htmlAttrEscape }}">)
//   - jsEscape: Escapes JavaScript strings (e.g., <script>var x = '{{ .data | jsEscape }}';</script>)
//
// IMPORTANT: Always use these functions when rendering user input or any untrusted data
// in HTML contexts to prevent XSS vulnerabilities.
func (e *TemplateEngine) getFuncMap() template.FuncMap {
	// Start with sprig functions
	funcMap := sprig.FuncMap()
	// Add HTML safety functions with comprehensive XSS protection
	funcMap["htmlEscape"] = html.EscapeString
	funcMap["htmlAttrEscape"] = html.EscapeString // For attribute values
	funcMap["jsEscape"] = html_template.JSEscapeString
	return funcMap
}

// AddTemplate adds a template to the engine
func (e *TemplateEngine) AddTemplate(name, templateStr string) error {
	// Preprocess template to handle hyphens in field names
	processedTemplate := e.preprocessTemplateForHyphens(templateStr)
	tmpl, err := template.New(name).Option("missingkey=error").Funcs(e.getFuncMap()).Parse(processedTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	e.mu.Lock()
	e.templates[name] = tmpl
	e.mu.Unlock()
	return nil
}

// HasTemplate returns true if the template contains template markers
func HasTemplate(template string) bool {
	return strings.Contains(template, "{{") || strings.Contains(template, "{{-")
}

// Render renders a template by name
func (e *TemplateEngine) Render(name string, context map[string]any) (string, error) {
	e.mu.RLock()
	tmpl, ok := e.templates[name]
	e.mu.RUnlock()
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
	tmpl, err := template.New("inline").Option("missingkey=error").Funcs(e.getFuncMap()).Parse(processedTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}
	return e.renderTemplate(tmpl, context)
}

// renderTemplate renders a parsed template with the given context
func (e *TemplateEngine) renderTemplate(tmpl *template.Template, context map[string]any) (string, error) {
	processedContext := e.preprocessContext(context)
	// Thread-safe access to global values - merge in global values
	e.mu.RLock()
	processedContext = core.CopyMaps(processedContext, e.globalValues)
	e.mu.RUnlock()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, processedContext); err != nil {
		if isExecMissingKey(err) {
			return "", fmt.Errorf("template execution error: %w", ErrMissingKey)
		}
		return "", fmt.Errorf("template execution error: %w", err)
	}
	return buf.String(), nil
}

// isExecMissingKey reports whether err came from text/template execution due
// to a missing map key (with missingkey=error). The Go templates library does
// not expose a typed error for this case, so we conservatively detect common
// messages and centralize the logic here.
func isExecMissingKey(err error) bool {
	if err == nil {
		return false
	}
	var execErr *template.ExecError
	if errors.As(err, &execErr) && execErr.Err != nil {
		msg := execErr.Err.Error()
		return strings.Contains(msg, "map has no entry for key") || strings.Contains(msg, "missingkey") ||
			strings.Contains(msg, "missing key")
	}
	// Some callers wrap the ExecError; fall back to message scan.
	msg := err.Error()
	return strings.Contains(msg, "map has no entry for key") || strings.Contains(msg, "missingkey") ||
		strings.Contains(msg, "missing key")
}

// ProcessString processes a template string and returns the result
func (e *TemplateEngine) ProcessString(templateStr string, context map[string]any) (string, error) {
	result, err := e.ParseAny(templateStr, context)
	if err != nil {
		return "", fmt.Errorf("failed to parse template string: %w", err)
	}
	value, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("failed to parse template string: %w", err)
	}
	return value, nil
}

// ProcessFile processes a template file and returns the result
func (e *TemplateEngine) ProcessFile(filePath string, context map[string]any) (string, error) {
	// Read the template file
	templateBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}
	// Determine format from file extension if not specified
	e.mu.RLock()
	currentFormat := e.format
	e.mu.RUnlock()
	if currentFormat == "" {
		e.mu.Lock()
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".yaml", ".yml":
			e.format = FormatYAML
		case ".json":
			e.format = FormatJSON
		default:
			e.format = FormatText
		}
		e.mu.Unlock()
	}
	// Process the template
	return e.ProcessString(string(templateBytes), context)
}

// ParseAny processes a value and resolves any templates within it
func (e *TemplateEngine) ParseAny(value any, ctxData map[string]any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return e.parseStringValue(v, ctxData)
	case map[string]any:
		return e.parseMapType(v, ctxData)
	case core.Output:
		return e.parseMapType(map[string]any(v), ctxData)
	case core.Input:
		return e.parseMapType(map[string]any(v), ctxData)
	case []any:
		return e.parseArrayType(v, ctxData)
	default:
		// For other types (int, float, bool, etc.), return as is
		return v, nil
	}
}

// parseMapType handles parsing of map-like types
func (e *TemplateEngine) parseMapType(m map[string]any, data map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(m))
	for k, val := range m {
		parsedVal, err := e.ParseAny(val, data)
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
		parsedVal, err := e.ParseAny(val, data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template in array index %d: %w", i, err)
		}
		result[i] = parsedVal
	}
	return result, nil
}

// containsRuntimeReferences checks if a string contains runtime-only template references
// containsRuntimeReferences reports whether the input string contains runtime-only
// task references that should be deferred until execution (specifically the
// ".tasks." path segment). Returns true if such a reference is present.
func containsRuntimeReferences(s string) bool {
	// Only check for task outputs which are truly runtime-only
	// .item and .index can be either runtime collection variables OR normal context variables
	// so we shouldn't block them here - let the template engine try to resolve them
	return strings.Contains(s, ".tasks.")
}

// extractTaskReferences extracts all task IDs referenced in a template string
// extractTaskReferences returns the task IDs referenced in s.
// It scans s using the internal `taskRefRe` regular expression for patterns like
// `.tasks.TASKID` and returns the captured TASKID values in order of appearance.
// If no task references are found, an empty slice is returned.
func extractTaskReferences(s string) []string {
	taskIDs := []string{}
	// Match patterns like .tasks.TASKID.
	matches := taskRefRe.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if len(match) > 1 {
			taskIDs = append(taskIDs, match[1])
		}
	}
	return taskIDs
}

// areAllTasksAvailable reports whether all task IDs in taskIDs exist as keys in tasksMap.
// Returns true if every id is present (and true for an empty taskIDs slice).
func areAllTasksAvailable(taskIDs []string, tasksMap map[string]any) bool {
	for _, taskID := range taskIDs {
		if _, exists := tasksMap[taskID]; !exists {
			return false
		}
	}
	return true
}

func (e *TemplateEngine) ParseMapWithFilter(value any, data map[string]any, filter func(k string) bool) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return e.parseStringWithFilter(v, data)
	case map[string]any:
		return e.parseMapWithFilter(v, data, filter)
	case []any:
		return e.parseSliceWithFilter(v, data, filter)
	default:
		return v, nil
	}
}

// parseStringWithFilter handles string values that may reference runtime-only task placeholders.
// Contract: When task references cannot be resolved (e.g., tasks not yet available),
// the original template string is returned unchanged for deferred resolution by upstream callers.
func (e *TemplateEngine) parseStringWithFilter(v string, data map[string]any) (any, error) {
	if HasTemplate(v) && containsRuntimeReferences(v) {
		if !e.canResolveTaskReferencesNow(v, data) {
			// Return unresolved template for downstream resolution
			return v, nil
		}
	}
	return e.parseStringValue(v, data)
}

// canResolveTaskReferencesNow checks whether all referenced tasks exist in the provided context.
func (e *TemplateEngine) canResolveTaskReferencesNow(v string, data map[string]any) bool {
	if data == nil {
		return false
	}
	tasksVal, ok := data["tasks"]
	if !ok || tasksVal == nil {
		return false
	}
	// Handle pointer cases first, then value cases
	var tasksMap map[string]any
	switch t := tasksVal.(type) {
	case *map[string]any:
		if t != nil {
			tasksMap = *t
		}
	case *core.Input:
		if t != nil {
			tasksMap = *t
		}
	case *core.Output:
		if t != nil {
			tasksMap = *t
		}
	case map[string]any:
		tasksMap = t
	case core.Input:
		tasksMap = t
	case core.Output:
		tasksMap = t
	default:
		return false // unsupported type – cannot resolve yet
	}
	if tasksMap == nil {
		return false
	}
	referenced := extractTaskReferences(v)
	return areAllTasksAvailable(referenced, tasksMap)
}

// parseMapWithFilter parses maps while honoring the provided filter function.
func (e *TemplateEngine) parseMapWithFilter(
	m map[string]any,
	data map[string]any,
	filter func(k string) bool,
) (map[string]any, error) {
	result := make(map[string]any, len(m))
	for k, val := range m {
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
}

// parseSliceWithFilter parses arrays while honoring the provided filter function.
func (e *TemplateEngine) parseSliceWithFilter(
	arr []any,
	data map[string]any,
	filter func(k string) bool,
) ([]any, error) {
	result := make([]any, len(arr))
	for i, val := range arr {
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
}

// ParseWithJSONHandling processes a value, resolves templates, and handles JSON parsing for strings
// This method is similar to ParseAny but with special handling for JSON strings
func (e *TemplateEngine) ParseWithJSONHandling(value any, data map[string]any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		// Check if it's a template
		if HasTemplate(v) {
			processed, err := e.ParseAny(v, data)
			if err != nil {
				return nil, err
			}
			// If the result is a string that looks like JSON, parse it
			if str, ok := processed.(string); ok && str != "" && (str[0] == '{' || str[0] == '[') {
				var parsed any
				if err := json.Unmarshal([]byte(str), &parsed); err == nil {
					// Now process any templates in the parsed JSON
					return e.ParseAny(parsed, data)
				}
			}
			return processed, nil
		}
		// Try to parse as JSON if it looks like JSON
		if v != "" && (v[0] == '{' || v[0] == '[') {
			var parsed any
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				// Now process any templates in the parsed JSON
				return e.ParseAny(parsed, data)
			}
		}
		return v, nil
	default:
		// For other types, use regular ParseAny
		return e.ParseAny(v, data)
	}
}

// parseStringValue handles parsing of string values that may contain templates
func (e *TemplateEngine) parseStringValue(v string, data map[string]any) (any, error) {
	if !HasTemplate(v) {
		// If precision preservation is enabled and this is a plain string value,
		// try to convert it with precision
		e.mu.RLock()
		preservePrecision := e.preserveNumericPrecision
		e.mu.RUnlock()
		if preservePrecision {
			pc := NewPrecisionConverter()
			return pc.ConvertWithPrecision(v), nil
		}
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
	// Apply precision conversion if enabled
	e.mu.RLock()
	preservePrecision := e.preserveNumericPrecision
	e.mu.RUnlock()
	if preservePrecision {
		pc := NewPrecisionConverter()
		return pc.ConvertWithPrecision(parsed), nil
	}
	// Convert boolean results from template rendering to strings
	if parsed == trueString || parsed == falseString {
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
	e.mu.Lock()
	e.globalValues[name] = value
	e.mu.Unlock()
}

// preprocessContext adds default fields to the context and ensures proper boolean handling
func (e *TemplateEngine) preprocessContext(ctx map[string]any) map[string]any {
	if ctx == nil {
		ctx = make(map[string]any)
	}
	result := core.CloneMap(ctx)
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
	// Use pre-compiled regular expressions for better performance
	return templateExpressionRegex.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Handle whitespace trimming directives {{- and -}}
		hasLeftTrim := strings.HasPrefix(match, "{{-")
		hasRightTrim := strings.HasSuffix(match, "-}}")

		// Extract the content between delimiters, accounting for trim markers
		startIdx := 2
		endIdx := len(match) - 2
		if hasLeftTrim {
			startIdx = 3
		}
		if hasRightTrim {
			endIdx = len(match) - 3
		}
		content := strings.TrimSpace(match[startIdx:endIdx])

		// Find dot-path patterns that contain hyphens using pre-compiled regex
		processedContent := hyphenatedPathRegex.ReplaceAllStringFunc(content, func(pathMatch string) string {
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

		// Reconstruct the template with proper delimiters
		leftDelim := "{{"
		rightDelim := "}}"
		if hasLeftTrim {
			leftDelim = "{{-"
		}
		if hasRightTrim {
			rightDelim = "-}}"
		}
		return leftDelim + " " + processedContent + " " + rightDelim
	})
}
