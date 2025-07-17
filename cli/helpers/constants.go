package helpers

// ContextKey is a custom type for context keys to avoid string collisions
type ContextKey string

const (
	// ConfigKey is the context key for storing configuration
	ConfigKey ContextKey = "config"
)

// OutputFormat represents different output formats
type OutputFormat string

const (
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatTable OutputFormat = "table"
	OutputFormatYAML  OutputFormat = "yaml"
	OutputFormatTUI   OutputFormat = "tui"
)
