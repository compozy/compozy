package template

import (
	"fmt"
	"sort"
	"sync"
)

// registry is the global template registry
type registry struct {
	mu        sync.RWMutex
	templates map[string]Template
}

// globalRegistry is the singleton instance
var globalRegistry = &registry{
	templates: make(map[string]Template),
}

// Register adds a template to the global registry
func Register(name string, template Template) error {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	if _, exists := globalRegistry.templates[name]; exists {
		return fmt.Errorf("template %q already registered", name)
	}
	globalRegistry.templates[name] = template
	return nil
}

// get retrieves a template from the registry
func (r *registry) get(name string) (Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	template, exists := r.templates[name]
	if !exists {
		return nil, fmt.Errorf("template %q not found", name)
	}
	return template, nil
}

// list returns all registered templates
func (r *registry) list() []Metadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var metadataList []Metadata
	for _, template := range r.templates {
		metadataList = append(metadataList, template.GetMetadata())
	}
	sort.Slice(metadataList, func(i, j int) bool {
		return metadataList[i].Name < metadataList[j].Name
	})
	return metadataList
}
