package template

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultMode is the default deployment mode for generated projects.
	DefaultMode = "memory"
)

var validModes = []string{"memory", "persistent", "distributed"}

// Template represents a project template that can generate files.
// Implementations must respect the selected deployment mode by generating mode-specific configuration,
// include Docker resources only when required, and provide documentation tailored to the chosen mode.
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

// GenerateOptions contains configuration for template-based project generation.
type GenerateOptions struct {
	Context     context.Context // Execution context for logging and configuration lookup
	Path        string          // Target directory that receives the generated files
	Name        string          // Project name used across generated assets
	Description string          // Project description for documentation and metadata
	Version     string          // Initial project version (for example, "0.1.0")
	Author      string          // Author name for README and metadata files
	AuthorURL   string          // Author contact URL or email address
	DockerSetup bool            // Generate Docker scaffolding when true
	Mode        string          // Deployment mode: memory, persistent, or distributed
}

// ValidateMode ensures the provided deployment mode is supported.
func ValidateMode(mode string) error {
	for _, valid := range validModes {
		if mode == valid {
			return nil
		}
	}
	if mode == "standalone" {
		return fmt.Errorf(
			"mode 'standalone' has been replaced. Use 'memory' for no persistence or 'persistent' for disk-backed projects",
		)
	}
	return fmt.Errorf("invalid mode '%s'. Must be one of: %s", mode, strings.Join(validModes, ", "))
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
