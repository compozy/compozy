package template

import (
	"os"
)

// Template defines the interface for project templates
type Template interface {
	// GetMetadata returns template information
	GetMetadata() Metadata

	// GetFiles returns all template files
	GetFiles() []File

	// GetDirectories returns required directories
	GetDirectories() []string

	// GetProjectConfig generates project configuration data
	GetProjectConfig(opts *GenerateOptions) any
}

// DockerTemplate is an optional interface for templates that support Docker setup
type DockerTemplate interface {
	Template
	// GetFilesWithOptions returns template files with Docker configuration
	GetFilesWithOptions(opts *GenerateOptions) []File
}

// Metadata contains information about a template
type Metadata struct {
	Name        string
	Description string
	Author      string
	Version     string
}

// File represents a file to be generated from a template
type File struct {
	Name        string
	Content     string
	Permissions os.FileMode
}

// GenerateOptions contains options for generating a project from a template
type GenerateOptions struct {
	Path        string
	Name        string
	Description string
	Version     string
	Author      string
	AuthorURL   string
	DockerSetup bool
}

// Service defines the interface for the template service
type Service interface {
	// Register adds a new template
	Register(name string, template Template) error

	// Get retrieves a template by name
	Get(name string) (Template, error)

	// List returns all available templates
	List() []Metadata

	// Generate creates project from template
	Generate(templateName string, opts *GenerateOptions) error
}
