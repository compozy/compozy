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
	tmpl, err := g.registry.get(templateName)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}
	compozyConfigPath := filepath.Join(opts.Path, "compozy.yaml")
	if _, err := os.Stat(compozyConfigPath); err == nil {
		return fmt.Errorf("project already exists at %s - aborting to prevent overwrite", opts.Path)
	}
	if err := os.MkdirAll(opts.Path, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	for _, dir := range tmpl.GetDirectories() {
		dirPath := filepath.Join(opts.Path, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	projectConfig := tmpl.GetProjectConfig(opts)
	files := tmpl.GetFiles()
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
	needsQuotes := strings.ContainsAny(s, "\"'\\:\n\r\t|>{}[]!#@*&") ||
		strings.HasPrefix(s, " ") ||
		strings.HasSuffix(s, " ") ||
		s == ""
	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "yes" || lower == "no" ||
		lower == "on" || lower == "off" || lower == "null" || lower == "~" {
		needsQuotes = true
	}
	if _, err := fmt.Sscanf(s, "%f", new(float64)); err == nil {
		needsQuotes = true
	}
	if needsQuotes {
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

// appendToGitignore appends content to an existing .gitignore file
func (g *generator) appendToGitignore(filePath string, tmpl *template.Template, data any) (writeErr error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .gitignore for appending: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && writeErr == nil {
			writeErr = fmt.Errorf("failed to close .gitignore file: %w", closeErr)
		}
	}()
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		writeErr = fmt.Errorf("failed to execute .gitignore template: %w", err)
		return
	}
	if _, err := f.WriteString("\n# Added by Compozy\n" + buf.String()); err != nil {
		writeErr = fmt.Errorf("failed to append to .gitignore: %w", err)
	}
	return
}

// createFile creates a single file from template
func (g *generator) createFile(basePath string, file File, data any) error {
	tmpl, err := template.New(file.Name).Funcs(createTemplateFuncMap()).Parse(file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	cleanFileName := filepath.Clean(file.Name)
	if strings.Contains(cleanFileName, "..") {
		return fmt.Errorf("invalid file name: %s contains path traversal", file.Name)
	}
	filePath := filepath.Join(basePath, cleanFileName)
	if file.Name == ".gitignore" {
		if _, err := os.Stat(filePath); err == nil {
			return g.appendToGitignore(filePath, tmpl, data)
		}
	}
	if file.Name == "env.example" {
		if _, err := os.Stat(filePath); err == nil {
			filePath = filepath.Join(basePath, "env-compozy.example")
		}
	}
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	executeErr := tmpl.Execute(f, data)
	closeErr := f.Close()
	if executeErr != nil {
		os.Remove(filePath)
		return fmt.Errorf("failed to execute template: %w", executeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close file: %w", closeErr)
	}
	if file.Permissions != 0 {
		if err := os.Chmod(filePath, file.Permissions); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}
	return nil
}
