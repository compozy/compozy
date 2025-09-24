package basic

import (
	_ "embed"

	"github.com/compozy/compozy/pkg/template"
)

// Embedded template files
//
//go:embed compozy.yaml.tmpl
var compozyTemplate string

//go:embed entrypoint.ts.tmpl
var entrypointTemplate string

//go:embed workflow.yaml.tmpl
var workflowTemplate string

//go:embed greeting_tool.ts.tmpl
var greetingToolTemplate string

//go:embed docker-compose.yaml.tmpl
var dockerComposeTemplate string

//go:embed env.example.tmpl
var envExampleTemplate string

//go:embed api.http.tmpl
var compozyHTTPTemplate string

//go:embed gitignore.tmpl
var gitignoreTemplate string

//go:embed README.md.tmpl
var readmeTemplate string

// Template implements the Template interface for the basic project template
type Template struct{}

// Register registers the basic template with the global registry
func Register() error {
	return template.Register("basic", &Template{})
}

// GetMetadata returns template information
func (t *Template) GetMetadata() template.Metadata {
	return template.Metadata{
		Name:        "basic",
		Description: "Basic Compozy project template with example workflow",
		Author:      "Compozy Team",
		Version:     "1.0.0",
	}
}

// GetFiles returns all template files
func (t *Template) GetFiles() []template.File {
	files := []template.File{
		{
			Name:    "compozy.yaml",
			Content: compozyTemplate,
		},
		{
			Name:        "entrypoint.ts",
			Content:     entrypointTemplate,
			Permissions: 0755,
		},
		{
			Name:    "workflows/main.yaml",
			Content: workflowTemplate,
		},
		{
			Name:    "greeting_tool.ts",
			Content: greetingToolTemplate,
		},
		{
			Name:    "api.http",
			Content: compozyHTTPTemplate,
		},
		{
			Name:    "env.example",
			Content: envExampleTemplate,
		},
		{
			Name:    ".gitignore",
			Content: gitignoreTemplate,
		},
		{
			Name:    "README.md",
			Content: readmeTemplate,
		},
	}
	return files
}

// GetDirectories returns required directories
func (t *Template) GetDirectories() []string {
	// Basic template only needs workflows directory
	// The autoload configuration in compozy.yaml handles agents and tools
	// but we don't create empty directories
	return []string{"workflows"}
}

// GetProjectConfig generates project configuration
func (t *Template) GetProjectConfig(opts *template.GenerateOptions) any {
	// ProjectConfig structure matching what the templates expect
	type AuthorConfig struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url,omitempty"`
	}
	type WorkflowRef struct {
		Source string `yaml:"source"`
	}
	type ModelConfig struct {
		Provider string `yaml:"provider"`
		Model    string `yaml:"model"`
		APIKey   string `yaml:"api_key,omitempty"`
		APIURL   string `yaml:"api_url,omitempty"`
	}
	type RuntimeConfig struct {
		Type        string   `yaml:"type"`
		Entrypoint  string   `yaml:"entrypoint"`
		Permissions []string `yaml:"permissions,omitempty"`
	}
	type AutoloadConfig struct {
		Enabled bool     `yaml:"enabled"`
		Strict  bool     `yaml:"strict"`
		Include []string `yaml:"include,omitempty"`
		Exclude []string `yaml:"exclude,omitempty"`
	}
	type ProjectConfig struct {
		Name        string            `yaml:"name"`
		Version     string            `yaml:"version"`
		Description string            `yaml:"description"`
		Author      *AuthorConfig     `yaml:"author,omitempty"`
		Workflows   []WorkflowRef     `yaml:"workflows,omitempty"`
		Models      []ModelConfig     `yaml:"models,omitempty"`
		Runtime     *RuntimeConfig    `yaml:"runtime,omitempty"`
		Autoload    *AutoloadConfig   `yaml:"autoload,omitempty"`
		Templates   map[string]string `yaml:"templates,omitempty"`
	}
	config := &ProjectConfig{
		Name:        opts.Name,
		Version:     opts.Version,
		Description: opts.Description,
		Workflows: []WorkflowRef{
			{Source: "./workflows/main.yaml"},
		},
		Models: []ModelConfig{
			{
				Provider: "openai",
				Model:    "gpt-4.1-2025-04-14",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
			},
		},
		Runtime: &RuntimeConfig{
			Type:       "bun",
			Entrypoint: "./entrypoint.ts",
			Permissions: []string{
				"--allow-read",
				"--allow-net",
				"--allow-write",
			},
		},
		Autoload: &AutoloadConfig{
			Enabled: true,
			Strict:  true,
			Include: []string{
				"agents/*.yaml",
				"tools/*.yaml",
			},
			Exclude: []string{
				"**/*~",
				"**/*.bak",
				"**/*.tmp",
			},
		},
	}
	// Add author if provided
	if opts.Author != "" {
		config.Author = &AuthorConfig{
			Name: opts.Author,
			URL:  opts.AuthorURL,
		}
	}
	return config
}

// AddDockerFiles adds Docker-related files when DockerSetup is enabled
func (t *Template) AddDockerFiles(opts *template.GenerateOptions) []template.File {
	if !opts.DockerSetup {
		return nil
	}
	return []template.File{
		{
			Name:    "docker-compose.yaml",
			Content: dockerComposeTemplate,
		},
	}
}

// GetFilesWithOptions returns all template files including optional Docker files
func (t *Template) GetFilesWithOptions(opts *template.GenerateOptions) []template.File {
	files := t.GetFiles()
	if opts.DockerSetup {
		files = append(files, template.File{
			Name:    "docker-compose.yaml",
			Content: dockerComposeTemplate,
		})
	}
	return files
}
