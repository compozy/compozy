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

// authorConfig represents optional author metadata for the generated project YAML.
type authorConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url,omitempty"`
}

// workflowRef links the default workflow file in the scaffolded project.
type workflowRef struct {
	Source string `yaml:"source"`
}

// modelConfig configures the default model provider used by the template.
type modelConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key,omitempty"`
	APIURL   string `yaml:"api_url,omitempty"`
}

// runtimeConfig declares the runtime entrypoint and permissions for the agent.
type runtimeConfig struct {
	Type        string   `yaml:"type"`
	Entrypoint  string   `yaml:"entrypoint"`
	Permissions []string `yaml:"permissions,omitempty"`
}

// autoloadConfig defines how project resources are auto-discovered at runtime.
type autoloadConfig struct {
	Enabled bool     `yaml:"enabled"`
	Strict  bool     `yaml:"strict"`
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// projectConfig mirrors the structure expected by the compozy project template.
type projectConfig struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Mode        string            `yaml:"mode"`
	Author      *authorConfig     `yaml:"author,omitempty"`
	Workflows   []workflowRef     `yaml:"workflows,omitempty"`
	Models      []modelConfig     `yaml:"models,omitempty"`
	Runtime     *runtimeConfig    `yaml:"runtime,omitempty"`
	Autoload    *autoloadConfig   `yaml:"autoload,omitempty"`
	Templates   map[string]string `yaml:"templates,omitempty"`
}

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
	return []string{"workflows"}
}

// GetProjectConfig generates project configuration
func (t *Template) GetProjectConfig(opts *template.GenerateOptions) any {
	cfg := baseProjectConfig(opts)
	cfg.Author = authorFromOptions(opts)
	return cfg
}

// baseProjectConfig prepares the default scaffold configuration for a project.
func baseProjectConfig(opts *template.GenerateOptions) *projectConfig {
	return &projectConfig{
		Name:        opts.Name,
		Version:     opts.Version,
		Description: opts.Description,
		Mode:        opts.Mode,
		Workflows: []workflowRef{
			{Source: "./workflows/main.yaml"},
		},
		Models: []modelConfig{
			{
				Provider: "openai",
				Model:    "gpt-4.1-2025-04-14",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
			},
		},
		Runtime: &runtimeConfig{
			Type:       "bun",
			Entrypoint: "./entrypoint.ts",
			Permissions: []string{
				"--allow-read",
				"--allow-net",
				"--allow-write",
			},
		},
		Autoload: &autoloadConfig{
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
}

// authorFromOptions converts generator options into an author configuration.
func authorFromOptions(opts *template.GenerateOptions) *authorConfig {
	if opts.Author == "" {
		return nil
	}
	return &authorConfig{
		Name: opts.Author,
		URL:  opts.AuthorURL,
	}
}

// AddDockerFiles adds Docker-related files when DockerSetup is enabled
func (t *Template) AddDockerFiles(opts *template.GenerateOptions) []template.File {
	if !opts.DockerSetup || opts.Mode != "distributed" {
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
	if opts.DockerSetup && opts.Mode == "distributed" {
		files = append(files, template.File{
			Name:    "docker-compose.yaml",
			Content: dockerComposeTemplate,
		})
	}
	return files
}
