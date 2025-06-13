package autoload

// Configurable is an interface for configurations that can be registered
type Configurable interface {
	GetResource() string
	GetID() string
}
