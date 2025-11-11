package template

import (
	"fmt"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
)

// service implements the Service interface
type service struct {
	generator *generator
	registry  *registry
}

// singleton instance
var (
	instance *service
	once     sync.Once
)

// GetService returns the singleton template service instance
func GetService() Service {
	once.Do(func() {
		instance = &service{
			generator: newGenerator(),
			registry:  globalRegistry,
		}
	})
	return instance
}

// Register adds a new template
func (s *service) Register(name string, template Template) error {
	return Register(name, template)
}

// Get retrieves a template by name
func (s *service) Get(name string) (Template, error) {
	return s.registry.get(name)
}

// List returns all available templates
func (s *service) List() []Metadata {
	return s.registry.list()
}

// Generate creates project from template
func (s *service) Generate(templateName string, opts *GenerateOptions) error {
	if opts == nil {
		return fmt.Errorf("generate options cannot be nil")
	}
	if opts.Context == nil {
		return fmt.Errorf("generate options context cannot be nil")
	}
	if opts.Mode == "" {
		opts.Mode = DefaultMode
	}
	if err := ValidateMode(opts.Mode); err != nil {
		return fmt.Errorf("invalid mode: %w", err)
	}
	logger.FromContext(opts.Context).Info("generating template", "template", templateName, "mode", opts.Mode)
	return s.generator.Generate(templateName, opts)
}
