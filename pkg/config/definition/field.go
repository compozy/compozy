package definition

import "reflect"

// FieldDef defines a configuration field with its metadata
// Keep it simple - just what we need to solve the duplication problem
type FieldDef struct {
	Path      string       // Config path like "server.port"
	Default   any          // Default value
	CLIFlag   string       // CLI flag name like "port"
	Shorthand string       // Single character shorthand like "p"
	EnvVar    string       // Environment variable name like "SERVER_PORT"
	Type      reflect.Type // Field type for validation
	Help      string       // Help text for CLI
}

// Registry holds all configuration field definitions
type Registry struct {
	fields map[string]FieldDef
}

// NewRegistry creates a new field registry
func NewRegistry() *Registry {
	return &Registry{
		fields: make(map[string]FieldDef),
	}
}

// Register adds a field definition to the registry
func (r *Registry) Register(field *FieldDef) {
	r.fields[field.Path] = *field
}

// GetField returns a field definition by path
func (r *Registry) GetField(path string) (FieldDef, bool) {
	field, exists := r.fields[path]
	return field, exists
}

// GetDefault returns the default value for a field path
func (r *Registry) GetDefault(path string) any {
	if field, exists := r.fields[path]; exists {
		return field.Default
	}
	return nil
}

// GetAllFields returns all registered fields
func (r *Registry) GetAllFields() map[string]FieldDef {
	result := make(map[string]FieldDef)
	for k, v := range r.fields {
		result[k] = v
	}
	return result
}

// GetCLIFlagMapping returns a map of CLI flag names to config paths
func (r *Registry) GetCLIFlagMapping() map[string]string {
	mapping := make(map[string]string)
	for path, field := range r.fields {
		if field.CLIFlag != "" {
			mapping[field.CLIFlag] = path
		}
	}
	return mapping
}
