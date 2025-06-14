package autoload

// Configurable is an interface for configurations that can be registered
type Configurable interface {
	// GetResource returns the resource type for this configuration
	GetResource() string
	// GetID returns the unique identifier for this configuration
	GetID() string
}
