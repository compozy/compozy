package template

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// generator implements the template generation logic
type generator struct {
	registry *registry
}

// newGenerator creates a new generator instance
func newGenerator() *generator {
	return &generator{
		registry: globalRegistry,
	}
}

// Generate creates a project from the specified template
func (g *generator) Generate(templateName string, opts *GenerateOptions) error {
	// Get the template
	tmpl, err := g.registry.get(templateName)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}
	// Check if project already exists
	compozyConfigPath := filepath.Join(opts.Path, "compozy.yaml")
	if _, err := os.Stat(compozyConfigPath); err == nil {
		return fmt.Errorf("project already exists at %s - aborting to prevent overwrite", opts.Path)
	}
	// Create project directory
	if err := os.MkdirAll(opts.Path, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	// Create subdirectories
	for _, dir := range tmpl.GetDirectories() {
		dirPath := filepath.Join(opts.Path, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	// Get project configuration
	projectConfig := tmpl.GetProjectConfig(opts)
	// Create all template files
	files := tmpl.GetFiles()
	// Check if template supports Docker setup
	if dockerTemplate, ok := tmpl.(DockerTemplate); ok && opts.DockerSetup {
		files = dockerTemplate.GetFilesWithOptions(opts)
	}
	for _, file := range files {
		if err := g.createFile(opts.Path, file, projectConfig); err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.Name, err)
		}
	}
	return nil
}

// yamlEscape escapes a string for safe YAML output
func yamlEscape(s string) string {
	// Check if string needs quoting for YAML
	needsQuotes := strings.ContainsAny(s, "\"'\\:\n\r\t|>{}[]!#@*&") ||
		strings.HasPrefix(s, " ") ||
		strings.HasSuffix(s, " ") ||
		s == ""
	// Check for YAML special values
	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "yes" || lower == "no" ||
		lower == "on" || lower == "off" || lower == "null" || lower == "~" {
		needsQuotes = true
	}
	// Check if it could be interpreted as a number
	if _, err := fmt.Sscanf(s, "%f", new(float64)); err == nil {
		needsQuotes = true
	}
	if needsQuotes {
		// Escape quotes within the string
		escaped := strings.ReplaceAll(s, "\"", "\\\"")
		return "\"" + escaped + "\""
	}
	return s
}

// createTemplateFuncMap creates the function map for templates
func createTemplateFuncMap() template.FuncMap {
	funcMap := sprig.TxtFuncMap()
	funcMap["jsEscape"] = template.JSEscapeString
	funcMap["yamlEscape"] = yamlEscape
	funcMap["htmlEscape"] = html.EscapeString
	return funcMap
}

// createFile creates a single file from template
func (g *generator) createFile(basePath string, file File, data any) error {
	// Parse template
	tmpl, err := template.New(file.Name).Funcs(createTemplateFuncMap()).Parse(file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	// Clean and validate file path
	cleanFileName := filepath.Clean(file.Name)
	if strings.Contains(cleanFileName, "..") {
		return fmt.Errorf("invalid file name: %s contains path traversal", file.Name)
	}
	filePath := filepath.Join(basePath, cleanFileName)
	// Handle special case for env.example
	if file.Name == "env.example" {
		if _, err := os.Stat(filePath); err == nil {
			// File exists, create env-compozy.example instead
			filePath = filepath.Join(basePath, "env-compozy.example")
		}
	}
	// Create the file
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	// Execute template and ensure file is closed properly
	executeErr := tmpl.Execute(f, data)
	closeErr := f.Close()
	// Handle errors in the correct order
	if executeErr != nil {
		// If execution failed, try to remove the partially written file
		os.Remove(filePath)
		return fmt.Errorf("failed to execute template: %w", executeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close file: %w", closeErr)
	}
	// Set permissions if specified
	if file.Permissions != 0 {
		if err := os.Chmod(filePath, file.Permissions); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}
	return nil
}
