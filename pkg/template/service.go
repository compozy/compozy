package template

import (
	"sync"
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
	return s.generator.Generate(templateName, opts)
}
