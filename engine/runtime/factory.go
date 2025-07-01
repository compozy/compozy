package runtime

import (
	"context"
	"fmt"
)

// Factory creates Runtime instances based on configuration.
// This allows for flexible runtime selection and easier testing.
type Factory interface {
	// CreateRuntime creates a new Runtime instance for the given configuration.
	// The runtime type is determined by the config.RuntimeType field.
	CreateRuntime(ctx context.Context, config *Config) (Runtime, error)
}

// DefaultFactory is the default implementation of the Factory interface
type DefaultFactory struct {
	projectRoot string
}

// NewDefaultFactory creates a new DefaultFactory with the specified project root.
// The project root is used to resolve relative paths in runtime configurations.
func NewDefaultFactory(projectRoot string) Factory {
	return &DefaultFactory{
		projectRoot: projectRoot,
	}
}

// CreateRuntime creates the appropriate runtime based on the configuration.
// It supports "bun" and "node" runtime types, defaulting to "bun" if not specified.
func (f *DefaultFactory) CreateRuntime(ctx context.Context, config *Config) (Runtime, error) {
	if config == nil {
		return nil, fmt.Errorf("runtime config must not be nil")
	}

	// Default to Bun if runtime type is not specified
	runtimeType := config.RuntimeType
	if runtimeType == "" {
		runtimeType = RuntimeTypeBun
	}

	switch runtimeType {
	case RuntimeTypeBun:
		// TODO: Implement NewBunManager in task 3.0
		// For now, return the existing Manager which will be updated to support Bun
		return NewRuntimeManager(ctx, f.projectRoot, WithConfig(config))
	case RuntimeTypeNode:
		// TODO: Implement NewNodeManager in a future task
		return nil, fmt.Errorf("node.js runtime not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}
}
